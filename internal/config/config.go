// Package config persists connection profiles under the per-user config
// directory (~/Library/Application Support/etcura on macOS) and keeps
// passwords out of that file by storing them in the OS keyring via
// go-keyring (macOS Keychain, Windows Credential Manager, Secret Service).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"

	"etcura/internal/model"
)

const (
	appDirName     = "etcura"
	fileName       = "connections.json"
	settingsFile   = "settings.json"
	keyringUser    = "etcura"
	keyringService = "etcura"
)

// keyringBackend abstracts the Keychain so tests can inject a fake.
var keyringBackend keyringBackendT = keyringBackendImpl{}

// userConfigDir is swapped for a temp dir in unit tests.
var userConfigDir = os.UserConfigDir

type keyringBackendT interface {
	Set(user, password string) error
	Get(user string) (string, error)
	Delete(user string) error
}

type keyringBackendImpl struct{}

func (keyringBackendImpl) Set(user, password string) error {
	return keyring.Set(keyringService, user, password)
}
func (keyringBackendImpl) Get(user string) (string, error) { return keyring.Get(keyringService, user) }
func (keyringBackendImpl) Delete(user string) error        { return keyring.Delete(keyringService, user) }

// Dir returns the per-user configuration directory, creating it if needed.
func Dir() (string, error) {
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func filePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Load reads all saved connection profiles. A missing file means "no saved
// connections", not an error.
func Load() ([]model.ConnectionConfig, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, model.NewError(model.CodeConfigError, "cannot read connections file").WithDetail(err.Error())
	}
	var conns []model.ConnectionConfig
	if err := json.Unmarshal(data, &conns); err != nil {
		return nil, model.NewError(model.CodeConfigError, "connections file is corrupted, starting with an empty list").
			WithDetail(err.Error())
	}
	return conns, nil
}

// Save writes the profiles atomically (tmp file + rename) with 0600 so other
// users on the machine cannot read the (password-free) file either.
func Save(conns []model.ConnectionConfig) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return model.NewError(model.CodeConfigError, "cannot write connections file").WithDetail(err.Error())
	}
	if err := os.Rename(tmp, path); err != nil {
		return model.NewError(model.CodeConfigError, "cannot replace connections file").WithDetail(err.Error())
	}
	return nil
}

// SetPassword stores the password in the Keychain, keyed by connection ID.
func SetPassword(connectionID, password string) error {
	if connectionID == "" {
		return model.NewInvalidArgument("connection id must not be empty")
	}
	if err := keyringBackend.Set(connectionID, password); err != nil {
		return model.NewError(model.CodeConfigError, "could not store password in the system keyring").
			WithDetail(err.Error())
	}
	return nil
}

// GetPassword reads the stored password. A missing entry yields "" without
// error so callers can treat it as "no password configured".
func GetPassword(connectionID string) string {
	if connectionID == "" {
		return ""
	}
	pw, err := keyringBackend.Get(connectionID)
	if err != nil || pw == "" {
		return ""
	}
	return pw
}

// HasPassword reports whether a password is stored for the connection.
func HasPassword(connectionID string) bool {
	if connectionID == "" {
		return false
	}
	_, err := keyringBackend.Get(connectionID)
	return err == nil
}

// DeletePassword removes the stored password (best effort).
func DeletePassword(connectionID string) error {
	if connectionID == "" {
		return nil
	}
	if err := keyringBackend.Delete(connectionID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keychain delete: %w", err)
	}
	return nil
}

// Settings holds app-level preferences that are not tied to one connection.
type Settings struct {
	// Script is the post-processing snippet applied to values. Nil means no
	// script is configured.
	Script *model.ScriptConfig `json:"script,omitempty"`
}

func settingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsFile), nil
}

// LoadSettings reads app settings; a missing file yields zero values.
func LoadSettings() (Settings, error) {
	var st Settings
	path, err := settingsPath()
	if err != nil {
		return st, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, model.NewError(model.CodeConfigError, "cannot read settings file").WithDetail(err.Error())
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, model.NewError(model.CodeConfigError, "settings file is corrupted, using defaults").
			WithDetail(err.Error())
	}
	return st, nil
}

// SaveSettings writes app settings atomically.
func SaveSettings(st Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return model.NewError(model.CodeConfigError, "cannot write settings file").WithDetail(err.Error())
	}
	if err := os.Rename(tmp, path); err != nil {
		return model.NewError(model.CodeConfigError, "cannot replace settings file").WithDetail(err.Error())
	}
	return nil
}
