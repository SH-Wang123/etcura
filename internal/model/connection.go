// Package model defines the data structures exchanged between the Go backend
// and the Svelte frontend through Wails bindings.
package model

import "time"

const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"
)

const (
	DefaultDialTimeout    = 5 * time.Second
	DefaultRequestTimeout = 10 * time.Second

	DefaultDialTimeoutMs    = int(DefaultDialTimeout / time.Millisecond)
	DefaultRequestTimeoutMs = int(DefaultRequestTimeout / time.Millisecond)
)

// TLSConfig carries file paths only; certificate contents are loaded from disk
// at connect time and never persisted.
type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	CAFile     string `json:"caFile,omitempty"`
	CertFile   string `json:"certFile,omitempty"`
	KeyFile    string `json:"keyFile,omitempty"`
	SkipVerify bool   `json:"skipVerify"`
}

// ConnectionConfig is one saved etcd server profile. The password is never
// part of this struct: it lives in the macOS Keychain (see internal/config).
type ConnectionConfig struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Endpoints        []string  `json:"endpoints"`
	Username         string    `json:"username,omitempty"`
	DialTimeoutMs    int       `json:"dialTimeoutMs"`
	RequestTimeoutMs int       `json:"requestTimeoutMs"`
	ReadOnly         bool      `json:"readOnly"`
	TLS              TLSConfig `json:"tls"`
}

// ConnectionSummary is what the connection list shows. It never contains
// secrets, only whether a password is stored in the Keychain.
type ConnectionSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Endpoints   []string `json:"endpoints"`
	HasPassword bool     `json:"hasPassword"`
}

// StatusInfo describes the current connection state machine value plus the
// last known cluster facts.
type StatusInfo struct {
	State       string `json:"state"`
	ProfileID   string `json:"profileId"`
	Session     string `json:"session"`
	ReadOnly    bool   `json:"readOnly"`
	Name        string `json:"name"`
	Endpoints   string `json:"endpoints"`
	Header      int64  `json:"header"`
	EtcdVersion string `json:"etcdVersion"`
	Message     string `json:"message"`
}

// Normalize fills defaults and cleans user input. It is called before every
// save/test/connect so the rest of the code can rely on valid values.
func (c *ConnectionConfig) Normalize() error {
	c.Name = trimSpace(c.Name)
	if c.Name == "" {
		return NewInvalidArgument("connection name must not be empty")
	}
	eps := make([]string, 0, len(c.Endpoints))
	for _, e := range c.Endpoints {
		e = trimSpace(e)
		if e == "" {
			continue
		}
		if !EndpointValid(e) {
			return NewInvalidArgument("invalid endpoint: %q (expected host:port or scheme://host:port)", e)
		}
		eps = append(eps, e)
	}
	if len(eps) == 0 {
		return NewInvalidArgument("at least one endpoint is required, e.g. 127.0.0.1:2379")
	}
	c.Endpoints = eps
	if c.DialTimeoutMs <= 0 {
		c.DialTimeoutMs = DefaultDialTimeoutMs
	}
	if c.RequestTimeoutMs <= 0 {
		c.RequestTimeoutMs = DefaultRequestTimeoutMs
	}
	return nil
}
