package etcd

import (
	"context"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/model"
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
	if p == "" {
		return "/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ListLevel lists the immediate directories and leaves under prefix.
func ListLevel(ctx context.Context, client *clientv3.Client, prefix, cursor string, pageSize int) (model.LevelPage, error) {
	prefix = NormalizeTreePrefix(prefix)
	pageSize = ClampPageSize(pageSize)
	ctx, cancel := context.WithTimeout(ctx, requestTimeoutFrom(ctx))
	defer cancel()

	start, end := RangeBounds(prefix)
	pos := start
	if cursor != "" {
		if cursor < start || cursor > end {
			return model.LevelPage{}, model.NewInvalidArgument("cursor is outside the directory")
		}
		pos = cursor
	}

	page := model.LevelPage{Prefix: prefix, Cursor: cursor, Rows: []model.LevelRow{}}
	for range maxLevelScanRPCs {
		if pos >= end {
			break
		}
		resp, err := client.Get(ctx, pos, clientv3.WithRange(end), clientv3.WithLimit(segmentScanBatch), clientv3.WithKeysOnly())
		if err != nil {
			return model.LevelPage{}, MapError(err)
		}
		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			if key < pos {
				continue
			}
			rest := key[len(prefix):]
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				name := rest[:slash+1]
				path := prefix + name
				page.Rows = append(page.Rows, model.LevelRow{Type: "segment", Name: name, Path: path})
				pos = PrefixEnd(path)
			} else {
				page.Rows = append(page.Rows, model.LevelRow{
					Type: "leaf", Name: rest, Path: key,
					ModRevision: kv.ModRevision, Version: kv.Version, Lease: kv.Lease,
				})
				pos = NextCursor(key)
			}
			if len(page.Rows) == pageSize {
				if pos >= end {
					return page, nil
				}
				probe, err := client.Get(ctx, pos, clientv3.WithRange(end), clientv3.WithLimit(1), clientv3.WithKeysOnly())
				if err != nil {
					return model.LevelPage{}, MapError(err)
				}
				page.HasMore = len(probe.Kvs) > 0
				page.NextCursor = pos
				return page, nil
			}
		}
		if !resp.More {
			return page, nil
		}
	}
	if pos < end {
		page.NextCursor = pos
		page.HasMore = true
	}
	return page, nil
}
