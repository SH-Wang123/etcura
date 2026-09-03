package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"etcd-eye/internal/config"
	"etcd-eye/internal/etcd"
	"etcd-eye/internal/model"
	"etcd-eye/internal/protodec"
)

// App is the Wails binding surface. It contains no etcd logic of its own: it
// wires the UI to the connection manager (internal/etcd), the kv operations
// and the config store, and enforces the cancellation policy for UI-driven
// requests that can be superseded by newer ones.
type App struct {
	ctx context.Context
	mgr *etcd.Manager

	// listMu guards the single cancellation slot for key-list requests: a new
	// ListKeys cancels the previous one so a slow stale query can never win.
	listMu     sync.Mutex
	listCancel context.CancelFunc

	// detailMu guards the same pattern for GetKey (rapid selection changes).
	detailMu     sync.Mutex
	detailCancel context.CancelFunc

	// protos holds the compiled descriptor set of imported .proto files for
	// heuristic protobuf value matching.
	protos *protodec.Importer
}

func NewApp() *App {
	return &App{mgr: etcd.NewManager(), protos: protodec.NewImporter()}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// shutdown runs on app exit: cancel outstanding requests, close the etcd
// client, then let Wails tear down.
func (a *App) shutdown(ctx context.Context) {
	a.listMu.Lock()
	if a.listCancel != nil {
		a.listCancel()
		a.listCancel = nil
	}
	a.listMu.Unlock()
	a.detailMu.Lock()
	if a.detailCancel != nil {
		a.detailCancel()
		a.detailCancel = nil
	}
	a.detailMu.Unlock()
	a.mgr.Close()
}

// ---------------------------------------------------------------- connections

// ListConnections returns all saved profiles without secrets.
func (a *App) ListConnections() []model.ConnectionSummary {
	conns, err := config.Load()
	if err != nil {
		return nil
	}
	out := make([]model.ConnectionSummary, 0, len(conns))
	for _, c := range conns {
		out = append(out, model.ConnectionSummary{
			ID:          c.ID,
			Name:        c.Name,
			Endpoints:   c.Endpoints,
			HasPassword: config.HasPassword(c.ID),
		})
	}
	return out
}

// SaveConnection creates or updates a profile. Password handling:
//   - remember && password != ""  -> store new password in Keychain
//   - remember && password == ""  -> keep whatever is stored (editing case)
//   - !remember                   -> remove stored password
func (a *App) SaveConnection(cfg model.ConnectionConfig, password string, rememberPassword bool) (model.ConnectionConfig, error) {
	if err := cfg.Normalize(); err != nil {
		return model.ConnectionConfig{}, err
	}

	conns, err := config.Load()
	if err != nil {
		return model.ConnectionConfig{}, err
	}

	isNew := cfg.ID == ""
	if isNew {
		id, err := newID()
		if err != nil {
			return model.ConnectionConfig{}, err
		}
		cfg.ID = id
	} else {
		found := false
		for _, c := range conns {
			if c.ID == cfg.ID {
				found = true
				break
			}
		}
		if !found {
			return model.ConnectionConfig{}, model.NewInvalidArgument("unknown connection id %q", cfg.ID)
		}
	}

	switch {
	case rememberPassword && password != "":
		if err := config.SetPassword(cfg.ID, password); err != nil {
			return model.ConnectionConfig{}, err
		}
	case !rememberPassword:
		if err := config.DeletePassword(cfg.ID); err != nil {
			return model.ConnectionConfig{}, model.NewError(model.CodeConfigError, "could not remove stored password").
				WithDetail(err.Error())
		}
	}

	if isNew {
		conns = append(conns, cfg)
	} else {
		for i := range conns {
			if conns[i].ID == cfg.ID {
				conns[i] = cfg
			}
		}
	}
	if err := config.Save(conns); err != nil {
		return model.ConnectionConfig{}, err
	}
	return cfg, nil
}

// DeleteConnection removes a profile and its Keychain entry. If it is the
// active connection the client is closed.
func (a *App) DeleteConnection(id string) error {
	if id == "" {
		return model.NewInvalidArgument("connection id must not be empty")
	}
	conns, err := config.Load()
	if err != nil {
		return err
	}
	kept := make([]model.ConnectionConfig, 0, len(conns))
	found := false
	for _, c := range conns {
		if c.ID == id {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	if !found {
		return model.NewInvalidArgument("unknown connection id %q", id)
	}
	if err := config.Save(kept); err != nil {
		return err
	}
	if err := config.DeletePassword(id); err != nil {
		return model.NewError(model.CodeConfigError, "could not remove stored password").WithDetail(err.Error())
	}
	if profile, ok := a.mgr.Profile(); ok && profile.ID == id {
		_ = a.mgr.Disconnect()
	}
	return nil
}

// GetConnection returns the full profile of one connection (no password).
func (a *App) GetConnection(id string) (model.ConnectionConfig, error) {
	conns, err := config.Load()
	if err != nil {
		return model.ConnectionConfig{}, err
	}
	for _, c := range conns {
		if c.ID == id {
			return c, nil
		}
	}
	return model.ConnectionConfig{}, model.NewInvalidArgument("unknown connection id %q", id)
}

// TestConnection dials the given profile (or the stored password when the
// field is blank) and runs a real Status RPC against it. It never touches the
// active connection.
func (a *App) TestConnection(cfg model.ConnectionConfig, password string) (model.StatusResponse, error) {
	if err := cfg.Normalize(); err != nil {
		return model.StatusResponse{}, err
	}
	if password == "" && cfg.ID != "" {
		password = config.GetPassword(cfg.ID)
	}

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	client, err := etcd.NewClient(ctx, &cfg, password)
	if err != nil {
		return model.StatusResponse{}, err
	}
	defer client.Close()

	resp, err := etcd.ProbeStatus(ctx, client, time.Duration(cfg.RequestTimeoutMs)*time.Millisecond)
	if err != nil {
		return model.StatusResponse{}, err
	}
	return *resp, nil
}

// Connect switches to the profile with the given id: close old client, dial,
// verify with Status, update the connection state.
func (a *App) Connect(id string) (model.StatusInfo, error) {
	if id == "" {
		return model.StatusInfo{}, model.NewInvalidArgument("connection id must not be empty")
	}
	conns, err := config.Load()
	if err != nil {
		return model.StatusInfo{}, err
	}
	var cfg *model.ConnectionConfig
	for i := range conns {
		if conns[i].ID == id {
			cfg = &conns[i]
			break
		}
	}
	if cfg == nil {
		return model.StatusInfo{}, model.NewInvalidArgument("unknown connection id %q", id)
	}
	password := config.GetPassword(id)

	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()

	if _, err := a.mgr.Connect(ctx, cfg, password); err != nil {
		return a.mgr.Status(), err
	}
	return a.mgr.Status(), nil
}

// Disconnect closes the active connection.
func (a *App) Disconnect() (model.StatusInfo, error) {
	if err := a.mgr.Disconnect(); err != nil {
		return a.mgr.Status(), err
	}
	return a.mgr.Status(), nil
}

// GetStatus reports the current connection state for the status bar.
func (a *App) GetStatus() model.StatusInfo { return a.mgr.Status() }

// ---------------------------------------------------------------------- keys

// ListKeys fetches one page. Starting a new list request cancels the previous
// in-flight one (prefix typed faster than the server answered), so stale
// responses can neither overwrite fresh state nor leak goroutines.
func (a *App) ListKeys(prefix, cursor string, pageSize int) (model.KeyPage, error) {
	client, err := a.mgr.Client()
	if err != nil {
		return model.KeyPage{}, err
	}

	a.listMu.Lock()
	if a.listCancel != nil {
		a.listCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.listCancel = cancel
	a.listMu.Unlock()

	ctx = etcd.WithRequestTimeout(ctx, a.mgr.RequestTimeout())
	page, err := etcd.ListPage(ctx, client, prefix, cursor, pageSize)
	cancel() // release this request's resources; the next call re-cancels anyway
	if err != nil {
		return model.KeyPage{}, err
	}
	a.mgr.SetRevision(page.Revision)
	return page, nil
}

// ListLevel returns one page of a dynamic browser column under prefix:
// distinct child segments interleaved with direct leaf keys, cursor-paginated.
// Tree refreshes issue several of these in parallel, so each request gets its
// own timeout context instead of a shared cancellation slot (parallel tree
// requests must not cancel each other).
func (a *App) ListLevel(prefix, cursor string, pageSize int) (model.LevelPage, error) {
	client, err := a.mgr.Client()
	if err != nil {
		return model.LevelPage{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.mgr.RequestTimeout())
	defer cancel()
	return etcd.ListLevel(ctx, client, prefix, cursor, pageSize)
}

// GetKey loads the full record (value included) for the selected key. Rapid
// selection changes cancel the previous load.
func (a *App) GetKey(key string) (model.KvDetail, error) {	client, err := a.mgr.Client()
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx, cancel := a.detailContext()
	defer cancel()
	detail, err := etcd.GetKey(ctx, client, key)
	if err != nil {
		return model.KvDetail{}, err
	}
	a.mgr.SetRevision(detail.ModRevision)
	return detail, nil
}

// CreateKey writes a brand-new key. expectedModRevision=0 in the CAS txn
// makes it fail with KEY_ALREADY_EXISTS instead of silently overwriting.
func (a *App) CreateKey(key, valueBase64, leaseID string) (model.KvDetail, error) {
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.KvDetail{}, err
	}
	lease, err := etcd.ParseLeaseID(leaseID)
	if err != nil {
		return model.KvDetail{}, err
	}
	client, err := a.mgr.Client()
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.mgr.RequestTimeout())
	defer cancel()
	return etcd.PutWithCAS(ctx, client, key, value, 0, lease)
}

// SaveKey updates an existing key under a ModRevision CAS check.
func (a *App) SaveKey(key, valueBase64 string, expectedModRevision int64) (model.KvDetail, error) {
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.KvDetail{}, err
	}
	client, err := a.mgr.Client()
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.mgr.RequestTimeout())
	defer cancel()
	return etcd.PutWithCAS(ctx, client, key, value, expectedModRevision, 0)
}

// DeleteKey deletes exactly one key after the UI has confirmed it.
func (a *App) DeleteKey(key string) error {
	client, err := a.mgr.Client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.mgr.RequestTimeout())
	defer cancel()
	return etcd.DeleteKey(ctx, client, key)
}

// ---------------------------------------------------------------- protobuf

// ImportProtos parses the given .proto files (cross-file imports resolved
// from their directories) and replaces the imported descriptor set.
func (a *App) ImportProtos(paths []string) ([]model.ProtoFileInfo, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return a.protos.Import(ctx, paths)
}

// ListProtos reports the currently imported proto files.
func (a *App) ListProtos() []model.ProtoFileInfo { return a.protos.Files() }

// ClearProtos forgets all imported protos.
func (a *App) ClearProtos() { a.protos.Clear() }

// MatchProto heuristically matches a value (base64) against every imported
// message type and returns ranked decoded candidates.
func (a *App) MatchProto(valueBase64 string) (model.ProtoMatchResult, error) {
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.ProtoMatchResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	_ = ctx // matching is pure CPU over in-memory descriptors
	return a.protos.Match(value), nil
}

// ------------------------------------------------------------------ internals

func (a *App) detailContext() (context.Context, context.CancelFunc) {
	a.detailMu.Lock()
	defer a.detailMu.Unlock()
	if a.detailCancel != nil {
		a.detailCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.detailCancel = cancel
	return etcd.WithRequestTimeout(ctx, a.mgr.RequestTimeout()), cancel
}

// decodeValue decodes the frontend-provided base64 payload strictly, so a
// broken encoding surfaces as INVALID_ARGUMENT instead of corrupted bytes.
func decodeValue(valueBase64 string) ([]byte, error) {
	if valueBase64 == "" {
		return []byte{}, nil
	}
	value, err := base64.StdEncoding.DecodeString(valueBase64)
	if err != nil {
		return nil, model.NewInvalidArgument("value is not valid base64").WithDetail(err.Error())
	}
	return value, nil
}

func newID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", model.NewError(model.CodeUnknown, "cannot generate connection id").WithDetail(err.Error())
	}
	return hex.EncodeToString(b[:]), nil
}
