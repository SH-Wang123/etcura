package etcd

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"etcd-eye/internal/model"
)

func TestMapErrorContext(t *testing.T) {
	err := MapError(context.DeadlineExceeded)
	if err.Code != model.CodeTimeout {
		t.Fatalf("DeadlineExceeded -> %s, want TIMEOUT", err.Code)
	}
	err = MapError(context.Canceled)
	if err.Code != model.CodeCancelled {
		t.Fatalf("Canceled -> %s, want CANCELLED", err.Code)
	}
	if err.Detail == "" {
		t.Fatal("detail must carry the original error text")
	}
}

func TestMapErrorGRPCStatus(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{status.Error(codes.Unavailable, "connection error"), model.CodeConnectionFailed},
		{status.Error(codes.DeadlineExceeded, "timeout"), model.CodeTimeout},
		{status.Error(codes.Canceled, "ctx canceled"), model.CodeCancelled},
		{status.Error(codes.PermissionDenied, "etcdserver: permission denied"), model.CodePermissionDenied},
		{status.Error(codes.Unauthenticated, "etcdserver: authentication failed"), model.CodePermissionDenied},
		{status.Error(codes.InvalidArgument, "etcdserver: key not found"), model.CodeKeyNotFound},
		{status.Error(codes.FailedPrecondition, "etcdserver: requested lease not found"), model.CodeLeaseNotFound},
		{status.Error(codes.ResourceExhausted, "etcdserver: quota exceeded"), model.CodeQuotaExceeded},
		{status.Error(codes.InvalidArgument, "etcdserver: something invalid"), model.CodeInvalidArgument},
	}
	for i, c := range cases {
		got := MapError(c.err)
		if got.Code != c.want {
			t.Errorf("case %d: %v -> %s, want %s", i, c.err, got.Code, c.want)
		}
		if got.Detail != c.err.Error() {
			t.Errorf("case %d: detail must preserve original error", i)
		}
	}
}

func TestMapErrorNetwork(t *testing.T) {
	opErr := &net.OpError{Op: "dial", Err: fmt.Errorf("connect: connection refused")}
	if got := MapError(opErr); got.Code != model.CodeConnectionFailed {
		t.Fatalf("connection refused -> %s, want CONNECTION_FAILED", got.Code)
	}

	// A (fake) timeout net error must map to TIMEOUT.
	timeoutErr := &fakeTimeoutNetError{}
	if got := MapError(timeoutErr); got.Code != model.CodeTimeout {
		t.Fatalf("net timeout -> %s, want TIMEOUT", got.Code)
	}
}

type fakeTimeoutNetError struct{}

func (fakeTimeoutNetError) Error() string { return "i/o timeout" }
func (fakeTimeoutNetError) Timeout() bool { return true }
func (fakeTimeoutNetError) Temporary() bool { return true }

func TestMapErrorTLS(t *testing.T) {
	err := MapError(x509.UnknownAuthorityError{})
	if err.Code != model.CodeTLSError {
		t.Fatalf("x509 -> %s, want TLS_ERROR", err.Code)
	}
}

func TestMapErrorPassthroughAppError(t *testing.T) {
	orig := model.NewInvalidArgument("bad input %d", 7)
	if got := MapError(orig); got != orig {
		t.Fatal("AppError must pass through unchanged")
	}
	if got := MapError(fmt.Errorf("wrapped: %w", orig)); got != orig {
		t.Fatal("wrapped AppError must pass through via errors.As")
	}
}

func TestMapErrorUnknown(t *testing.T) {
	if got := MapError(errors.New("something weird")); got.Code != model.CodeUnknown {
		t.Fatalf("got %s", got.Code)
	}
}

func TestAppErrorJSONRoundTrip(t *testing.T) {
	err := model.NewError(model.CodeRevisionConflict, "conflict on %s", "/k").WithDetail("rev=5")
	// Error() must be parseable JSON so the frontend can decode it.
	var parsed map[string]string
	if e := json.Unmarshal([]byte(err.Error()), &parsed); e != nil {
		t.Fatalf("Error() is not valid JSON: %v", e)
	}
	if parsed["code"] != string(model.CodeRevisionConflict) {
		t.Fatalf("code = %q", parsed["code"])
	}
	if parsed["message"] != "conflict on /k" {
		t.Fatalf("message = %q", parsed["message"])
	}
	if parsed["detail"] != "rev=5" {
		t.Fatalf("detail = %q", parsed["detail"])
	}
}

func TestParseLeaseID(t *testing.T) {
	if id, err := ParseLeaseID(""); err != nil || id != 0 {
		t.Fatalf("empty -> %d, %v", id, err)
	}
	if id, err := ParseLeaseID(" 123 "); err != nil || id != 123 {
		t.Fatalf("got %d, %v", id, err)
	}
	if _, err := ParseLeaseID("abc"); err == nil {
		t.Fatal("non-numeric lease id must fail")
	}
}

func TestNormalizeEndpoints(t *testing.T) {
	cfg := &model.ConnectionConfig{Endpoints: []string{"127.0.0.1:2379", "https://10.0.0.1:2379"}}
	cfg.TLS.Enabled = true
	got := NormalizeEndpoints(cfg)
	want := []string{"https://127.0.0.1:2379", "https://10.0.0.1:2379"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	cfg.TLS.Enabled = false
	got = NormalizeEndpoints(cfg)
	if got[0] != "http://127.0.0.1:2379" {
		t.Fatalf("plaintext scheme not applied: %v", got)
	}
}

func TestModelNormalize(t *testing.T) {
	cfg := model.ConnectionConfig{Name: " x ", Endpoints: []string{" 127.0.0.1:2379 ", ""}, DialTimeoutMs: 0, RequestTimeoutMs: 0}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if cfg.Name != "x" || cfg.Endpoints[0] != "127.0.0.1:2379" {
		t.Fatalf("not normalized: %+v", cfg)
	}
	if cfg.DialTimeoutMs != int(model.DefaultDialTimeout/time.Millisecond) {
		t.Fatalf("dial timeout default not applied: %d", cfg.DialTimeoutMs)
	}

	cfg = model.ConnectionConfig{Name: "", Endpoints: []string{"127.0.0.1:2379"}}
	if err := cfg.Normalize(); err == nil {
		t.Fatal("empty name must be rejected")
	}

	cfg = model.ConnectionConfig{Name: "x", Endpoints: []string{"not a host"}}
	if err := cfg.Normalize(); err == nil {
		t.Fatal("invalid endpoint must be rejected")
	}

	cfg = model.ConnectionConfig{Name: "x", Endpoints: []string{"http://127.0.0.1:2379"}}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("scheme endpoint must be accepted: %v", err)
	}
}

func TestEndpointValidation(t *testing.T) {
	valid := []string{"127.0.0.1:2379", "localhost:22379", "https://etcd.example.com:2379", "http://[::1]:2379"}
	for _, ep := range valid {
		if !model.EndpointValid(ep) {
			t.Errorf("%q should be valid", ep)
		}
	}
	invalid := []string{"", "127.0.0.1", "://", "ftp://x:2379", "127.0.0.1:"}
	for _, ep := range invalid {
		if model.EndpointValid(ep) {
			t.Errorf("%q should be invalid", ep)
		}
	}
}
