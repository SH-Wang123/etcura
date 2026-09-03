package etcd

import (
	"fmt"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestPrefixEnd(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{"", "\x00"},                  // all keys: unbounded end
		{"/", "0"},                    // '/'=0x2F -> 0x30
		{"/registry/", "/registry0"},  // increment last byte
		{"/a\x00", "/a\x01"},          // 0x00 -> 0x01
		{"\xff", "\x00"},              // all 0xff: no upper bound
		{"/a\xff", "/b"},              // carry into previous byte
	}
	for _, c := range cases {
		if got := PrefixEnd(c.prefix); got != c.want {
			t.Errorf("PrefixEnd(%q) = %q, want %q", c.prefix, got, c.want)
		}
	}
}

func TestPrefixEndCoversAllPrefixedKeys(t *testing.T) {
	// Any key beginning with the prefix must sort inside [prefix, prefixEnd).
	prefix := "/services/"
	end := PrefixEnd(prefix)
	inside := []string{
		"/services/",
		"/services/a",
		"/services/abc/def",
		"/services/\xff\xff",
		"/services/z\x00\x00",
	}
	for _, k := range inside {
		if !(prefix <= k && k < end) {
			t.Errorf("key %q not inside [%q, %q)", k, prefix, end)
		}
	}
	// Keys just outside the prefix must not be covered.
	outside := []string{"/servicet", "/servicer", "/servics", "/t"}
	for _, k := range outside {
		if k < end && k >= prefix {
			t.Errorf("key %q unexpectedly inside [%q, %q)", k, prefix, end)
		}
	}
}

func TestRangeBounds(t *testing.T) {
	start, end := RangeBounds("")
	if start != "\x00" || end != "\x00" {
		t.Fatalf("RangeBounds(\"\") = %q, %q", start, end)
	}
	start, end = RangeBounds("/foo/")
	if start != "/foo/" || end != "/foo0" {
		t.Fatalf("RangeBounds(/foo/) = %q, %q", start, end)
	}
}

func TestNextCursor(t *testing.T) {
	// lastKey + "\x00" must be strictly greater than lastKey but smaller than
	// any key that extends it, so no continuation key is skipped.
	last := "/benchmark/00000010"
	cur := NextCursor(last)
	if !(last < cur) {
		t.Fatalf("cursor %q must be > lastKey %q", cur, last)
	}
	if !(cur < last+"a") {
		t.Fatalf("cursor %q must be < %q", cur, last+"a")
	}
	if cur != last+"\x00" {
		t.Fatalf("cursor must be lastKey+\\x00, got %q", cur)
	}
}

func TestClampPageSize(t *testing.T) {
	if got := ClampPageSize(0); got != DefaultPageSize {
		t.Fatalf("ClampPageSize(0) = %d", got)
	}
	if got := ClampPageSize(-5); got != DefaultPageSize {
		t.Fatalf("negative page size must fall back to default, got %d", got)
	}
	if got := ClampPageSize(50); got != 50 {
		t.Fatalf("got %d", got)
	}
	if got := ClampPageSize(1_000_000); got != MaxPageSize {
		t.Fatalf("oversized page size must be clamped to %d, got %d", MaxPageSize, got)
	}
}

func kv(key string) *mvccpb.KeyValue { return &mvccpb.KeyValue{Key: []byte(key)} }

func TestBuildPage(t *testing.T) {
	t.Run("hasMore with 201 kvs and page size 200", func(t *testing.T) {
		kvs := make([]*mvccpb.KeyValue, 201)
		for i := range kvs {
			kvs[i] = kv(fmt.Sprintf("/p/%06d", i))
		}
		page := BuildPage("/p/", "/p/", 200, kvs)
		if len(page.Items) != 200 {
			t.Fatalf("items = %d, want 200", len(page.Items))
		}
		if !page.HasMore {
			t.Fatal("hasMore must be true")
		}
		wantNext := NextCursor(fmt.Sprintf("/p/%06d", 199))
		if page.NextCursor != wantNext {
			t.Fatalf("nextCursor = %q, want %q", page.NextCursor, wantNext)
		}
		if page.Items[0].Key != "/p/000000" {
			t.Fatalf("first key = %q", page.Items[0].Key)
		}
		if page.Items[199].Key != fmt.Sprintf("/p/%06d", 199) {
			t.Fatalf("last key = %q (overflow row must be dropped)", page.Items[199].Key)
		}
	})
	t.Run("exact page means no more", func(t *testing.T) {
		kvs := make([]*mvccpb.KeyValue, 200)
		for i := range kvs {
			kvs[i] = kv(fmt.Sprintf("/p/%06d", i))
		}
		page := BuildPage("/p/", "/p/", 200, kvs)
		if page.HasMore || page.NextCursor != "" {
			t.Fatalf("hasMore=%v nextCursor=%q", page.HasMore, page.NextCursor)
		}
	})
	t.Run("empty page", func(t *testing.T) {
		page := BuildPage("/p/", "/p/", 200, nil)
		if page.HasMore || len(page.Items) != 0 || page.NextCursor != "" {
			t.Fatalf("unexpected page: %+v", page)
		}
	})
	t.Run("metadata survives keysOnly", func(t *testing.T) {
		kv := &mvccpb.KeyValue{Key: []byte("/k"), CreateRevision: 11, ModRevision: 22, Version: 3, Lease: 44}
		page := BuildPage("", "", 10, []*mvccpb.KeyValue{kv})
		got := page.Items[0]
		if got.CreateRevision != 11 || got.ModRevision != 22 || got.Version != 3 || got.Lease != 44 {
			t.Fatalf("metadata lost: %+v", got)
		}
	})
}
