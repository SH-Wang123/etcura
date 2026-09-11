package protodec

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoregistry"

	"etcura/internal/model"
)

// fileEntry is one selected proto file read into memory.
type fileEntry struct {
	abs  string // slash-normalized absolute path
	data string
}

// protoSet holds the selected .proto files addressed by the import path each
// file is referenced by in the sources. Naming follows the imports rather than
// the on-disk layout, so a selection made from any directory resolves the same
// way `protoc -I <root>` would.
type protoSet struct {
	sources map[string]string // canonical import path -> source text
	origin  map[string]string // canonical import path -> absolute path on disk
	names   []string          // canonical paths, sorted (deterministic matching)
}

// setResolver resolves proto imports against the in-memory set. Well-known
// types (google/protobuf/*.proto) are served by WithStandardImports layered on
// top, so they resolve even when the user did not select them.
type setResolver struct {
	set *protoSet
}

func (r *setResolver) FindFileByPath(importPath string) (protocompile.SearchResult, error) {
	key := path.Clean(filepath.ToSlash(strings.TrimSpace(importPath)))
	if key == "." || key == "" {
		return protocompile.SearchResult{}, protoregistry.NotFound
	}

	// 1) exact canonical match (the common case: names come from imports).
	if src, ok := r.set.sources[key]; ok {
		return protocompile.SearchResult{Source: strings.NewReader(src)}, nil
	}

	// 2) suffix match: the import may include more (or fewer) leading
	// directories than the canonical name, depending on where the files live.
	var matches []string
	for _, name := range r.set.names {
		if strings.HasSuffix(name, "/"+key) {
			matches = append(matches, name)
		}
	}
	switch len(matches) {
	case 1:
		return protocompile.SearchResult{Source: strings.NewReader(r.set.sources[matches[0]])}, nil
	case 0:
		return protocompile.SearchResult{}, protoregistry.NotFound
	default:
		sort.Strings(matches)
		return protocompile.SearchResult{}, fmt.Errorf(
			"ambiguous import %q matches %d selected files:\n  %s\nselect a narrower set of files",
			importPath, len(matches), strings.Join(matches, "\n  "))
	}
}

// importStmtRe matches import statements (including public/weak modifiers).
var importStmtRe = regexp.MustCompile(`(?m)^\s*import\s+(?:public\s+|weak\s+)?"([^"]+)"\s*;`)

// buildProtoSet reads the selected files and derives each file's import path
// from how the sources actually import it, falling back to its base name.
func buildProtoSet(paths []string) (*protoSet, error) {
	files, err := readSelectedFiles(paths)
	if err != nil {
		return nil, err
	}

	// Canonical name per file: prefer the string other files import it by.
	nameOf := map[string]string{}   // abs -> canonical
	originOf := map[string]string{} // canonical -> abs
	for _, imp := range collectImports(files) {
		if _, taken := originOf[imp]; taken {
			continue
		}
		abs, ok, merr := matchImport(files, imp)
		if merr != nil {
			return nil, protoErrorf("%s", merr.Error())
		}
		if !ok {
			// Not among the selected files: either a well-known type (handled
			// by standard imports) or a genuinely missing file, which the
			// compiler reports with full context.
			continue
		}
		if _, done := nameOf[abs]; done {
			continue
		}
		nameOf[abs] = imp
		originOf[imp] = abs
	}

	// Files nobody imports (typically the entry points) get their base name,
	// extended with parent directories only when needed for uniqueness.
	root := commonDir(files)
	for _, f := range files {
		if _, done := nameOf[f.abs]; done {
			continue
		}
		name := uniqueFallbackName(f.abs, root, files, nameOf, originOf)
		if prev, dup := originOf[name]; dup {
			return nil, protoErrorf(
				"two selected files map to the same import path %q:\n  %s\n  %s",
				name, prev, f.abs)
		}
		nameOf[f.abs] = name
		originOf[name] = f.abs
	}

	set := &protoSet{
		sources: make(map[string]string, len(files)),
		origin:  make(map[string]string, len(files)),
	}
	for _, f := range files {
		name := nameOf[f.abs]
		set.sources[name] = f.data
		set.origin[name] = f.abs
		set.names = append(set.names, name)
	}
	sort.Strings(set.names)
	return set, nil
}

// MaxProtoFiles caps how many .proto files one import may pull in when the
// user selects a directory, so pointing at a huge tree cannot stall the app.
const MaxProtoFiles = 2000

// isMetadataFile reports whether a path is filesystem bookkeeping rather than
// a real source file. macOS writes AppleDouble companions ("._name") and
// ".DS_Store" into directories that have been copied or synced, and those are
// binary blobs whose extension still looks like ".proto".
func isMetadataFile(p string) bool {
	base := path.Base(filepath.ToSlash(p))
	return strings.HasPrefix(base, "._") || base == ".DS_Store" ||
		strings.HasPrefix(base, ".Spotlight-") || base == ".localized"
}

// looksBinary reports whether data is clearly not a text proto file.
func looksBinary(data []byte) bool {
	// NUL bytes never appear in proto source; AppleDouble blobs start with them.
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// readSelectedFiles reads the selected .proto files. A selected directory is
// expanded recursively to every *.proto inside it, which is the natural way to
// import a real project where files import each other by relative paths.
// macOS metadata files ("._foo.proto") are ignored rather than parsed.
func readSelectedFiles(paths []string) ([]fileEntry, error) {
	var candidates []string
	skippedMetadata := 0
	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, protoErrorf("cannot resolve path %s", p).WithDetail(err.Error())
		}
		info, serr := os.Stat(abs)
		if serr != nil {
			return nil, protoErrorf("cannot read proto file: %s", filepath.ToSlash(abs)).WithDetail(serr.Error())
		}
		if !info.IsDir() {
			if isMetadataFile(abs) {
				skippedMetadata++
				continue
			}
			candidates = append(candidates, abs)
			continue
		}
		// Directory: collect *.proto files recursively, skipping hidden and
		// vendored directories as well as filesystem metadata files.
		var found []string
		werr := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entries are skipped, not fatal
			}
			name := d.Name()
			if d.IsDir() {
				if p != abs && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "third_party") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ".proto") {
				return nil
			}
			if isMetadataFile(p) {
				skippedMetadata++
				return nil
			}
			found = append(found, p)
			if len(found) >= MaxProtoFiles {
				return filepath.SkipAll
			}
			return nil
		})
		if werr != nil {
			return nil, protoErrorf("cannot scan directory: %s", filepath.ToSlash(abs)).WithDetail(werr.Error())
		}
		if len(found) == 0 {
			return nil, protoErrorf("no .proto files found in directory: %s", filepath.ToSlash(abs))
		}
		candidates = append(candidates, found...)
	}

	var files []fileEntry
	seen := map[string]bool{}
	for _, abs := range candidates {
		abs = filepath.ToSlash(abs)
		if seen[abs] {
			continue // selecting the same file twice is harmless
		}
		seen[abs] = true
		data, rerr := os.ReadFile(abs)
		if rerr != nil {
			return nil, protoErrorf("cannot read proto file: %s", abs).WithDetail(rerr.Error())
		}
		if looksBinary(data) {
			// Defense in depth: name the offender instead of letting the
			// compiler report a cryptic "invalid control character".
			return nil, protoErrorf(
				"%s is not a text proto file (it looks binary — often a macOS \"._\" metadata file or a build artifact); remove or ignore it",
				abs)
		}
		files = append(files, fileEntry{abs: abs, data: string(data)})
	}
	if len(files) == 0 {
		if skippedMetadata > 0 {
			return nil, protoErrorf("only macOS metadata files were selected (%d ignored) — pick the real .proto files or their folder", skippedMetadata)
		}
		return nil, protoErrorf("no proto files selected")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].abs < files[j].abs })
	return files, nil
}

// collectImports returns every distinct import string used by the sources, in
// a stable order (shortest first, so the most canonical name wins).
func collectImports(files []fileEntry) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		for _, m := range importStmtRe.FindAllStringSubmatch(f.data, -1) {
			imp := strings.TrimSpace(m[1])
			if imp == "" || seen[imp] {
				continue
			}
			seen[imp] = true
			out = append(out, imp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// matchImport finds the selected file an import string refers to: either an
// exact path tail or a base-name equality. Ambiguity is reported.
func matchImport(files []fileEntry, imp string) (string, bool, error) {
	imp = path.Clean(filepath.ToSlash(imp))
	var exact, byBase []string
	for _, f := range files {
		if f.abs == imp || strings.HasSuffix(f.abs, "/"+imp) {
			exact = append(exact, f.abs)
		} else if path.Base(f.abs) == imp {
			byBase = append(byBase, f.abs)
		}
	}
	pick := exact
	if len(pick) == 0 {
		pick = byBase
	}
	switch len(pick) {
	case 0:
		return "", false, nil
	case 1:
		return pick[0], true, nil
	default:
		sort.Strings(pick)
		return "", false, fmt.Errorf(
			"import %q matches %d selected files:\n  %s\nselect only the one you need",
			imp, len(pick), strings.Join(pick, "\n  "))
	}
}

// uniqueFallbackName picks a name for a file nobody imports, extending the
// base name with parent directories until it no longer collides.
func uniqueFallbackName(abs, root string, files []fileEntry, nameOf, originOf map[string]string) string {
	rel := canonicalName(abs, root)
	base := path.Base(abs)

	// Candidates from most specific (relative to root) to least (base name).
	var candidates []string
	if rel != base {
		candidates = append(candidates, rel)
		parts := strings.Split(rel, "/")
		for i := 1; i < len(parts); i++ {
			candidates = append(candidates, strings.Join(parts[i:], "/"))
		}
	}
	candidates = append(candidates, base)

	// A base name is unsafe when another selected file shares it.
	baseCollision := false
	for _, f := range files {
		if f.abs != abs && path.Base(f.abs) == base {
			baseCollision = true
			break
		}
	}

	for _, c := range candidates {
		if _, taken := originOf[c]; taken {
			continue
		}
		if c == base && baseCollision {
			continue
		}
		return c
	}
	return rel
}

// commonDir returns the longest common directory prefix of the given files.
func commonDir(files []fileEntry) string {
	if len(files) == 0 {
		return "/"
	}
	parts := strings.Split(path.Dir(files[0].abs), "/")
	for _, f := range files[1:] {
		dir := strings.Split(path.Dir(f.abs), "/")
		n := 0
		for n < len(parts) && n < len(dir) && parts[n] == dir[n] {
			n++
		}
		parts = parts[:n]
	}
	root := strings.Join(parts, "/")
	if root == "" {
		return "/"
	}
	return root
}

// canonicalName strips root from abs; files outside root fall back to their
// base name.
func canonicalName(abs, root string) string {
	rel := strings.TrimPrefix(abs, strings.TrimSuffix(root, "/")+"/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.HasPrefix(rel, "..") {
		return path.Base(abs)
	}
	return rel
}

// protoErrorf builds a PROTO_ERROR AppError.
func protoErrorf(format string, args ...interface{}) *model.AppError {
	return model.NewError(model.CodeProtoError, format, args...)
}

// compileError turns a protocompile failure into an actionable message while
// preserving the original compiler text (with file:line) in Detail.
func compileError(err error, set *protoSet) *model.AppError {
	msg := err.Error()
	detail := msg
	if len(set.origin) > 0 {
		// Help the user map canonical import paths back to real files.
		var lines []string
		for _, name := range set.names {
			lines = append(lines, fmt.Sprintf("%s  ->  %s", name, set.origin[name]))
		}
		detail = msg + "\n\nselected files:\n  " + strings.Join(lines, "\n  ")
	}

	if errors.Is(err, protoregistry.NotFound) || strings.Contains(msg, "could not resolve path") ||
		strings.Contains(msg, "no such file or directory") {
		return protoErrorf(
			"a proto import could not be resolved. Select the imported file too (or the directory that contains it); google/protobuf/*.proto well-known types are provided automatically.").
			WithDetail(detail)
	}
	if strings.Contains(msg, "ambiguous import") || (strings.Contains(msg, "matches") && strings.Contains(msg, "selected files")) {
		return protoErrorf("a proto import matches more than one selected file — select a narrower set of files.").
			WithDetail(detail)
	}
	if strings.Contains(msg, "already defined") || strings.Contains(msg, "duplicate") {
		return protoErrorf("the selected files define conflicting symbols — check for duplicate message or field names.").
			WithDetail(detail)
	}
	return protoErrorf("failed to compile proto files.").WithDetail(detail)
}

// readAll is a tiny helper kept for symmetry with io.Reader based resolvers.
func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	return string(b), err
}
