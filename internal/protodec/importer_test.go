package protodec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoProto = `syntax = "proto3";
package demo;
import "common.proto";
message Order {
  string order_id = 1;
  int32 quantity = 2;
  repeated string tags = 3;
  demo.Status status = 4;
}
`
const userProto = `syntax = "proto3";
package demo;
message User {
  string name = 1;
  int32 age = 2;
}
`
const commonProtoFixed = `syntax = "proto3";
package demo;
enum Status {
  STATUS_UNKNOWN = 0;
  STATUS_OK = 1;
}
message PageMeta {
  map<string, string> labels = 1;
}
`

func writeTestProtos(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"common.proto": commonProtoFixed,
		"demo.proto":   demoProto,
		"user.proto":   userProto,
	}
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

// orderBlob encodes Order{order_id:"ord-1", quantity:3, tags:["a","b"]} by
// hand (field 1 string, field 2 varint, field 3 repeated string).
func orderBlob() []byte {
	var b []byte
	b = append(b, 0x0a, 0x05)
	b = append(b, "ord-1"...)
	b = append(b, 0x10, 0x03)
	b = append(b, 0x1a, 0x01, 'a')
	b = append(b, 0x1a, 0x01, 'b')
	return b
}

func TestImportAndMatch(t *testing.T) {
	im := NewImporter()
	paths := writeTestProtos(t)

	files, err := im.Import(context.Background(), paths)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3", len(files))
	}
	// Order + PageMeta (map entry skipped) + User = 3 messages.
	if im.MessageCount() != 3 {
		t.Fatalf("message count = %d, want 3", im.MessageCount())
	}
	if im.Files()[0].Package != "demo" {
		t.Fatalf("package = %q", im.Files()[0].Package)
	}

	res := im.Match(orderBlob())
	if len(res.Candidates) == 0 {
		t.Fatal("expected at least one candidate for a valid Order payload")
	}
	if res.Candidates[0].MessageName != "demo.Order" {
		t.Fatalf("best candidate = %s, want demo.Order", res.Candidates[0].MessageName)
	}
	// protojson emits spaces after commas; normalize before asserting.
	compact := strings.ReplaceAll(res.Candidates[0].JSON, " ", "")
	if !strings.Contains(compact, `"order_id":"ord-1"`) {
		t.Fatalf("JSON = %s", res.Candidates[0].JSON)
	}
	if !strings.Contains(compact, `"tags":["a","b"]`) {
		t.Fatalf("JSON = %s", res.Candidates[0].JSON)
	}
	// demo.User (fields 1,2 only) also decodes but must rank below Order,
	// which covers all three wire tags.
	if len(res.Candidates) < 2 || res.Candidates[1].MessageName != "demo.User" {
		var names []string
		for _, c := range res.Candidates {
			names = append(names, c.MessageName)
		}
		t.Fatalf("candidates = %v, want demo.Order ranked above demo.User", names)
	}
	if res.Candidates[0].Score <= res.Candidates[1].Score {
		t.Fatalf("Order score %d must exceed User score %d",
			res.Candidates[0].Score, res.Candidates[1].Score)
	}
}

func TestMatchTextIsNotProto(t *testing.T) {
	im := NewImporter()
	if _, err := im.Import(context.Background(), writeTestProtos(t)); err != nil {
		t.Fatal(err)
	}
	res := im.Match([]byte(`{"order_id":"this is plain JSON"}`))
	if len(res.Candidates) != 0 {
		t.Fatalf("plain JSON must not match, got %+v", res.Candidates)
	}
}

func TestMatchWithoutImport(t *testing.T) {
	im := NewImporter()
	res := im.Match(orderBlob())
	if len(res.Candidates) != 0 {
		t.Fatalf("no protos imported: candidates = %+v", res.Candidates)
	}
}

func TestMatchEmptyAndTooLarge(t *testing.T) {
	im := NewImporter()
	res := im.Match(nil)
	if len(res.Candidates) != 0 || res.ValueTooLarge {
		t.Fatalf("empty value: %+v", res)
	}
	big := make([]byte, MaxMatchValueBytes+1)
	res = im.Match(big)
	if !res.ValueTooLarge {
		t.Fatal("oversized value must be flagged")
	}
}

func TestImportMissingFile(t *testing.T) {
	im := NewImporter()
	if _, err := im.Import(context.Background(), []string{"/nonexistent/x.proto"}); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestImportDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o700)
	os.WriteFile(filepath.Join(dir, "a.proto"), []byte(userProto), 0o600)
	os.WriteFile(filepath.Join(sub, "a.proto"), []byte(demoProto), 0o600)
	im := NewImporter()
	if _, err := im.Import(context.Background(), []string{
		filepath.Join(dir, "a.proto"), filepath.Join(sub, "a.proto"),
	}); err == nil {
		t.Fatal("duplicate base names must fail with a clear error")
	}
}

func TestClearResetsImporter(t *testing.T) {
	im := NewImporter()
	if _, err := im.Import(context.Background(), writeTestProtos(t)); err != nil {
		t.Fatal(err)
	}
	im.Clear()
	if im.MessageCount() != 0 || len(im.Files()) != 0 {
		t.Fatal("Clear must reset importer state")
	}
}
