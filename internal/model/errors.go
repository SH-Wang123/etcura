package model

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// Error codes returned to the frontend inside AppError.Code.
const (
	CodeConnectionFailed  = "CONNECTION_FAILED"
	CodeNotConnected      = "NOT_CONNECTED"
	CodeTimeout           = "TIMEOUT"
	CodeCancelled         = "CANCELLED"
	CodeKeyNotFound       = "KEY_NOT_FOUND"
	CodeKeyAlreadyExists  = "KEY_ALREADY_EXISTS"
	CodeRevisionConflict  = "REVISION_CONFLICT"
	CodeLeaseNotFound     = "LEASE_NOT_FOUND"
	CodePermissionDenied  = "PERMISSION_DENIED"
	CodeTLSError          = "TLS_ERROR"
	CodeInvalidArgument   = "INVALID_ARGUMENT"
	CodeQuotaExceeded     = "QUOTA_EXCEEDED"
	CodeConfigError       = "CONFIG_ERROR"
	CodeUnknown           = "UNKNOWN"
)

// AppError is the unified error model returned to the UI. Wails serialises a
// returned error as its Error() string, so Error() returns the JSON encoding
// of the struct and the frontend can parse it back into a typed object.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"code":%q,"message":%q}`, e.Code, e.Message)
	}
	return string(b)
}

func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}

func NewError(code, format string, args ...interface{}) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func NewInvalidArgument(format string, args ...interface{}) *AppError {
	return NewError(CodeInvalidArgument, format, args...)
}

// trimSpace is strings.TrimSpace without importing strings at each call site.
func trimSpace(s string) string { return strings.TrimSpace(s) }

// EndpointValid accepts "host:port" or "scheme://host:port" forms.
func EndpointValid(ep string) bool {
	if ep == "" {
		return false
	}
	if i := strings.Index(ep, "://"); i >= 0 {
		scheme := ep[:i]
		switch scheme {
		case "http", "https", "unix", "unixs":
		default:
			return false
		}
		ep = ep[i+3:]
	}
	host, port, err := net.SplitHostPort(ep)
	return err == nil && host != "" && port != ""
}
