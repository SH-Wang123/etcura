package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"etcura/internal/config"
	"etcura/internal/etcd"
	"etcura/internal/model"
	"etcura/internal/protodec"
	"etcura/internal/script"
	"etcura/internal/workbench"
)

// App is the Wails binding surface. It contains no etcd logic of its own: it
// wires the UI to the connection manager (internal/etcd), the kv operations
// and the config store, and enforces the cancellation policy for UI-driven
// requests that can be superseded by newer ones.
type App struct {
	workbench  *workbench.Engine
	ctx        context.Context
	mgr        *etcd.Manager
	profilesMu sync.RWMutex

	// protos holds the compiled descriptor set of imported .proto files for
	// heuristic protobuf value matching.
	protos *protodec.Importer

	// script runs the user-chosen post-processing script over values.
	script *script.Runner
}

func NewApp() *App {
	app := &App{
		mgr:    etcd.NewManager(),
		protos: protodec.NewImporter(),
		script: script.NewRunner(),
	}
	// Re-apply the persisted script. A configuration that no longer validates
	// is ignored rather than preventing startup.
	app.workbench = workbench.New(app.mgr)
	if st, err := config.LoadSettings(); err == nil {
		_ = app.script.Restore(st.Script)
	}
	return app
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(_ context.Context) {
	if a.workbench != nil {
		a.workbench.Close()
	}
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
	a.profilesMu.Lock()
	defer a.profilesMu.Unlock()
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
	if !isNew {
		a.mgr.DisconnectProfile(cfg.ID)
	}
	return cfg, nil
}

// DeleteConnection removes a profile and its Keychain entry. If it is the
// active connection the client is closed.
func (a *App) DeleteConnection(id string) error {
	a.profilesMu.Lock()
	defer a.profilesMu.Unlock()
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
	a.mgr.DisconnectProfile(id)
	if err := config.DeletePassword(id); err != nil {
		return model.NewError(model.CodeConfigError, "could not remove stored password").WithDetail(err.Error())
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
	a.profilesMu.RLock()
	defer a.profilesMu.RUnlock()
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

	return a.mgr.Connect(ctx, cfg, password)
}

// Disconnect closes the client for a single workspace tab.
func (a *App) Disconnect(session string) { a.mgr.Disconnect(session) }

// ListSessions reports all open connections.
func (a *App) ListSessions() []model.StatusInfo { return a.mgr.Status() }

// ---------------------------------------------------------------------- keys

// ListKeys fetches one page of key metadata for the supplied session.
func (a *App) ListKeys(session, prefix, cursor string, pageSize int) (model.KeyPage, error) {
	client, timeout, err := a.mgr.Client(session, false)
	if err != nil {
		return model.KeyPage{}, err
	}
	page, err := etcd.ListPage(etcd.WithRequestTimeout(a.ctx, timeout), client, prefix, cursor, pageSize)
	if err == nil {
		a.mgr.SetRevision(session, page.Revision)
	}
	return page, err
}

// ListLevel returns one page of a dynamic browser column under prefix:
// distinct child segments interleaved with direct leaf keys, cursor-paginated.
// Tree refreshes issue several of these in parallel, so each request gets its
// own timeout context instead of a shared cancellation slot (parallel tree
// requests must not cancel each other).
func (a *App) ListLevel(session, prefix, cursor string, pageSize int) (model.LevelPage, error) {
	client, timeout, err := a.mgr.Client(session, false)
	if err != nil {
		return model.LevelPage{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()
	return etcd.ListLevel(etcd.WithRequestTimeout(ctx, timeout), client, prefix, cursor, pageSize)
}

// GetKey loads the full record for the selected key in the supplied session.
func (a *App) GetKey(session, key string) (model.KvDetail, error) {
	client, timeout, err := a.mgr.Client(session, false)
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx := etcd.WithRequestTimeout(a.ctx, timeout)
	detail, err := etcd.GetKey(ctx, client, key)
	if err != nil {
		return model.KvDetail{}, err
	}
	a.mgr.SetRevision(session, detail.ModRevision)
	return detail, nil
}

// CreateKey writes a brand-new key. expectedModRevision=0 in the CAS txn
// makes it fail with KEY_ALREADY_EXISTS instead of silently overwriting.
func (a *App) CreateKey(session, key, valueBase64, leaseID string) (model.KvDetail, error) {
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.KvDetail{}, err
	}
	lease, err := etcd.ParseLeaseID(leaseID)
	if err != nil {
		return model.KvDetail{}, err
	}
	client, timeout, err := a.mgr.Client(session, true)
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()
	return etcd.PutWithCAS(etcd.WithRequestTimeout(ctx, timeout), client, key, value, 0, lease)
}

// SaveKey updates an existing key under a ModRevision CAS check.
func (a *App) SaveKey(session, key, valueBase64 string, expectedModRevision int64) (model.KvDetail, error) {
	if expectedModRevision <= 0 {
		return model.KvDetail{}, model.NewInvalidArgument("a positive revision is required")
	}
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.KvDetail{}, err
	}
	client, timeout, err := a.mgr.Client(session, true)
	if err != nil {
		return model.KvDetail{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()
	return etcd.PutWithCAS(etcd.WithRequestTimeout(ctx, timeout), client, key, value, expectedModRevision, 0)
}

// DeleteKey deletes exactly one key after the UI has confirmed it.
func (a *App) DeleteKey(session, key string, expectedModRevision int64) error {
	client, timeout, err := a.mgr.Client(session, true)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()
	return etcd.DeleteKey(etcd.WithRequestTimeout(ctx, timeout), client, key, expectedModRevision)
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

// ------------------------------------------------------------------ script

// SetScript stores the post-processing snippet. The app wraps the body with
// the configured input/output variable bindings: the raw value bytes are bound
// to InputVar, and OutputVar is read back as the displayed result. Either name
// may be empty (no input binding / display stdout respectively). The raw bytes
// always reach the script's stdin as well.
func (a *App) SetScript(cfg model.ScriptConfig) (model.ScriptInfo, error) {
	if err := a.script.Set(cfg); err != nil {
		return model.ScriptInfo{}, err
	}
	stored := a.script.Config()
	info := a.script.Info()
	if stored == nil || info == nil {
		return model.ScriptInfo{}, model.NewError(model.CodeScriptError, "Script was not configured")
	}
	if err := config.SaveSettings(config.Settings{Script: stored}); err != nil {
		return *info, err
	}
	return *info, nil
}

// ClearScript removes the configured script.
func (a *App) ClearScript() error {
	a.script.Clear()
	return config.SaveSettings(config.Settings{})
}

// GetScript reports the configured script, or a zero value when none is set.
func (a *App) GetScript() model.ScriptInfo {
	if info := a.script.Info(); info != nil {
		return *info
	}
	return model.ScriptInfo{}
}

// RunScript runs the configured snippet over the value (base64) and returns
// its output for display.
func (a *App) RunScript(valueBase64 string) (model.ScriptOutput, error) {
	value, err := decodeValue(valueBase64)
	if err != nil {
		return model.ScriptOutput{}, err
	}
	return a.script.Run(a.ctx, value)
}

// ------------------------------------------------------------------ internals

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
