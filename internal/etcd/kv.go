package etcd

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"
	"unicode/utf8"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/model"
)

// ListPage fetches one server-side page of keys under prefix, starting at the
// opaque cursor ("" = the beginning of the range). It asks etcd for
// pageSize+1 keys-only KVs so hasMore can be derived without a second request
// or any counting scan; the +1 overflow row never reaches the UI.
func ListPage(ctx context.Context, client *clientv3.Client, prefix, cursor string, pageSize int) (model.KeyPage, error) {
	pageSize = ClampPageSize(pageSize)

	start, end := RangeBounds(prefix)
	if cursor != "" {
		start = cursor
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	resp, err := client.Get(cctx, start,
		clientv3.WithRange(end),
		clientv3.WithLimit(int64(pageSize+1)),
		clientv3.WithKeysOnly(),
	)
	if err != nil {
		return model.KeyPage{}, MapError(err)
	}
	page := BuildPage(prefix, cursor, pageSize, resp.Kvs)
	page.Revision = resp.Header.Revision
	return page, nil
}

// GetKey fetches one key with its full value and metadata. Values travel as
// base64 and are flagged UTF-8 or binary so the UI can pick a safe view.
func GetKey(ctx context.Context, client *clientv3.Client, key string) (model.KvDetail, error) {
	if key == "" {
		return model.KvDetail{}, model.NewInvalidArgument("key must not be empty")
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	resp, err := client.Get(cctx, key)
	if err != nil {
		return model.KvDetail{}, MapError(err)
	}
	if resp.Count == 0 {
		return model.KvDetail{}, model.NewError(model.CodeKeyNotFound, "Key not found: %s", key)
	}
	return DetailFromKV(resp.Kvs[0]), nil
}

// DetailFromKV converts a raw etcd KV into the wire detail. The base64
// round-trip guarantees binary values survive JSON serialization untouched.
func DetailFromKV(kv *mvccpb.KeyValue) model.KvDetail {
	value := kv.Value
	return model.KvDetail{
		Key:            string(kv.Key),
		ValueBase64:    base64.StdEncoding.EncodeToString(value),
		IsUTF8:         utf8.Valid(value),
		SizeBytes:      len(value),
		CreateRevision: kv.CreateRevision,
		ModRevision:    kv.ModRevision,
		Version:        kv.Version,
		Lease:          kv.Lease,
	}
}

// PutWithCAS writes value to key guarded by a ModRevision compare. It is the
// only write path in the app, so two admins (or two tabs) can never silently
// overwrite each other. expectedModRevision 0 means "only create if absent".
// Returns the fresh detail after the write.
func PutWithCAS(ctx context.Context, client *clientv3.Client, key string, value []byte, expectedModRevision, leaseID int64) (model.KvDetail, error) {
	if key == "" {
		return model.KvDetail{}, model.NewInvalidArgument("key must not be empty")
	}

	put := clientv3.OpPut(key, string(value))
	if leaseID != 0 {
		put = clientv3.OpPut(key, string(value), clientv3.WithLease(clientv3.LeaseID(leaseID)))
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	txn, err := client.Txn(cctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", expectedModRevision)).
		Then(put).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return model.KvDetail{}, MapError(err)
	}
	if !txn.Succeeded {
		// The Else branch ran: the key changed under us, already exists
		// (create path), or vanished (update path).
		var current *mvccpb.KeyValue
		if len(txn.Responses) > 0 {
			if getResp := txn.Responses[0].GetResponseRange(); getResp != nil && getResp.Count > 0 {
				current = getResp.Kvs[0]
			}
		}
		if expectedModRevision == 0 {
			if current != nil {
				return model.KvDetail{}, model.NewError(model.CodeKeyAlreadyExists,
					"Key already exists: %s (ModRevision %d)", key, current.ModRevision)
			}
			return model.KvDetail{}, model.NewError(model.CodeKeyNotFound, "Key no longer exists: %s", key)
		}
		if current != nil {
			return model.KvDetail{}, model.NewError(model.CodeRevisionConflict,
				"The key has been modified by another client (current ModRevision %d, expected %d). Reload before saving.",
				current.ModRevision, expectedModRevision).
				WithDetail("current_mod_revision=" + strconv.FormatInt(current.ModRevision, 10))
		}
		return model.KvDetail{}, model.NewError(model.CodeKeyNotFound, "Key no longer exists: %s", key)
	}

	return GetKey(cctx, client, key)
}

// DeleteKey removes exactly one key. There is deliberately no prefix variant:
// mass deletion is out of scope and dangerous.
func DeleteKey(ctx context.Context, client *clientv3.Client, key string) error {
	if key == "" {
		return model.NewInvalidArgument("key must not be empty")
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	resp, err := client.Delete(cctx, key)
	if err != nil {
		return MapError(err)
	}
	if resp.Deleted == 0 {
		return model.NewError(model.CodeKeyNotFound, "Key not found: %s", key)
	}
	return nil
}

// requestTimeoutFrom reads the per-request deadline configured on the parent
// context (set by the app layer from the active connection profile). Falls
// back to the default so no RPC can ever run without a timeout.
func requestTimeoutFrom(ctx context.Context) time.Duration {
	if v, ok := ctx.Value(requestTimeoutKey{}).(time.Duration); ok && v > 0 {
		return v
	}
	return model.DefaultRequestTimeout
}

type requestTimeoutKey struct{}

// WithRequestTimeout stores the profile's request timeout in the context so
// the kv layer applies it to every RPC it issues.
func WithRequestTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, requestTimeoutKey{}, d)
}
