package etcd

import (
	"context"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/model"
)

// Manager owns the single long-lived clientv3.Client for the app. Exactly one
// connection is active at a time; switching profiles closes the old client
// before dialing the new one, so clients never leak.
type Manager struct {
	mu     sync.Mutex
	client *clientv3.Client
	cfg    *model.ConnectionConfig

	state   string
	message string
	rev     int64
	version string
}

func NewManager() *Manager {
	return &Manager{state: model.StateDisconnected}
}

// Connect closes any previous client, dials the new profile and verifies it
// with a real Status RPC before flipping the state to Connected.
func (m *Manager) Connect(ctx context.Context, cfg *model.ConnectionConfig, password string) (*model.StatusResponse, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.state = model.StateConnecting
	m.message = ""
	m.mu.Unlock()

	// Old client first: no half-open duplicate connections.
	m.closeLocked()

	client, err := NewClient(ctx, cfg, password)
	if err != nil {
		m.setError(err)
		return nil, err
	}

	probe, err := ProbeStatus(ctx, client, time.Duration(cfg.RequestTimeoutMs)*time.Millisecond)
	if err != nil {
		_ = client.Close()
		m.setError(err)
		return nil, err
	}

	m.mu.Lock()
	m.client = client
	m.cfg = cfg
	m.state = model.StateConnected
	m.rev = probe.Header
	m.version = probe.EtcdVersion
	m.message = ""
	m.mu.Unlock()

	return probe, nil
}

// Disconnect closes the active client, if any.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeLocked()
	m.state = model.StateDisconnected
	m.message = ""
	m.rev = 0
	m.version = ""
	return nil
}

// Client returns the active client or NOT_CONNECTED. Callers use it for RPCs
// with their own timeout context and never Close it themselves.
func (m *Manager) Client() (*clientv3.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client == nil {
		return nil, model.NewError(model.CodeNotConnected, "Not connected to an etcd server")
	}
	return m.client, nil
}

// RequestTimeout is the request timeout of the active profile.
func (m *Manager) RequestTimeout() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return model.DefaultRequestTimeout
	}
	return time.Duration(m.cfg.RequestTimeoutMs) * time.Millisecond
}

// Profile returns a copy of the active connection config.
func (m *Manager) Profile() (model.ConnectionConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return model.ConnectionConfig{}, false
	}
	return *m.cfg, true
}

// Status returns a snapshot for the status bar.
func (m *Manager) Status() model.StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	info := model.StatusInfo{State: m.state, Message: m.message}
	if m.cfg != nil {
		info.Name = m.cfg.Name
		info.Endpoints = strings.Join(m.cfg.Endpoints, ", ")
	}
	if m.state == model.StateConnected {
		info.Header = m.rev
		info.EtcdVersion = m.version
	}
	return info
}

// SetRevision updates the cached cluster revision after a successful read or
// write so the status bar stays truthful.
func (m *Manager) SetRevision(rev int64) {
	m.mu.Lock()
	m.rev = rev
	m.mu.Unlock()
}

// Close shuts the manager down (application exit path).
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeLocked()
	m.state = model.StateDisconnected
}

func (m *Manager) setError(err error) {
	m.mu.Lock()
	m.state = model.StateError
	m.message = err.Error()
	m.mu.Unlock()
}

func (m *Manager) closeLocked() {
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	m.cfg = nil
}
