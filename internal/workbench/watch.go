package workbench

import (
	"context"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/etcd"
	"etcura/internal/model"
)

const maxWatchEvents = 128

type watch struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	events   []model.WatchEvent
	state    string
	revision int64
	missed   int
	message  string
}

func (e *Engine) StartWatch(ctx context.Context, req model.WatchStart) (string, error) {
	if req.Key == "" {
		return "", model.NewInvalidArgument("Enter a key or prefix to watch")
	}
	client, timeout, err := e.manager.Client(req.Session, false)
	if err != nil {
		return "", err
	}
	first, err := watchRevision(ctx, client, timeout, req)
	if err != nil {
		return "", err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.watches) >= 8 {
		return "", model.NewInvalidArgument("Close a watch before opening another")
	}
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(client.Ctx(), cancel)
	w := &watch{cancel: cancel, state: "live", revision: first}
	id := e.nextID()
	e.watches[id] = w
	go func() {
		defer stop()
		defer cancel()
		w.run(ctx, client, timeout, req, first)
	}()
	return id, nil
}

func (e *Engine) PollWatch(id string) (model.WatchBatch, error) {
	e.mu.Lock()
	w := e.watches[id]
	e.mu.Unlock()
	if w == nil {
		return model.WatchBatch{}, model.NewInvalidArgument("Watch is closed")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	events := w.events
	if events == nil {
		events = []model.WatchEvent{}
	}
	batch := model.WatchBatch{Events: events, State: w.state, Revision: rev(w.revision), Missed: w.missed, Error: w.message}
	w.events, w.missed = nil, 0
	return batch, nil
}

func (e *Engine) StopWatch(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if w := e.watches[id]; w != nil {
		delete(e.watches, id)
		w.cancel()
	}
}

func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil {
		e.current.cancel()
	}
	e.current, e.plan, e.starting = nil, nil, ""
	for id, w := range e.watches {
		w.cancel()
		delete(e.watches, id)
	}
}

func watchOptions(req model.WatchStart) []clientv3.OpOption {
	if req.Prefix {
		return []clientv3.OpOption{clientv3.WithPrefix()}
	}
	return nil
}

func watchRevision(ctx context.Context, client *clientv3.Client, timeout time.Duration, req model.WatchStart) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.Get(ctx, req.Key, append(watchOptions(req), clientv3.WithLimit(1), clientv3.WithKeysOnly())...)
	if err != nil {
		return 0, etcd.MapError(err)
	}
	return resp.Header.Revision, nil
}

func (w *watch) status(state, message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state, w.message = state, message
}

func (w *watch) append(event model.WatchEvent) {
	if len(w.events) == maxWatchEvents {
		copy(w.events, w.events[1:])
		w.events = w.events[:len(w.events)-1]
		w.missed++
	}
	w.events = append(w.events, event)
}

func (w *watch) run(ctx context.Context, client *clientv3.Client, timeout time.Duration, req model.WatchStart, revision int64) {
	watcher := clientv3.NewWatcher(client)
	defer watcher.Close()
	defer func() {
		if ctx.Err() != nil {
			w.status("stopped", "")
		}
	}()
	var compacted int64
	for ctx.Err() == nil {
		if compacted > 0 {
			current, err := watchRevision(ctx, client, min(timeout, 3*time.Second), req)
			if err != nil {
				w.status("reconnecting", err.Error())
				if !retryWatch(ctx) {
					return
				}
				continue
			}
			w.mu.Lock()
			w.append(model.WatchEvent{Type: "gap", Revision: rev(compacted), SeenAt: time.Now().UnixMilli()})
			w.revision = current
			w.mu.Unlock()
			revision, compacted = current, 0
		}
		watchCtx, cancel := context.WithCancel(clientv3.WithRequireLeader(ctx))
		opts := append(watchOptions(req), clientv3.WithRev(revision+1), clientv3.WithPrevKV(), clientv3.WithCreatedNotify(), clientv3.WithProgressNotify())
		stream := watcher.Watch(watchCtx, req.Key, opts...)
		ticker := time.NewTicker(3 * time.Second)
		err := func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					if _, err := watchRevision(ctx, client, min(timeout, 1500*time.Millisecond), req); err != nil {
						return err
					}
				case response, ok := <-stream:
					if !ok {
						return model.NewError(model.CodeConnectionFailed, "Watch stream disconnected")
					}
					if response.CompactRevision > 0 {
						compacted = response.CompactRevision
					}
					if err := response.Err(); err != nil {
						return err
					}
					w.mu.Lock()
					if response.IsProgressNotify() {
						revision = max(revision, response.Header.Revision)
					}
					for _, event := range response.Events {
						kind := "update"
						after := record(event.Kv, 32<<10)
						if event.Type == mvccpb.DELETE {
							kind, after = "delete", nil
						} else if event.IsCreate() {
							kind = "create"
						}
						w.append(model.WatchEvent{
							Type: kind, Key: displayKey(string(event.Kv.Key)), Revision: rev(event.Kv.ModRevision),
							SeenAt: time.Now().UnixMilli(), Before: record(event.PrevKv, 32<<10), After: after,
						})
						revision = max(revision, event.Kv.ModRevision)
					}
					w.revision, w.state, w.message = revision, "live", ""
					w.mu.Unlock()
				}
			}
		}()
		ticker.Stop()
		cancel()
		if ctx.Err() != nil {
			return
		}
		if mapped := etcd.MapError(err); mapped.Code == model.CodePermissionDenied {
			w.status("error", mapped.Error())
			return
		}
		w.status("reconnecting", "")
		if !retryWatch(ctx) {
			return
		}
	}
}

func retryWatch(ctx context.Context) bool {
	timer := time.NewTimer(750 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
