package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// saveConfig writes to a private temporary file then atomically replaces the
// configuration. A failed save never overwrites a working configuration.
func saveConfig(cfg Config) error {
	stored, err := encryptConfigSecrets(cfg)
	if err != nil {
		return fmt.Errorf("加密 API 凭据: %w", err)
	}
	b, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
