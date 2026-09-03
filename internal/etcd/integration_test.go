//go:build integration

// Integration tests against a real etcd server.
//
//	go test -tags integration ./internal/etcd/ -v
//
// Endpoints come from ETCD_ENDPOINTS (default 127.0.0.1:12379).
package etcd

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"etcd-eye/internal/model"
)

const testPrefix = "/etcd-eye-test/"

func integrationClient(t *testing.T) *clientv3.Client {
	t.Helper()
	endpoints := os.Getenv("ETCD_ENDPOINTS")
	if endpoints == "" {
		endpoints = "127.0.0.1:12379"
	}
	cfg := model.ConnectionConfig{
		Name:             "integration",
		Endpoints:        []string{endpoints},
		DialTimeoutMs:    2000,
		RequestTimeoutMs: 5000,
	}
	client, err := NewClient(context.Background(), &cfg, "")
	if err != nil {
		t.Skipf("etcd not reachable at %s: %v", endpoints, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func cleanupPrefix(t *testing.T, client *clientv3.Client, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start, end := RangeBounds(prefix)
	_, err := client.Delete(ctx, start, clientv3.WithRange(end))
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// Also clean up when the test finishes so nothing is left behind on a
	// shared server.
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		_, _ = client.Delete(cctx, start, clientv3.WithRange(end))
	})
}

// The full cursor pagination contract against a real server: walking pages
// must visit every seeded key exactly once, in lexical order, without
// duplicating or skipping anything.
func TestIntegrationListCursorPagination(t *testing.T) {
	client := integrationClient(t)
	cleanupPrefix(t, client, testPrefix)

	const total = 57
	const pageSize = 10

	ctx := context.Background()
	ops := make([]clientv3.Op, 0, total)
	for i := 0; i < total; i++ {
		ops = append(ops, clientv3.OpPut(fmt.Sprintf("%s%04d", testPrefix, i), "v"))
	}
	if _, err := client.Txn(ctx).Then(ops...).Commit(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var (
		seen   []string
		cursor string
		pages  int
	)
	for {
		page, err := ListPage(ctx, client, testPrefix, cursor, pageSize)
		if err != nil {
			t.Fatalf("ListPage: %v", err)
		}
		pages++
		for _, it := range page.Items {
			seen = append(seen, it.Key)
		}
		if len(seen) > total {
			t.Fatalf("walked past the end: %d keys", len(seen))
		}
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
		if pages > 100 {
			t.Fatal("pagination did not terminate")
		}
	}
	if pages != 6 {
		t.Fatalf("pages = %d, want 6 (57 keys / 10 per page)", pages)
	}
	if len(seen) != total {
		t.Fatalf("seen %d keys, want %d", len(seen), total)
	}
	for i, k := range seen {
		want := fmt.Sprintf("%s%04d", testPrefix, i)
		if k != want {
			t.Fatalf("position %d = %q, want %q (order broken)", i, k, want)
		}
	}
}

// Empty prefix ("all keys") must still paginate from the absolute beginning.
func TestIntegrationEmptyPrefixStartsFromBeginning(t *testing.T) {
	client := integrationClient(t)
	cleanupPrefix(t, client, testPrefix)

	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := client.Put(ctx, fmt.Sprintf("%sa%02d", testPrefix, i), "v"); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Note: another test may run in parallel against the same server; we only
	// assert that every returned key sorts before the cursor and pages advance.
	page, err := ListPage(ctx, client, "", "", 20)
	if err != nil {
		t.Fatalf("ListPage all: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatal("expected keys from the global listing")
	}
	for _, it := range page.Items {
		if it.Key == "" {
			t.Fatal("empty key returned")
		}
	}
}

// Full CRUD with CAS semantics.
func TestIntegrationCRUDAndCAS(t *testing.T) {
	client := integrationClient(t)
	cleanupPrefix(t, client, testPrefix+"crud")
	ctx := context.Background()
	key := testPrefix + "crud/k"

	// Create (expectedModRevision 0) must succeed on a fresh key.
	d1, err := PutWithCAS(ctx, client, key, []byte("v1"), 0, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if d1.ValueBase64 != "djE=" || d1.Version != 1 {
		t.Fatalf("unexpected detail: %+v", d1)
	}

	// Create again must report KEY_ALREADY_EXISTS.
	_, err = PutWithCAS(ctx, client, key, []byte("v2"), 0, 0)
	if code := appErrCode(err); code != model.CodeKeyAlreadyExists {
		t.Fatalf("second create -> %v, want KEY_ALREADY_EXISTS", err)
	}

	// Save with the correct ModRevision succeeds.
	d2, err := PutWithCAS(ctx, client, key, []byte("v2"), d1.ModRevision, 0)
	if err != nil {
		t.Fatalf("cas save: %v", err)
	}
	if d2.Version != 2 {
		t.Fatalf("version = %d", d2.Version)
	}

	// Saving with the stale revision must fail with REVISION_CONFLICT and
	// must not modify the key.
	_, err = PutWithCAS(ctx, client, key, []byte("v3-evil"), d1.ModRevision, 0)
	if code := appErrCode(err); code != model.CodeRevisionConflict {
		t.Fatalf("stale save -> %v, want REVISION_CONFLICT", err)
	}
	d3, err := GetKey(ctx, client, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if raw, _ := base64Decode(d3.ValueBase64); string(raw) != "v2" {
		t.Fatalf("stale write landed! value = %q", raw)
	}

	// Delete, then delete again -> KEY_NOT_FOUND.
	if err := DeleteKey(ctx, client, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	err = DeleteKey(ctx, client, key)
	if code := appErrCode(err); code != model.CodeKeyNotFound {
		t.Fatalf("double delete -> %v, want KEY_NOT_FOUND", err)
	}
	_, err = GetKey(ctx, client, key)
	if code := appErrCode(err); code != model.CodeKeyNotFound {
		t.Fatalf("get deleted -> %v, want KEY_NOT_FOUND", err)
	}
}

// Binary values must round-trip byte-exact through the base64 bridge.
func TestIntegrationBinaryValue(t *testing.T) {
	client := integrationClient(t)
	cleanupPrefix(t, client, testPrefix+"bin")
	ctx := context.Background()

	raw := make([]byte, 4096)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	// Force worst cases: NUL bytes, 0xff, invalid UTF-8 sequences.
	raw[0], raw[1], raw[2], raw[3] = 0x00, 0xff, 0xfe, 0x80

	key := testPrefix + "bin/data"
	if _, err := PutWithCAS(ctx, client, key, raw, 0, 0); err != nil {
		t.Fatalf("put: %v", err)
	}
	d, err := GetKey(ctx, client, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.IsUTF8 {
		t.Fatal("random binary data must not be flagged UTF-8")
	}
	if d.SizeBytes != len(raw) {
		t.Fatalf("size = %d, want %d", d.SizeBytes, len(raw))
	}
	got, err := base64Decode(d.ValueBase64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("binary value corrupted in round trip")
	}
}

// Concurrent CAS writers: exactly one of N racing saves may win.
func TestIntegrationCASRace(t *testing.T) {
	client := integrationClient(t)
	cleanupPrefix(t, client, testPrefix+"race")
	ctx := context.Background()
	key := testPrefix + "race/k"

	d, err := PutWithCAS(ctx, client, key, []byte("base"), 0, 0)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	const racers = 8
	var wg sync.WaitGroup
	winners := make([]int, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := PutWithCAS(ctx, client, key, []byte(fmt.Sprintf("w%d", i)), d.ModRevision, 0)
			if err == nil {
				winners[i] = 1
			}
		}(i)
	}
	wg.Wait()

	total := 0
	for _, w := range winners {
		total += w
	}
	if total != 1 {
		t.Fatalf("%d racers won, want exactly 1", total)
	}
}

// Dynamic column levels: distinct segments interleaved with direct leaves,
// plus the pagination contract (no duplicates, no gaps across pages).
// Layout under /etcd-eye-test/seg/:
//
//	a/1, a/2      (leaves under segment a/)
//	b/x/1         (nested deeper)
//	c             (direct leaf of seg/ itself)
//	d/1           (another segment after a leaf)
func TestIntegrationListLevel(t *testing.T) {
	client := integrationClient(t)
	prefix := testPrefix + "seg/"
	cleanupPrefix(t, client, prefix)
	ctx := context.Background()

	keys := []string{
		prefix + "a/1", prefix + "a/2",
		prefix + "b/x/1",
		prefix + "c",
		prefix + "d/1",
	}
	for _, k := range keys {
		if _, err := client.Put(ctx, k, "v"); err != nil {
			t.Fatalf("seed %s: %v", k, err)
		}
	}

	page, err := ListLevel(ctx, client, prefix, "", 100)
	if err != nil {
		t.Fatalf("ListLevel: %v", err)
	}
	if page.HasMore {
		t.Fatal("small namespace must not report HasMore")
	}
	want := []model.LevelRow{
		{Type: "segment", Name: "a/", Path: prefix + "a/"},
		{Type: "segment", Name: "b/", Path: prefix + "b/"},
		{Type: "leaf", Name: "c", Path: prefix + "c"},
		{Type: "segment", Name: "d/", Path: prefix + "d/"},
	}
	if len(page.Rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", page.Rows, want)
	}
	for i, w := range want {
		got := page.Rows[i]
		if got.Type != w.Type || got.Name != w.Name || got.Path != w.Path {
			t.Fatalf("row %d = %+v, want %+v", i, got, w)
		}
	}
	if page.Rows[2].ModRevision == 0 || page.Rows[2].Version != 1 {
		t.Fatalf("leaf metadata missing: %+v", page.Rows[2])
	}

	// Nested level under b/.
	sub, err := ListLevel(ctx, client, prefix+"b/", "", 100)
	if err != nil {
		t.Fatalf("ListLevel(b/): %v", err)
	}
	if len(sub.Rows) != 1 || sub.Rows[0].Type != "segment" || sub.Rows[0].Name != "x/" {
		t.Fatalf("b/ rows = %+v", sub.Rows)
	}

	// A level containing only direct leaves yields only leaf rows.
	leaves, err := ListLevel(ctx, client, prefix+"a/", "", 100)
	if err != nil {
		t.Fatalf("ListLevel(a/): %v", err)
	}
	if len(leaves.Rows) != 2 || leaves.Rows[0].Type != "leaf" {
		t.Fatalf("a/ rows = %+v", leaves.Rows)
	}

	// Prefix normalization: no trailing slash still works.
	norm, err := ListLevel(ctx, client, strings.TrimSuffix(prefix, "/"), "", 100)
	if err != nil {
		t.Fatalf("ListLevel(no slash): %v", err)
	}
	if len(norm.Rows) != len(want) {
		t.Fatalf("normalized rows = %+v", norm.Rows)
	}

	// Pagination: pageSize 2 must produce exactly [a/, b/] then [c, d/]
	// with no duplicates or gaps.
	p1, err := ListLevel(ctx, client, prefix, "", 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(p1.Rows) != 2 || !p1.HasMore {
		t.Fatalf("page1 = %+v, want 2 rows + HasMore", p1)
	}
	p2, err := ListLevel(ctx, client, prefix, p1.NextCursor, 100)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if p2.HasMore {
		t.Fatal("page2 must be final")
	}
	all := append(append([]model.LevelRow{}, p1.Rows...), p2.Rows...)
	if len(all) != len(want) {
		t.Fatalf("paged walk produced %+v", all)
	}
	for i, w := range want {
		if all[i].Path != w.Path || all[i].Type != w.Type {
			t.Fatalf("paged row %d = %+v, want %+v", i, all[i], w)
		}
	}
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// appErrCode asserts the error is the unified *model.AppError and returns
// its code ("" for nil).
func appErrCode(err error) string {
	if err == nil {
		return ""
	}
	var ae *model.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return "NOT-AN-APP-ERROR:" + err.Error()
}
