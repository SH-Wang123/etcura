package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"etcd-eye/internal/model"
)

// fakeKeyring records calls so tests never touch the real Keychain.
type fakeKeyring struct {
	items map[string]string
}

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{items: map[string]string{}} }

func (f *fakeKeyring) Set(user, password string) error {
	f.items[user] = password
	return nil
}
func (f *fakeKeyring) Get(user string) (string, error) {
	pw, ok := f.items[user]
	if !ok {
		return "", errors.New("secret not found in keyring")
	}
	return pw, nil
}
func (f *fakeKeyring) Delete(user string) error {
	delete(f.items, user)
	return nil
}

func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	userConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userConfigDir = os.UserConfigDir })
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempConfig(t)

	conns := []model.ConnectionConfig{
		{ID: "a1", Name: "dev", Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:22379"}, DialTimeoutMs: 5000, RequestTimeoutMs: 10000},
		{ID: "b2", Name: "prod", Endpoints: []string{"https://etcd.prod:2379"}, Username: "admin",
			DialTimeoutMs: 5000, RequestTimeoutMs: 10000,
			TLS: model.TLSConfig{Enabled: true, CAFile: "/ca.pem", CertFile: "/c.pem", KeyFile: "/k.pem", SkipVerify: false}},
	}
	if err := Save(conns); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d connections", len(loaded))
	}
	if loaded[1].TLS.CAFile != "/ca.pem" || loaded[1].TLS.Enabled != true {
		t.Fatalf("TLS config lost: %+v", loaded[1].TLS)
	}
	if loaded[0].Endpoints[1] != "127.0.0.1:22379" {
		t.Fatalf("multi-endpoint lost: %+v", loaded[0])
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	withTempConfig(t)
	conns, err := Load()
	if err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected empty list, got %d", len(conns))
	}
}

func TestSaveFilePermissions(t *testing.T) {
	withTempConfig(t)
	if err := Save(nil); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := filePath()
	if err != nil {
		t.Fatalf("filePath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("connections.json permission = %o, want 600", perm)
	}
}

func TestPasswordsNeverInConfigFile(t *testing.T) {
	withTempConfig(t)
	fake := newFakeKeyring()
	keyringBackend = fake
	t.Cleanup(func() { keyringBackend = keyringBackendImpl{} })

	if err := SetPassword("conn-1", "s3cret"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if GetPassword("conn-1") != "s3cret" {
		t.Fatal("password round trip failed")
	}
	if !HasPassword("conn-1") {
		t.Fatal("HasPassword should be true")
	}

	path, err := filePath()
	if err != nil {
		t.Fatalf("filePath: %v", err)
	}
	_ = path
	// Save a profile and verify the file content contains no password trace.
	if err := Save([]model.ConnectionConfig{{ID: "conn-1", Name: "x", Endpoints: []string{"127.0.0.1:2379"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	dir, _ := Dir()
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(string(data), "s3cret") {
		t.Fatal("config file must never contain the password")
	}

	if err := DeletePassword("conn-1"); err != nil {
		t.Fatalf("DeletePassword: %v", err)
	}
	if GetPassword("conn-1") != "" {
		t.Fatal("password should be gone")
	}
}

func TestGetPasswordMissingIsOK(t *testing.T) {
	withTempConfig(t)
	fake := newFakeKeyring()
	keyringBackend = fake
	t.Cleanup(func() { keyringBackend = keyringBackendImpl{} })

	if pw := GetPassword("does-not-exist"); pw != "" {
		t.Fatalf("missing password must be empty, got %q", pw)
	}
	if HasPassword("") {
		t.Fatal("empty id must report false")
	}
}
