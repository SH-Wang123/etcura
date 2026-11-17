package etcd

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/model"
)

// Manager owns the active client and invalidates its session on every switch.
type Manager struct {
	mu           sync.Mutex
	client       *clientv3.Client
	cfg          *model.ConnectionConfig
	probeCancel  context.CancelFunc
	clientCancel context.CancelFunc
	generation   uint64
	state        string
	message      string
	rev          int64
	version      string
}

func NewManager() *Manager {
	return &Manager{state: model.StateDisconnected}
}

func (m *Manager) Connect(ctx context.Context, cfg *model.ConnectionConfig, password string) (model.StatusInfo, error) {
	profile := *cfg
	if err := profile.Normalize(); err != nil {
		return model.StatusInfo{}, err
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.mu.Lock()
	m.resetLocked(model.StateConnecting)
	generation := m.generation
	m.probeCancel = cancel
	m.cfg = &profile
	m.mu.Unlock()

	// The dial and status probe are bounded separately from the client's
	// lifetime. Close releases the client on switch, disconnect or shutdown.
	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))
	stopAbort := context.AfterFunc(probeCtx, clientCancel)
	client, err := NewClient(clientCtx, &profile, password)
	var probe *model.StatusResponse
	if err == nil {
		probe, err = ProbeStatus(probeCtx, client, time.Duration(profile.RequestTimeoutMs)*time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	stopped := stopAbort()
	if generation != m.generation {
		clientCancel()
		if client != nil {
			_ = client.Close()
		}
		return model.StatusInfo{}, model.NewError(model.CodeConnectionChanged, "The connection changed")
	}
	m.probeCancel = nil
	if err == nil && (!stopped || probeCtx.Err() != nil) {
		err = MapError(probeCtx.Err())
	}
	if err != nil {
		clientCancel()
		if client != nil {
			_ = client.Close()
		}
		m.cfg = nil
		m.state = model.StateError
		m.message = err.Error()
		return m.statusLocked(), err
	}

	m.client = client
	m.clientCancel = clientCancel
	m.state = model.StateConnected
	m.rev = probe.Header
	m.version = probe.EtcdVersion
	return m.statusLocked(), nil
}

func (m *Manager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetLocked(model.StateDisconnected)
}

func (m *Manager) DisconnectProfile(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg != nil && m.cfg.ID == id {
		m.resetLocked(model.StateDisconnected)
	}
}

// Client validates the session and returns a client and timeout from the same
// profile. An in-flight caller can only use the client it originally acquired.
func (m *Manager) Client(session string, write bool) (*clientv3.Client, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client == nil || session == "" || session != m.sessionLocked() {
		return nil, 0, model.NewError(model.CodeConnectionChanged, "The connection changed")
	}
	if write && m.cfg.ReadOnly {
		return nil, 0, model.NewError(model.CodeReadOnly, "This connection is read-only")
	}
	return m.client, time.Duration(m.cfg.RequestTimeoutMs) * time.Millisecond, nil
}

func (m *Manager) Status() model.StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) statusLocked() model.StatusInfo {
	info := model.StatusInfo{State: m.state, Message: m.message}
	if m.client != nil {
		info.ProfileID = m.cfg.ID
		info.Session = m.sessionLocked()
		info.ReadOnly = m.cfg.ReadOnly
		info.Name = m.cfg.Name
		info.Endpoints = strings.Join(m.cfg.Endpoints, ", ")
		info.Header = m.rev
		info.EtcdVersion = m.version
	}
	return info
}

func (m *Manager) SetRevision(session string, rev int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil && session == m.sessionLocked() && rev > m.rev {
		m.rev = rev
	}
}

func (m *Manager) sessionLocked() string {
	return strconv.FormatUint(m.generation, 10)
}

func (m *Manager) resetLocked(state string) {
	m.generation++
	if m.probeCancel != nil {
		m.probeCancel()
		m.probeCancel = nil
	}
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	if m.clientCancel != nil {
		m.clientCancel()
		m.clientCancel = nil
	}
	m.cfg = nil
	m.state = state
	m.message = ""
	m.rev = 0
	m.version = ""
}
