package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"etcd-eye/internal/model"
)

// MapError converts any error surfaced by the etcd client, gRPC, TLS or this
// package into the unified *model.AppError so the UI never shows raw
// "rpc error: code = ..." strings. The original text is preserved in Detail.
func MapError(err error) *model.AppError {
	if err == nil {
		return nil
	}
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	detail := err.Error()

	switch {
	case errors.Is(err, context.Canceled):
		return model.NewError(model.CodeCancelled, "Request cancelled").
			WithDetail(detail)
	case errors.Is(err, context.DeadlineExceeded):
		return model.NewError(model.CodeTimeout, "Request timed out").
			WithDetail(detail)
	}

	// etcd typed errors (auth, lease, key not found, ...).
	switch {
	case errors.Is(err, clientv3.ErrNoAvailableEndpoints):
		return model.NewError(model.CodeConnectionFailed, "No available etcd endpoints").
			WithDetail(detail)
	}

	if st, ok := status.FromError(err); ok {
		return mapGRPCStatus(st, detail)
	}

	var x509Err x509.UnknownAuthorityError
	if errors.As(err, &x509Err) {
		return model.NewError(model.CodeTLSError, "TLS: certificate signed by unknown authority").
			WithDetail(detail)
	}
	var x509HostErr x509.HostnameError
	if errors.As(err, &x509HostErr) {
		return model.NewError(model.CodeTLSError, "TLS: server certificate does not match the endpoint hostname").
			WithDetail(detail)
	}
	var tlsRecordErr tls.RecordHeaderError
	if errors.As(err, &tlsRecordErr) {
		return model.NewError(model.CodeTLSError, "TLS handshake failed (is the endpoint speaking TLS?)").
			WithDetail(detail)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return model.NewError(model.CodeTimeout, "Network timeout while talking to etcd").
				WithDetail(detail)
		}
		return model.NewError(model.CodeConnectionFailed, "Could not reach etcd server").
			WithDetail(detail)
	}

	if isConnRefused(err) {
		return model.NewError(model.CodeConnectionFailed, "Connection refused by etcd endpoint").
			WithDetail(detail)
	}

	return model.NewError(model.CodeUnknown, "Unexpected error").
		WithDetail(detail)
}

func mapGRPCStatus(st *status.Status, detail string) *model.AppError {
	code := st.Code()
	msg := strings.ToLower(st.Message())

	switch {
	case code == codes.Unavailable,
		code == codes.Unimplemented && strings.Contains(msg, "unknown service"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "no leader"),
		strings.Contains(msg, "unhealthy"):
		return model.NewError(model.CodeConnectionFailed, "Could not reach etcd server").
			WithDetail(detail)
	case code == codes.DeadlineExceeded:
		return model.NewError(model.CodeTimeout, "Request timed out").
			WithDetail(detail)
	case code == codes.Canceled:
		return model.NewError(model.CodeCancelled, "Request cancelled").
			WithDetail(detail)
	case code == codes.PermissionDenied, code == codes.Unauthenticated,
		strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "auth failed"),
		strings.Contains(msg, "invalid auth token"),
		strings.Contains(msg, "user name is empty"):
		return model.NewError(model.CodePermissionDenied, "Authentication or permission check failed").
			WithDetail(detail)
	case strings.Contains(msg, "key not found"):
		return model.NewError(model.CodeKeyNotFound, "Key not found").
			WithDetail(detail)
	case strings.Contains(msg, "lease not found"):
		return model.NewError(model.CodeLeaseNotFound, "Requested lease not found").
			WithDetail(detail)
	case strings.Contains(msg, "mvcc: database space exceeded"):
		return model.NewError(model.CodeQuotaExceeded, "etcd backend quota exceeded (compaction/defrag needed)").
			WithDetail(detail)
	case code == codes.InvalidArgument, code == codes.OutOfRange, code == codes.FailedPrecondition:
		return model.NewError(model.CodeInvalidArgument, "etcd rejected the request as invalid").
			WithDetail(detail)
	case code == codes.ResourceExhausted:
		return model.NewError(model.CodeQuotaExceeded, "etcd reported a resource/quota limit").
			WithDetail(detail)
	}
	return model.NewError(model.CodeUnknown, "Unexpected etcd error").
		WithDetail(detail)
}

func isConnRefused(err error) bool {
	var sysErr *net.OpError
	if errors.As(err, &sysErr) && sysErr.Err != nil {
		msg := sysErr.Err.Error()
		if strings.Contains(msg, "connection refused") ||
			strings.Contains(msg, "no route to host") ||
			strings.Contains(msg, "network is unreachable") {
			return true
		}
	}
	return strings.Contains(err.Error(), "connection refused")
}

// ParseLeaseID parses a user supplied lease id ("1234" or empty).
func ParseLeaseID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, model.NewInvalidArgument("lease id must be an integer, got %q", s)
	}
	return id, nil
}

// NormalizeEndpoints applies the correct scheme so TLS vs plaintext is not
// ambiguous: TLS-enabled connections default to https://, others to http://.
// Note that url.Parse treats "host:port" as scheme+opaque, so the check is a
// plain "contains ://" instead.
func NormalizeEndpoints(cfg *model.ConnectionConfig) []string {
	out := make([]string, 0, len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		if !strings.Contains(ep, "://") {
			if cfg.TLS.Enabled {
				ep = "https://" + ep
			} else {
				ep = "http://" + ep
			}
		}
		out = append(out, ep)
	}
	return out
}
