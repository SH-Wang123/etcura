package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"etcura/internal/model"
)

// NewClient dials a real clientv3 client from a normalized connection config.
// The dial itself is lazy in gRPC; callers must follow up with a Status RPC
// (see ProbeStatus) to prove the server is actually reachable.
func NewClient(ctx context.Context, cfg *model.ConnectionConfig, password string) (*clientv3.Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, model.NewInvalidArgument("at least one endpoint is required")
	}

	tlsCfg, err := LoadTLSConfig(&cfg.TLS)
	if err != nil {
		return nil, err
	}

	dialTimeout := time.Duration(cfg.DialTimeoutMs) * time.Millisecond
	if dialTimeout <= 0 {
		dialTimeout = model.DefaultDialTimeout
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   NormalizeEndpoints(cfg),
		DialTimeout: dialTimeout,
		Username:    cfg.Username,
		Password:    password,
		TLS:         tlsCfg,
		Logger:      zap.NewNop(),
		Context:     ctx,
	})
	if err != nil {
		return nil, MapError(err)
	}
	return client, nil
}

// ProbeStatus returns the first successful Status response. Concurrent probes
// let a healthy configured endpoint answer even if an earlier one is down.
func ProbeStatus(ctx context.Context, client *clientv3.Client, timeout time.Duration) (*model.StatusResponse, error) {
	if timeout <= 0 {
		timeout = model.DefaultRequestTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		status *model.StatusResponse
		err    error
	}
	endpoints := client.Endpoints()
	if len(endpoints) == 0 {
		return nil, model.NewInvalidArgument("at least one endpoint is required")
	}
	results := make(chan result, len(endpoints))
	for _, endpoint := range endpoints {
		go func() {
			resp, err := client.Status(ctx, endpoint)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{status: &model.StatusResponse{
				Ok: true, Header: resp.Header.Revision, EtcdVersion: resp.Version, Endpoint: endpoint,
			}}
		}()
	}
	var lastErr error
	for range endpoints {
		probe := <-results
		if probe.err == nil {
			return probe.status, nil
		}
		lastErr = probe.err
	}
	return nil, MapError(lastErr)
}

// LoadTLSConfig materialises the configured certificate files into a
// *tls.Config. Paths were chosen by the user in the connection dialog, so
// unreadable files become a TLS_ERROR with a precise message.
func LoadTLSConfig(tc *model.TLSConfig) (*tls.Config, error) {
	if tc == nil || !tc.Enabled {
		return nil, nil
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if tc.SkipVerify {
		tlsCfg.InsecureSkipVerify = true
	}
	if tc.CAFile != "" {
		pem, err := os.ReadFile(tc.CAFile)
		if err != nil {
			return nil, model.NewError(model.CodeTLSError, "cannot read CA certificate file: %s", tc.CAFile).WithDetail(err.Error())
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, model.NewError(model.CodeTLSError, "CA certificate file contains no valid PEM certificates: %s", tc.CAFile)
		}
		tlsCfg.RootCAs = pool
	}
	if tc.CertFile != "" || tc.KeyFile != "" {
		if tc.CertFile == "" || tc.KeyFile == "" {
			return nil, model.NewError(model.CodeTLSError, "client certificate and client private key must be configured together")
		}
		cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
		if err != nil {
			return nil, model.NewError(model.CodeTLSError, "cannot load client certificate/key pair").
				WithDetail(err.Error())
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}
