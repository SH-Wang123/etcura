package workbench

import (
	"bytes"
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/etcd"
	"etcura/internal/model"
)

const (
	pageRows     = 80
	pageScan     = 600
	scanBatch    = 8
	maxSyncItems = 100
	maxSyncBytes = 16 << 20
)

type Engine struct {
	mu       sync.Mutex
	next     uint64
	manager  *etcd.Manager
	starting string
	current  *comparison
	plan     *syncPlan
	watches  map[string]*watch
}

func New(manager *etcd.Manager) *Engine {
	return &Engine{manager: manager, watches: map[string]*watch{}}
}

func (e *Engine) nextID() string {
	e.next++
	return strconv.FormatUint(e.next, 10)
}

type snapshot struct {
	session, prefix string
	client          *clientv3.Client
	timeout         time.Duration
	revision        int64
}

type comparison struct {
	id             string
	ctx            context.Context
	cancel         context.CancelFunc
	source, target snapshot
}

func (e *Engine) StartCompare(ctx context.Context, req model.CompareRequest) (model.CompareInfo, error) {
	if req.SourcePrefix == "" || req.TargetPrefix == "" || req.SourceSession == req.TargetSession {
		return model.CompareInfo{}, model.NewInvalidArgument("Select two connections and enter both prefixes")
	}
	e.mu.Lock()
	id := e.nextID()
	e.starting = id
	if e.current != nil {
		e.current.cancel()
	}
	e.current, e.plan = nil, nil
	e.mu.Unlock()

	source, err := e.snapshot(ctx, req.SourceSession, req.SourcePrefix)
	if err != nil {
		return model.CompareInfo{}, err
	}
	target, err := e.snapshot(ctx, req.TargetSession, req.TargetPrefix)
	if err != nil {
		return model.CompareInfo{}, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.starting != id {
		return model.CompareInfo{}, model.NewError(model.CodeCancelled, "Comparison superseded")
	}
	ctx, cancel := context.WithCancel(ctx)
	cmp := &comparison{id: id, ctx: ctx, cancel: cancel, source: source, target: target}
	e.current, e.plan = cmp, nil
	_, _, readOnly := e.manager.Client(target.session, true)
	return model.CompareInfo{
		ID: cmp.id, SourceRevision: rev(source.revision), TargetRevision: rev(target.revision),
		TargetReadOnly: readOnly != nil,
	}, nil
}

func (e *Engine) snapshot(ctx context.Context, session, prefix string) (snapshot, error) {
	client, timeout, err := e.manager.Client(session, false)
	if err != nil {
		return snapshot{}, err
	}
	request, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.Get(request, prefix, clientv3.WithPrefix(), clientv3.WithLimit(1), clientv3.WithKeysOnly())
	if err != nil {
		return snapshot{}, etcd.MapError(err)
	}
	return snapshot{session, prefix, client, timeout, resp.Header.Revision}, nil
}

func (e *Engine) CancelCompare(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil && e.current.id == id {
		e.current.cancel()
		e.current, e.plan = nil, nil
	}
}

func (e *Engine) comparison(id string) (*comparison, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil || e.current.id != id {
		return nil, model.NewInvalidArgument("Start a new comparison")
	}
	return e.current, nil
}

type iterator struct {
	snapshot
	pos       string
	buffer    []*mvccpb.KeyValue
	exhausted bool
}

func (it *iterator) peek(ctx context.Context) (*mvccpb.KeyValue, error) {
	if len(it.buffer) == 0 && !it.exhausted {
		request, cancel := context.WithTimeout(ctx, it.timeout)
		defer cancel()
		resp, err := it.client.Get(request, it.pos, clientv3.WithRange(etcd.PrefixEnd(it.prefix)),
			clientv3.WithRev(it.revision), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
			clientv3.WithLimit(scanBatch))
		if err != nil {
			return nil, etcd.MapError(err)
		}
		it.buffer = resp.Kvs
		it.exhausted = !resp.More
		if resp.More {
			it.pos = etcd.NextCursor(string(resp.Kvs[len(resp.Kvs)-1].Key))
		}
	}
	if len(it.buffer) == 0 {
		return nil, nil
	}
	return it.buffer[0], nil
}

func (e *Engine) ComparePage(id, cursor, filter string) (model.ComparePage, error) {
	cmp, err := e.comparison(id)
	if err != nil {
		return model.ComparePage{}, err
	}
	suffix, err := decodeKey(cursor)
	if err != nil {
		return model.ComparePage{}, err
	}
	filterValid := filter == "different" || filter == "all" || filter == "source" || filter == "target" || filter == "changed" || filter == "same"
	if !filterValid {
		return model.ComparePage{}, model.NewInvalidArgument("Unknown comparison filter")
	}
	left := iterator{snapshot: cmp.source, pos: cmp.source.prefix + suffix}
	right := iterator{snapshot: cmp.target, pos: cmp.target.prefix + suffix}
	page := model.ComparePage{Rows: []model.CompareRow{}}
	for page.Scanned < pageScan && len(page.Rows) < pageRows {
		s, err := left.peek(cmp.ctx)
		if err != nil {
			return model.ComparePage{}, err
		}
		t, err := right.peek(cmp.ctx)
		if err != nil {
			return model.ComparePage{}, err
		}
		if s == nil && t == nil {
			return page, nil
		}

		switch {
		case t == nil || s != nil && bytes.Compare(s.Key[len(left.prefix):], t.Key[len(right.prefix):]) < 0:
			suffix, t = string(s.Key[len(left.prefix):]), nil
		case s == nil || bytes.Compare(s.Key[len(left.prefix):], t.Key[len(right.prefix):]) > 0:
			suffix, s = string(t.Key[len(right.prefix):]), nil
		default:
			suffix = string(s.Key[len(left.prefix):])
		}
		if s != nil {
			left.buffer = left.buffer[1:]
		}
		if t != nil {
			right.buffer = right.buffer[1:]
		}
		row := compareRow(suffix, s, t)
		if filter == "all" || filter == row.State || filter == "different" && row.State != "same" {
			page.Rows = append(page.Rows, row)
		}
		page.Scanned++
	}
	if len(left.buffer) > 0 || len(right.buffer) > 0 || !left.exhausted || !right.exhausted {
		page.NextCursor = encodeKey(etcd.NextCursor(suffix))
	}
	return page, nil
}

func compareRow(suffix string, source, target *mvccpb.KeyValue) model.CompareRow {
	row := model.CompareRow{Key: encodeKey(suffix), Label: displayKey(suffix)}
	switch {
	case source == nil && target == nil:
		row.State = "missing"
	case source == nil:
		row.State = "target"
	case target == nil:
		row.State = "source"
		row.SourceLease = source.Lease != 0
	case bytes.Equal(source.Value, target.Value):
		row.State = "same"
	default:
		row.State = "changed"
	}
	return row
}

func displayKey(key string) string {
	if utf8.ValidString(key) && strings.IndexFunc(key, unicode.IsControl) < 0 {
		return key
	}
	return "base64:" + encodeKey(key)
}

func encodeKey(key string) string { return base64.StdEncoding.EncodeToString([]byte(key)) }
func decodeKey(key string) (string, error) {
	raw, err := base64.StdEncoding.Strict().DecodeString(key)
	if err != nil {
		return "", model.NewInvalidArgument("Invalid key encoding")
	}
	return string(raw), nil
}
func rev(value int64) string { return strconv.FormatInt(value, 10) }

func readVersion(ctx context.Context, side snapshot, suffix string) (*mvccpb.KeyValue, error) {
	ctx, cancel := context.WithTimeout(ctx, side.timeout)
	defer cancel()
	resp, err := side.client.Get(ctx, side.prefix+suffix, clientv3.WithRev(side.revision))
	if err != nil {
		return nil, etcd.MapError(err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return resp.Kvs[0], nil
}

func (cmp *comparison) values(suffix string) (*mvccpb.KeyValue, *mvccpb.KeyValue, error) {
	source, err := readVersion(cmp.ctx, cmp.source, suffix)
	if err != nil {
		return nil, nil, err
	}
	target, err := readVersion(cmp.ctx, cmp.target, suffix)
	return source, target, err
}

func record(kv *mvccpb.KeyValue, limit int) *model.RecordValue {
	if kv == nil {
		return nil
	}
	value := kv.Value
	display := value[:min(len(value), limit)]
	return &model.RecordValue{
		Base64: base64.StdEncoding.EncodeToString(display), UTF8: utf8.Valid(display),
		Bytes: len(value), Truncated: len(value) > limit,
	}
}

func (e *Engine) CompareDetail(id, key string) (model.CompareDetail, error) {
	cmp, err := e.comparison(id)
	if err != nil {
		return model.CompareDetail{}, err
	}
	suffix, err := decodeKey(key)
	if err != nil {
		return model.CompareDetail{}, err
	}
	s, t, err := cmp.values(suffix)
	if err != nil {
		return model.CompareDetail{}, err
	}
	return model.CompareDetail{Row: compareRow(suffix, s, t), Source: record(s, 4<<20), Target: record(t, 4<<20)}, nil
}

type syncItem struct {
	row      model.CompareRow
	key      string
	value    []byte
	revision int64
}

type syncPlan struct {
	id    string
	cmp   *comparison
	items []syncItem
}

func (e *Engine) PreviewSync(id string, keys []string) (model.SyncPlan, error) {
	cmp, err := e.comparison(id)
	if err != nil {
		return model.SyncPlan{}, err
	}
	if len(keys) == 0 || len(keys) > maxSyncItems {
		return model.SyncPlan{}, model.NewInvalidArgument("Select between 1 and %d keys", maxSyncItems)
	}
	if _, _, err := e.manager.Client(cmp.target.session, true); err != nil {
		return model.SyncPlan{}, err
	}

	plan := &syncPlan{cmp: cmp}
	rows := make([]model.CompareRow, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	total := 0
	for _, key := range keys {
		suffix, err := decodeKey(key)
		if err != nil {
			return model.SyncPlan{}, err
		}
		if seen[suffix] {
			continue
		}
		seen[suffix] = true
		source, target, err := cmp.values(suffix)
		if err != nil {
			return model.SyncPlan{}, err
		}
		row := compareRow(suffix, source, target)
		if row.State != "source" && row.State != "changed" {
			return model.SyncPlan{}, model.NewInvalidArgument("Selected key has no value to synchronize")
		}
		item := syncItem{row: row, key: cmp.target.prefix + suffix, value: source.Value}
		total += len(item.key) + len(item.value)
		if total > maxSyncBytes {
			return model.SyncPlan{}, model.NewInvalidArgument("Select a smaller batch (maximum 16 MiB)")
		}
		if target != nil {
			item.revision = target.ModRevision
		}
		plan.items = append(plan.items, item)
		rows = append(rows, row)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != cmp {
		return model.SyncPlan{}, model.NewInvalidArgument("The comparison changed")
	}
	plan.id = e.nextID()
	e.plan = plan
	return model.SyncPlan{ID: plan.id, Rows: rows}, nil
}

func (e *Engine) ApplySync(id string, permanent []string) ([]model.SyncResult, error) {
	e.mu.Lock()
	plan := e.plan
	if plan == nil || plan.id != id {
		e.mu.Unlock()
		return nil, model.NewInvalidArgument("Preview the changes again")
	}
	e.plan = nil
	e.mu.Unlock()

	allowPermanent := make(map[string]bool, len(permanent))
	for _, key := range permanent {
		allowPermanent[key] = true
	}
	results := make([]model.SyncResult, 0, len(plan.items))
	interrupted := false
	for _, item := range plan.items {
		result := model.SyncResult{Key: item.row.Key, Label: item.row.Label, State: "success"}
		if interrupted || item.row.SourceLease && !allowPermanent[item.row.Key] {
			result.State = "skipped"
			results = append(results, result)
			continue
		}
		client, timeout, err := e.manager.Client(plan.cmp.target.session, true)
		switch {
		case err != nil:
			result.State, result.Message = "failed", err.Error()
		case plan.cmp.ctx.Err() != nil:
			result.State = "skipped"
		default:
			_, err = etcd.PutWithCAS(etcd.WithRequestTimeout(plan.cmp.ctx, timeout), client, item.key, item.value, item.revision, 0)
			if err != nil {
				mapped := etcd.MapError(err)
				result.Message = mapped.Message
				switch mapped.Code {
				case model.CodeRevisionConflict, model.CodeKeyAlreadyExists, model.CodeKeyNotFound:
					result.State = "conflict"
				case model.CodeTimeout, model.CodeCancelled, model.CodeConnectionFailed, model.CodeUnknown:
					result.State = "unknown"
					interrupted = true
				default:
					result.State = "failed"
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}
