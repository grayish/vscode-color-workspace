package vscodesettings

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
)

// WriteOrDelete writes s.Raw to s.Path, or deletes the file (and any
// now-empty parent .vscode/ directory) if s.Raw is empty.
func WriteOrDelete(s *Settings) error {
	if s == nil || s.Path == "" {
		return nil
	}
	if s.IsEmpty() {
		if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete %s: %w", s.Path, err)
		}
		parent := filepath.Dir(s.Path)
		if filepath.Base(parent) == ".vscode" {
			entries, err := os.ReadDir(parent)
			if err == nil && len(entries) == 0 {
				if err := os.Remove(parent); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("delete %s: %w", parent, err)
				}
			}
		}
		return nil
	}
	data, err := jsonc.Write(s.Raw)
	if err != nil {
		return err
	}
	return atomicWrite(s.Path, data, 0o644)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccws-*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	tmpPath := tmp.Name()
	cleaned := false
	defer func() {
		if !cleaned {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleaned = true
	return nil
}
