package etcd

import (
	"context"
	"strings"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"etcd-eye/internal/model"
)

// segmentScanBatch is how many keys-only KVs one RPC fetches while deriving
// a column's rows (segments + direct leaves).
const segmentScanBatch = 500

// maxLevelScanRPCs bounds one page's scan: 200 batches × 500 keys = 100k keys
// examined per page at most.
const maxLevelScanRPCs = 200

// NormalizeTreePrefix turns user input into a directory-style prefix:
// empty becomes "/" and a missing trailing slash is appended. Tree nodes and
// the items list share this form so relative key display stays consistent.
func NormalizeTreePrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ListLevel returns one page of a dynamic browser column: the distinct child
// segments (directories) interleaved with the direct leaf keys under prefix,
// in etcd's byte-lexical order, cursor-paginated.
//
// Cursor contract (no duplicates, no gaps):
//   - the cursor is always the scan position just after the LAST ROW KEPT on
//     the previous page (lastKey + "\x00"), advanced past the whole subtree
//     of the last emitted segment when one ends the page — subtree keys can
//     never be level-rows, so skipping them neither hides nor duplicates
//     anything at this level;
//   - segment dedup happens per call; across pages the cursor position is
//     already past every segment emitted on earlier pages.
func ListLevel(ctx context.Context, client *clientv3.Client, prefix, cursor string, pageSize int) (model.LevelPage, error) {
	prefix = NormalizeTreePrefix(prefix)
	pageSize = ClampPageSize(pageSize)

	cctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	start, end := RangeBounds(prefix)
	pos := start
	if cursor != "" {
		pos = cursor
	}

	page := model.LevelPage{Prefix: prefix, Cursor: cursor, Rows: []model.LevelRow{}}
	seen := map[string]bool{}
	budget := maxLevelScanRPCs

	for len(page.Rows) < pageSize && budget > 0 {
		budget--
		if end != "\x00" && pos >= end {
			break
		}
		resp, err := client.Get(cctx, pos,
			clientv3.WithRange(end),
			clientv3.WithLimit(segmentScanBatch),
			clientv3.WithKeysOnly(),
		)
		if err != nil {
			return model.LevelPage{}, MapError(err)
		}
		if len(resp.Kvs) == 0 {
			break
		}

		batchScannedEnd := ""
		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			batchScannedEnd = key
			rest := key[len(prefix):]
			if i := strings.Index(rest, "/"); i >= 0 {
				seg := rest[:i+1]
				if !seen[seg] {
					seen[seg] = true
					page.Rows = append(page.Rows, model.LevelRow{
						Type: "segment",
						Name: seg,
						Path: prefix + seg,
					})
					if len(page.Rows) >= pageSize {
						break
					}
				}
			} else {
				page.Rows = append(page.Rows, model.LevelRow{
					Type:        "leaf",
					Name:        rest,
					Path:        key,
					ModRevision: kv.ModRevision,
					Version:     kv.Version,
					Lease:       kv.Lease,
				})
				if len(page.Rows) >= pageSize {
					break
				}
			}
		}
		_ = batchScannedEnd

		if len(page.Rows) >= pageSize {
			break
		}
		if !resp.More || len(resp.Kvs) < segmentScanBatch {
			// Whole range scanned.
			break
		}

		// Advance the raw scan position: past the last segment seen in this
		// batch when the batch ended inside its subtree, otherwise past the
		// last scanned key.
		if last := lastSegmentInBatch(prefix, resp.Kvs); last != "" {
			pos = PrefixEnd(prefix + last)
		} else {
			pos = NextCursor(string(resp.Kvs[len(resp.Kvs)-1].Key))
		}
	}

	if len(page.Rows) > 0 {
		last := page.Rows[len(page.Rows)-1]
		next := NextCursor(last.Path)
		if last.Type == "segment" {
			// Skip the segment's whole subtree; it holds no level rows.
			if skip := PrefixEnd(last.Path); skip > next {
				next = skip
			}
		}
		// If any segment row was emitted on this page, the continuation must
		// also sit past the last such subtree (defensive max: subtree keys
		// are never level rows, so this can neither skip nor duplicate one).
		for i := len(page.Rows) - 1; i >= 0; i-- {
			if page.Rows[i].Type == "segment" {
				if skip := PrefixEnd(page.Rows[i].Path); skip > next {
					next = skip
				}
				break
			}
		}
		page.NextCursor = next

		probe, err := client.Get(cctx, next,
			clientv3.WithRange(end),
			clientv3.WithLimit(1),
			clientv3.WithKeysOnly(),
		)
		page.HasMore = err == nil && len(probe.Kvs) > 0
	}
	return page, nil
}

// lastSegmentInBatch returns the last distinct segment name encountered in a
// scanned batch (the one whose subtree the scanner may jump past).
func lastSegmentInBatch(prefix string, kvs []*mvccpb.KeyValue) string {
	last := ""
	for _, kv := range kvs {
		key := string(kv.Key)
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):]
		if i := strings.Index(rest, "/"); i >= 0 {
			seg := rest[:i+1]
			// Only a jump to the LAST segment is valid: anything sorting
			// after it has not been examined yet.
			if seg >= last {
				last = seg
			}
		}
	}
	return last
}
