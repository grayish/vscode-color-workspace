// Package vscodesettings reads and modifies .vscode/settings.json, with
// Peacock-specific helpers for detection and cleanup.
package vscodesettings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// Settings is a loaded .vscode/settings.json as a raw map.
type Settings struct {
	Path string
	Raw  map[string]any
}

// Read loads the settings file at path. Returns (nil, nil) for missing file.
func Read(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("settings read: %w", err)
	}
	var raw map[string]any
	if err := jsonc.Read(data, &raw); err != nil {
		return nil, fmt.Errorf("settings read %s: %w", path, err)
	}
	return &Settings{Path: path, Raw: raw}, nil
}

// PeacockColor returns the peacock.color setting if present.
func (s *Settings) PeacockColor() (string, bool) {
	v, ok := s.Raw[peacock.SettingColor].(string)
	return v, ok && v != ""
}
