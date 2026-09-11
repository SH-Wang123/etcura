// Package protodec imports .proto files (compiled in-process by
// bufbuild/protocompile — no protoc needed) and heuristically matches raw
// etcd values against the imported message types using dynamic messages.
//
// Honest limitation: the protobuf wire format is not self-describing, so a
// value may decode successfully into several structurally similar messages.
// The matcher ranks candidates (fields decoded, wire-tag coverage, unknown
// bytes) and the UI auto-selects the best while letting the user switch.
package protodec

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"sync"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"etcura/internal/model"
)

const (
	// maxCandidatesTried bounds dynamic unmarshal attempts per match run so a
	// huge type space cannot stall the UI thread pool.
	maxCandidatesTried = 5000
	// maxTopCandidates is how many decoded candidates are returned to the UI.
	maxTopCandidates = 6
	// maxJSONBytes caps the protojson payload per candidate.
	maxJSONBytes = 1 << 20 // 1 MB
	// MaxMatchValueBytes caps the input value size for matching.
	MaxMatchValueBytes = 8 << 20 // 8 MB
)

// protoMarshaler renders decoded messages as JSON with original proto field
// names (e.g. "order_id" instead of "orderId"), which is what users expect
// when inspecting raw data.
var protoMarshaler = protojson.MarshalOptions{UseProtoNames: true}

// Importer holds the compiled descriptor set of all imported proto files.
// It is safe for concurrent use; Import replaces the whole set.
type Importer struct {
	mu       sync.RWMutex
	files    []model.ProtoFileInfo
	messages []protoreflect.MessageDescriptor
	enums    int
}

func NewImporter() *Importer { return &Importer{} }

// Files returns a snapshot of imported file infos.
func (im *Importer) Files() []model.ProtoFileInfo {
	im.mu.RLock()
	defer im.mu.RUnlock()
	out := make([]model.ProtoFileInfo, len(im.files))
	copy(out, im.files)
	return out
}

// MessageCount is the number of message types available for matching.
func (im *Importer) MessageCount() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return len(im.messages)
}

// Import compiles the given proto files and replaces the current descriptor
// set. Relative layout is preserved (canonical paths derive from the files'
// longest common directory), so cross-file imports such as
// `import "api/v1/common.proto"` resolve exactly as with `protoc -I <root>`.
// Well-known types (google/protobuf/*.proto) resolve even when not selected.
func (im *Importer) Import(ctx context.Context, paths []string) ([]model.ProtoFileInfo, error) {
	set, err := buildProtoSet(paths)
	if err != nil {
		return nil, err
	}

	compiler := protocompile.Compiler{
		// WithStandardImports serves the well-known types bundled with protoc
		// (timestamp.proto, wrappers.proto, descriptor.proto, …).
		Resolver: protocompile.WithStandardImports(&setResolver{set: set}),
	}
	results, err := compiler.Compile(ctx, set.names...)
	if err != nil {
		return nil, compileError(err, set)
	}

	var files []model.ProtoFileInfo
	var messages []protoreflect.MessageDescriptor
	enumCount := 0

	for _, res := range results {
		// linker.File embeds protoreflect.FileDescriptor.
		fd := protoreflect.FileDescriptor(res)
		info := model.ProtoFileInfo{
			// Keep the full import path so users can tell two files with the
			// same base name apart.
			Name:    fd.Path(),
			Package: string(fd.Package()),
		}
		mds := fd.Messages()
		for i := 0; i < mds.Len(); i++ {
			collectMessages(mds.Get(i), &messages, &info.MessageCount)
		}
		eds := fd.Enums()
		enumCount += eds.Len()
		info.EnumCount = eds.Len()
		files = append(files, info)
	}

	im.mu.Lock()
	im.files = files
	im.messages = messages
	im.enums = enumCount
	im.mu.Unlock()

	return files, nil
}

// Clear removes all imported protos.
func (im *Importer) Clear() {
	im.mu.Lock()
	im.files = nil
	im.messages = nil
	im.enums = 0
	im.mu.Unlock()
}

func collectMessages(md protoreflect.MessageDescriptor, out *[]protoreflect.MessageDescriptor, count *int) {
	if md.IsMapEntry() {
		return
	}
	*out = append(*out, md)
	*count++
	nested := md.Messages()
	for i := 0; i < nested.Len(); i++ {
		collectMessages(nested.Get(i), out, count)
	}
}

// Match heuristically matches value against every imported message type and
// returns ranked candidates. Non-proto-shaped input yields an empty result.
func (im *Importer) Match(value []byte) model.ProtoMatchResult {
	result := model.ProtoMatchResult{}
	if len(value) == 0 || len(value) > MaxMatchValueBytes {
		result.ValueTooLarge = len(value) > MaxMatchValueBytes
		return result
	}

	tags := wireTagNumbers(value)
	if tags == nil {
		// Not proto-shaped at the top level (e.g. plain JSON/text).
		return result
	}
	for t := range tags {
		result.WireTagNums = append(result.WireTagNums, int(t))
	}
	sort.Ints(result.WireTagNums)

	im.mu.RLock()
	messages := im.messages
	im.mu.RUnlock()
	if len(messages) == 0 {
		return result
	}

	// Fast prefilter: a candidate must own at least one field number that
	// appears in the blob's top-level tags.
	tagSet := tags
	tried := 0
	type scored struct {
		name  string
		score int
		cand  model.ProtoCandidate
	}
	var matches []scored

	for _, md := range messages {
		if tried >= maxCandidatesTried {
			break
		}
		if !mdFieldsIntersect(md, tagSet) {
			continue
		}
		tried++

		msg := dynamicpb.NewMessage(md)
		if err := proto.Unmarshal(value, msg); err != nil {
			continue
		}

		cand := model.ProtoCandidate{
			MessageName: string(md.FullName()),
			UnknownBytes: len(msg.GetUnknown()),
		}
		score := 0
		covered := 0
		msg.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			cand.SetFieldNums = append(cand.SetFieldNums, int(fd.Number()))
			if tagSet[int32(fd.Number())] {
				covered++
			}
			score += 10
			return true
		})
		sort.Ints(cand.SetFieldNums)
		if cand.UnknownBytes > 0 {
			penalty := 8 + cand.UnknownBytes/64
			if penalty > 20 {
				penalty = 20
			}
			score -= penalty
		} else {
			score += 3
		}
		score += covered * 5

		// JSON only for reasonable sizes; large packed payloads stay viewable
		// via the field list without blowing up the bridge.
		if j, jerr := protoMarshaler.Marshal(msg); jerr == nil && len(j) <= maxJSONBytes {
			cand.JSON = string(j)
		}

		matches = append(matches, scored{name: cand.MessageName, score: score, cand: cand})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].name < matches[j].name
	})
	result.Scanned = tried
	for i, m := range matches {
		if i >= maxTopCandidates {
			break
		}
		m.cand.Score = m.score
		result.Candidates = append(result.Candidates, m.cand)
	}
	return result
}

// mdFieldsIntersect reports whether any direct field number of md is in tags.
func mdFieldsIntersect(md protoreflect.MessageDescriptor, tags map[int32]bool) bool {
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		if tags[int32(fields.Get(i).Number())] {
			return true
		}
	}
	return false
}

// wireTagNumbers walks the top-level protobuf wire structure and returns the
// set of field numbers. Returns nil when the bytes are not proto-shaped
// (malformed varints, truncated payloads, unsupported group wire types).
func wireTagNumbers(b []byte) map[int32]bool {
	tags := map[int32]bool{}
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 {
			return nil
		}
		fieldNum := int32(tag >> 3)
		wireType := int(tag & 0x7)
		if fieldNum == 0 {
			return nil
		}
		b = b[n:]
		switch wireType {
		case 0: // varint
			_, n = binary.Uvarint(b)
			if n <= 0 {
				return nil
			}
			b = b[n:]
		case 1: // fixed64
			if len(b) < 8 {
				return nil
			}
			b = b[8:]
		case 2: // length-delimited
			l, n := binary.Uvarint(b)
			if n <= 0 || uint64(len(b)-n) < l {
				return nil
			}
			b = b[n+int(l):]
		case 5: // fixed32
			if len(b) < 4 {
				return nil
			}
			b = b[4:]
		default: // groups (3,4) and reserved: treat as not-proto
			return nil
		}
		tags[fieldNum] = true
	}
	return tags
}

// ErrNotCompiled is returned by helpers that need a descriptor set when none
// was imported. Kept for API completeness; Match returns an empty result.
var ErrNotCompiled = errors.New("no proto files imported")
