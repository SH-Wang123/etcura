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

const maxSessions = 16

type connection struct {
	client *clientv3.Client
	stop   context.CancelFunc
	probe  context.CancelFunc
	status model.StatusInfo
	config model.ConnectionConfig
}

// Manager owns independent etcd clients and issues process-unique session IDs.
type Manager struct {
	mu       sync.Mutex
	next     uint64
	profiles map[string]*connection
	sessions map[string]*connection
	closed   bool
}

func NewManager() *Manager {
	return &Manager{profiles: map[string]*connection{}, sessions: map[string]*connection{}}
}

func (m *Manager) Connect(ctx context.Context, profile *model.ConnectionConfig, password string) (model.StatusInfo, error) {
	cfg := *profile
	if err := cfg.Normalize(); err != nil {
		return model.StatusInfo{}, err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return model.StatusInfo{}, model.NewError(model.CodeNotConnected, "Connection manager is closed")
	}
	if existing := m.profiles[cfg.ID]; existing != nil {
		status := existing.status
		m.mu.Unlock()
		if status.State == model.StateConnected {
			return status, nil
		}
		return model.StatusInfo{}, model.NewError(model.CodeConnectionFailed, "This profile is already connecting")
	}
	if len(m.profiles) >= maxSessions {
		m.mu.Unlock()
		return model.StatusInfo{}, model.NewInvalidArgument("At most %d connections may be open", maxSessions)
	}

	probeCtx, cancelProbe := context.WithCancel(ctx)
	defer cancelProbe()
	clientCtx, cancelClient := context.WithCancel(context.WithoutCancel(ctx))
	stopAbort := context.AfterFunc(probeCtx, cancelClient)
	m.next++
	id := strconv.FormatUint(m.next, 10)
	conn := &connection{stop: cancelClient, probe: cancelProbe, config: cfg}
	m.profiles[cfg.ID] = conn
	m.mu.Unlock()

	client, err := NewClient(clientCtx, &cfg, password)
	var probe *model.StatusResponse
	if err == nil {
		probe, err = ProbeStatus(probeCtx, client, time.Duration(cfg.RequestTimeoutMs)*time.Millisecond)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	stopped := stopAbort()
	if m.profiles[cfg.ID] != conn {
		err = model.NewError(model.CodeConnectionChanged, "The connection was closed")
	} else if err == nil && (!stopped || probeCtx.Err() != nil) {
		err = MapError(probeCtx.Err())
	}
	if err != nil {
		cancelClient()
		if client != nil {
			_ = client.Close()
		}
		if m.profiles[cfg.ID] == conn {
			delete(m.profiles, cfg.ID)
		}
		return model.StatusInfo{}, err
	}

	conn.client = client
	conn.probe = nil
	conn.status = model.StatusInfo{
		State: model.StateConnected, ProfileID: cfg.ID, Session: id,
		ReadOnly: cfg.ReadOnly, Name: cfg.Name, Environment: cfg.Environment,
		Endpoints: strings.Join(cfg.Endpoints, ", "), Header: probe.Header, EtcdVersion: probe.EtcdVersion,
	}
	m.sessions[id] = conn
	return conn.status, nil
}

func (m *Manager) Disconnect(session string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn := m.sessions[session]; conn != nil {
		m.closeLocked(conn)
	}
}

func (m *Manager) DisconnectProfile(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn := m.profiles[id]; conn != nil {
		m.closeLocked(conn)
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, conn := range m.profiles {
		m.closeLocked(conn)
	}
}

func (m *Manager) closeLocked(conn *connection) {
	delete(m.profiles, conn.config.ID)
	delete(m.sessions, conn.status.Session)
	if conn.probe != nil {
		conn.probe()
	}
	conn.stop()
	if conn.client != nil {
		_ = conn.client.Close()
	}
}

func (m *Manager) Client(session string, write bool) (*clientv3.Client, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn := m.sessions[session]
	if conn == nil {
		return nil, 0, model.NewError(model.CodeConnectionChanged, "The connection was closed")
	}
	if write && conn.status.ReadOnly {
		return nil, 0, model.NewError(model.CodeReadOnly, "This connection is read-only")
	}
	return conn.client, time.Duration(conn.config.RequestTimeoutMs) * time.Millisecond, nil
}

func (m *Manager) Status() []model.StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]model.StatusInfo, 0, len(m.sessions))
	for _, conn := range m.sessions {
		result = append(result, conn.status)
	}
	return result
}

func (m *Manager) SetRevision(session string, rev int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn := m.sessions[session]; conn != nil && rev > conn.status.Header {
		conn.status.Header = rev
	}
}
