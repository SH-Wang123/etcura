package protodec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree creates a nested proto project and returns its root.
func writeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"api/v1/common.proto": `syntax = "proto3";
package shop.v1;
message Money { int64 cents = 1; string currency = 2; }
`,
		"api/v1/order.proto": `syntax = "proto3";
package shop.v1;
import "api/v1/common.proto";
import "google/protobuf/timestamp.proto";
message Order {
  string id = 1;
  Money total = 2;
  google.protobuf.Timestamp created_at = 3;
}
`,
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func p(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

// Regression: imports written as directory-relative paths used to fail because
// files were flattened into a staging directory (lost their layout).
func TestImportDirectoryRelativePaths(t *testing.T) {
	root := writeTree(t)
	im := NewImporter()
	files, err := im.Import(context.Background(), []string{
		p(root, "api/v1/order.proto"),
		p(root, "api/v1/common.proto"),
	})
	if err != nil {
		t.Fatalf("Import with directory-relative import failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	// Order + Money (google.protobuf.Timestamp comes from standard imports and
	// is not part of the imported set).
	if im.MessageCount() != 2 {
		t.Fatalf("message count = %d, want 2", im.MessageCount())
	}
	// The imported file is identified by the path its importer uses, so users
	// can tell which file resolved.
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	if !contains(names, "api/v1/common.proto") {
		t.Fatalf("import-derived name missing: %v", names)
	}
	if names[0] == names[1] {
		t.Fatalf("file names must be distinct: %v", names)
	}
}

// Two files sharing a base name in different directories must stay distinct.
func TestImportSameBaseNameDisambiguated(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a/common.proto", `syntax = "proto3";
package pa;
message A { string x = 1; }
`)
	write("b/common.proto", `syntax = "proto3";
package pb;
message B { string y = 1; }
`)
	im := NewImporter()
	files, err := im.Import(context.Background(), []string{
		p(root, "a/common.proto"), p(root, "b/common.proto"),
	})
	if err != nil {
		t.Fatalf("same-base-name import failed: %v", err)
	}
	if len(files) != 2 || files[0].Name == files[1].Name {
		t.Fatalf("names must be disambiguated, got: %+v", files)
	}
	if im.MessageCount() != 2 {
		t.Fatalf("message count = %d, want 2", im.MessageCount())
	}
}

// Well-known types resolve even when the user did not select them.
func TestImportWellKnownTypesNotSelected(t *testing.T) {
	root := writeTree(t)
	im := NewImporter()
	if _, err := im.Import(context.Background(), []string{p(root, "api/v1/order.proto"), p(root, "api/v1/common.proto")}); err != nil {
		t.Fatalf("well-known import should resolve via standard imports: %v", err)
	}
}

// Nested and deep relative paths (api/v1/...) must resolve from any selection.
func TestImportSubsetStillResolves(t *testing.T) {
	root := writeTree(t)
	im := NewImporter()
	// only select the importing file's own directory + the imported one
	if _, err := im.Import(context.Background(), []string{p(root, "api/v1/order.proto"), p(root, "api/v1/common.proto")}); err != nil {
		t.Fatalf("subset import failed: %v", err)
	}
}

// A genuinely missing import must produce an actionable message.
func TestImportMissingImportMessage(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "a", "needs_missing.proto")
	if err := os.MkdirAll(filepath.Dir(bad), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(`syntax = "proto3";
import "does/not/exist.proto";
message A { string x = 1; }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	im := NewImporter()
	_, err := im.Import(context.Background(), []string{bad})
	if err == nil {
		t.Fatal("missing import must fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "could not be resolved") {
		t.Fatalf("expected actionable missing-import message, got: %s", msg)
	}
	if !strings.Contains(msg, "does/not/exist.proto") {
		t.Fatalf("detail should name the missing import, got: %s", msg)
	}
	if !strings.Contains(msg, "selected files") {
		t.Fatalf("detail should list selected files for mapping, got: %s", msg)
	}
}

// Same file selected twice must not be treated as a duplicate error.
func TestImportDuplicateSelectionIsIdempotent(t *testing.T) {
	root := writeTree(t)
	order := p(root, "api/v1/order.proto")
	im := NewImporter()
	files, err := im.Import(context.Background(), []string{order, order, p(root, "api/v1/common.proto")})
	if err != nil {
		t.Fatalf("duplicate selection should be tolerated: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// Selecting a directory imports every .proto inside it recursively, which is
// how real projects are shared (files import each other by relative path).
func TestImportDirectoryRecursive(t *testing.T) {
	root := writeTree(t)
	im := NewImporter()
	files, err := im.Import(context.Background(), []string{root})
	if err != nil {
		t.Fatalf("directory import failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if im.MessageCount() != 2 {
		t.Fatalf("message count = %d, want 2", im.MessageCount())
	}
}

// An empty directory must produce a clear error, not a silent no-op.
func TestImportEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	im := NewImporter()
	_, err := im.Import(context.Background(), []string{dir})
	if err == nil {
		t.Fatal("empty directory must fail")
	}
	if !strings.Contains(err.Error(), "no .proto files found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Regression: macOS AppleDouble companions ("._foo.proto") are binary blobs
// that must be ignored, not parsed. Reported from a real project that had been
// copied/synced, where scanning the folder picked up "._metanode.proto".
func TestImportSkipsMacOSMetadataFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "pb")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// real proto files
	if err := os.WriteFile(filepath.Join(dir, "metanode.proto"), []byte(`syntax = "proto3";
package pb;
message MetaNode { string id = 1; }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.proto"), []byte(`syntax = "proto3";
package pb;
message Metadata { string key = 1; }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	// AppleDouble companion: binary, extension still ".proto"
	appleDouble := append([]byte{0x00, 0x05, 0x16, 0x07, 0x00, 0x02, 0x00, 0x00}, []byte("Mac OS X")...)
	if err := os.WriteFile(filepath.Join(dir, "._metanode.proto"), appleDouble, 0o600); err != nil {
		t.Fatal(err)
	}
	// and a .DS_Store for good measure
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}

	im := NewImporter()
	files, err := im.Import(context.Background(), []string{dir})
	if err != nil {
		t.Fatalf("import must ignore metadata files, got: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 (metadata files skipped)", len(files))
	}
	for _, f := range files {
		if strings.Contains(f.Name, "._") {
			t.Fatalf("AppleDouble file was imported: %s", f.Name)
		}
	}
	if im.MessageCount() != 2 {
		t.Fatalf("message count = %d, want 2", im.MessageCount())
	}
}

// Explicitly selecting only metadata files must produce a clear message.
func TestImportOnlyMetadataFiles(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "._x.proto")
	if err := os.WriteFile(bad, []byte{0x00, 0x05, 0x16, 0x07}, 0o600); err != nil {
		t.Fatal(err)
	}
	im := NewImporter()
	_, err := im.Import(context.Background(), []string{bad})
	if err == nil {
		t.Fatal("selecting only metadata files must fail")
	}
	if !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("error should explain the metadata case, got: %v", err)
	}
}

// A binary file that is not an AppleDouble name must still be named clearly.
func TestImportBinaryFileNamedClearly(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "weird.proto")
	if err := os.WriteFile(bin, []byte{0x00, 0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatal(err)
	}
	im := NewImporter()
	_, err := im.Import(context.Background(), []string{bin})
	if err == nil {
		t.Fatal("binary proto file must fail with a clear error")
	}
	if !strings.Contains(err.Error(), "not a text proto file") {
		t.Fatalf("error should identify the file as non-text, got: %v", err)
	}
	if !strings.Contains(err.Error(), "weird.proto") {
		t.Fatalf("error should name the offending file, got: %v", err)
	}
}
