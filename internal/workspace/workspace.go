// Package workspace handles reading, merging, and writing VSCode
// .code-workspace files (JSONC format).
package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// Folder is an entry in the top-level "folders" array.
type Folder struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// Workspace mirrors the VSCode .code-workspace JSON schema. We keep the
// settings block as a free-form map so we don't lose unknown keys during
// round-trip.
type Workspace struct {
	Folders    []Folder       `json:"folders"`
	Settings   map[string]any `json:"settings,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
	Launch     map[string]any `json:"launch,omitempty"`
	Tasks      map[string]any `json:"tasks,omitempty"`
	Other      map[string]any `json:"-"`
}

// PeacockColor returns the peacock.color setting if present and stored as a
// string. Returns ("", false) when the workspace is nil, has no settings, or
// the key is missing/wrong type.
func (ws *Workspace) PeacockColor() (string, bool) {
	if ws == nil || ws.Settings == nil {
		return "", false
	}
	s, ok := ws.Settings[peacock.SettingColor].(string)
	if !ok {
		return "", false
	}
	return s, true
}

// Read parses the file at path. Returns (nil, nil) if the file does not exist.
func Read(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("workspace read: %w", err)
	}
	var raw map[string]any
	if err := jsonc.Read(data, &raw); err != nil {
		return nil, fmt.Errorf("workspace read %s: %w", path, err)
	}
	return fromMap(raw), nil
}

// Write serializes ws to path atomically (temp file + rename).
func Write(path string, ws *Workspace) error {
	data, err := jsonc.Write(toMap(ws))
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0o644)
}

func fromMap(m map[string]any) *Workspace {
	ws := &Workspace{Other: map[string]any{}}
	for k, v := range m {
		switch k {
		case "folders":
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if o, ok := e.(map[string]any); ok {
						f := Folder{}
						if p, ok := o["path"].(string); ok {
							f.Path = p
						}
						if n, ok := o["name"].(string); ok {
							f.Name = n
						}
						ws.Folders = append(ws.Folders, f)
					}
				}
			}
		case "settings":
			if o, ok := v.(map[string]any); ok {
				ws.Settings = o
			}
		case "extensions":
			if o, ok := v.(map[string]any); ok {
				ws.Extensions = o
			}
		case "launch":
			if o, ok := v.(map[string]any); ok {
				ws.Launch = o
			}
		case "tasks":
			if o, ok := v.(map[string]any); ok {
				ws.Tasks = o
			}
		default:
			ws.Other[k] = v
		}
	}
	return ws
}

func toMap(ws *Workspace) map[string]any {
	out := map[string]any{}
	for k, v := range ws.Other {
		out[k] = v
	}
	folders := make([]map[string]any, 0, len(ws.Folders))
	for _, f := range ws.Folders {
		m := map[string]any{"path": f.Path}
		if f.Name != "" {
			m["name"] = f.Name
		}
		folders = append(folders, m)
	}
	out["folders"] = folders
	if len(ws.Settings) > 0 {
		out["settings"] = ws.Settings
	}
	if len(ws.Extensions) > 0 {
		out["extensions"] = ws.Extensions
	}
	if len(ws.Launch) > 0 {
		out["launch"] = ws.Launch
	}
	if len(ws.Tasks) > 0 {
		out["tasks"] = ws.Tasks
	}
	return out
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccws-*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
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
		return fmt.Errorf("atomic write: copy: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomic write: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomic write: close: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("atomic write: chmod: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	cleaned = true
	return nil
}
