package etcd

import (
	"go.etcd.io/etcd/api/v3/mvccpb"

	"etcura/internal/model"
)

const (
	// DefaultPageSize is the key-list page size used when the UI does not
	// ask for something specific.
	DefaultPageSize = 200
	// MaxPageSize is the hard upper bound. The backend clamps every request
	// so a runaway client can never ask for one million keys at once.
	MaxPageSize = 1000
	// cursorSentinel is appended to the last key of a page to build the range
	// start of the next page. Because 0x00 is the smallest byte value, key+"\x00"
	// is the first possible position strictly after `key` in etcd's byte-wise
	// lexical order.
	cursorSentinel = "\x00"
	// allKeysStart / allKeysEnd implement "browse every key" using etcd's
	// from-key semantics: a range end of "\x00" means "no upper bound".
	allKeysStart = "\x00"
	allKeysEnd   = "\x00"
)

// ClampPageSize forces every incoming page size into [1, MaxPageSize].
func ClampPageSize(n int) int {
	if n <= 0 {
		return DefaultPageSize
	}
	if n > MaxPageSize {
		return MaxPageSize
	}
	return n
}

// RangeBounds returns the half-open etcd range [start, end) that covers every
// key sharing `prefix`, computed with the same algorithm as clientv3.WithPrefix.
// An empty prefix means "all keys" and maps to etcd's unbounded range.
func RangeBounds(prefix string) (start, end string) {
	if prefix == "" {
		return allKeysStart, allKeysEnd
	}
	return prefix, PrefixEnd(prefix)
}

// PrefixEnd returns the smallest key that sorts after every key beginning with
// prefix (lexicographic, byte-wise). It increments the last byte that is not
// 0xff and truncates; an all-0xff (or empty) prefix has no upper bound and
// therefore yields etcd's unbounded-end sentinel "\x00".
func PrefixEnd(prefix string) string {
	if prefix == "" {
		return allKeysEnd
	}
	b := []byte(prefix)
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < 0xff {
			b[i]++
			return string(b[:i+1])
		}
	}
	return allKeysEnd
}

// NextCursor builds the range start for the page after one ending at lastKey.
// lastKey + "\x00" is the first position strictly after lastKey and is a
// legal key itself, so nothing is skipped.
func NextCursor(lastKey string) string {
	return lastKey + cursorSentinel
}

// BuildPage turns the raw server response (pageSize+1 keys-only KVs) into the
// wire page: at most pageSize summaries, a hasMore flag and the next cursor.
// It is pure so the cursor contract can be unit tested without a server.
func BuildPage(prefix, cursor string, pageSize int, kvs []*mvccpb.KeyValue) model.KeyPage {
	hasMore := len(kvs) > pageSize
	if hasMore {
		kvs = kvs[:pageSize]
	}
	page := model.KeyPage{
		Prefix:  prefix,
		Cursor:  cursor,
		HasMore: hasMore,
		Items:   make([]model.KeySummary, 0, len(kvs)),
	}
	for _, kv := range kvs {
		page.Items = append(page.Items, model.KeySummary{
			Key:            string(kv.Key),
			CreateRevision: kv.CreateRevision,
			ModRevision:    kv.ModRevision,
			Version:        kv.Version,
			Lease:          kv.Lease,
		})
	}
	if hasMore && len(kvs) > 0 {
		page.NextCursor = NextCursor(string(kvs[len(kvs)-1].Key))
	}
	return page
}
