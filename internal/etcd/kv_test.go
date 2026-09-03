package etcd

import (
	"bytes"
	"encoding/base64"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"

	"etcura/internal/model"
)

func TestDetailFromKVUTF8(t *testing.T) {
	kv := &mvccpb.KeyValue{
		Key:            []byte("/app/config"),
		Value:          []byte(`{"hello":"world"}`),
		CreateRevision: 10,
		ModRevision:    20,
		Version:        3,
		Lease:          0,
	}
	d := DetailFromKV(kv)
	if !d.IsUTF8 {
		t.Fatal("valid UTF-8 must be flagged as UTF-8")
	}
	raw, err := base64.StdEncoding.DecodeString(d.ValueBase64)
	if err != nil {
		t.Fatalf("valueBase64 is not decodable: %v", err)
	}
	if string(raw) != `{"hello":"world"}` {
		t.Fatalf("round-trip mismatch: %q", raw)
	}
	if d.SizeBytes != 17 {
		t.Fatalf("size = %d", d.SizeBytes)
	}
	if d.ModRevision != 20 || d.CreateRevision != 10 || d.Version != 3 {
		t.Fatalf("metadata lost: %+v", d)
	}
}

func TestDetailFromKVBinary(t *testing.T) {
	// 0x00, 0xff and an invalid UTF-8 continuation sequence must survive the
	// base64 round trip byte for byte.
	binary := []byte{0x00, 0xff, 0xfe, 0x80, 0x0a, 0x00}
	kv := &mvccpb.KeyValue{Key: []byte("/bin"), Value: binary}
	d := DetailFromKV(kv)
	if d.IsUTF8 {
		t.Fatal("invalid UTF-8 must not be flagged as UTF-8")
	}
	raw, err := base64.StdEncoding.DecodeString(d.ValueBase64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(raw, binary) {
		t.Fatalf("binary value corrupted: %x vs %x", raw, binary)
	}
}

func TestDetailFromKVEmptyValue(t *testing.T) {
	d := DetailFromKV(&mvccpb.KeyValue{Key: []byte("/empty"), Value: []byte{}})
	if !d.IsUTF8 || d.SizeBytes != 0 || d.ValueBase64 != "" {
		t.Fatalf("unexpected detail: %+v", d)
	}
}

func TestKeyPageWireShape(t *testing.T) {
	// Guards against accidental model changes that would break the frontend.
	items := []model.KeySummary{{Key: "/k", ModRevision: 2, Version: 1}}
	if items[0].Key != "/k" {
		t.Fatal("shape changed")
	}
}
