package vscodesettings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_Missing(t *testing.T) {
	s, err := Read(filepath.Join(t.TempDir(), ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s != nil {
		t.Errorf("missing file should return nil, got %+v", s)
	}
}

func TestRead_Existing(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	content := `{
		"peacock.color": "#5a3b8c",
		"editor.tabSize": 2,
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c"
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Read(path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s == nil {
		t.Fatal("expected settings")
	}
	if s.Raw["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", s.Raw["peacock.color"])
	}
	if s.Raw["editor.tabSize"].(float64) != 2 {
		t.Errorf("editor.tabSize = %v", s.Raw["editor.tabSize"])
	}
}

func TestPeacockColor(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#abcdef",
	}}
	if got, ok := s.PeacockColor(); !ok || got != "#abcdef" {
		t.Errorf("PeacockColor = %q, %v", got, ok)
	}
}

func TestPeacockColor_Missing(t *testing.T) {
	s := &Settings{Raw: map[string]any{}}
	if _, ok := s.PeacockColor(); ok {
		t.Error("should not be present")
	}
}

func TestResidualColorKeys_None(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#5a3b8c",
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
		},
	}}
	if got := ResidualColorKeys(s); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestResidualColorKeys_HasNonPeacock(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
			"editor.background":      "#000000",
			"terminal.background":    "#111111",
		},
	}}
	got := ResidualColorKeys(s)
	if len(got) != 2 {
		t.Errorf("got %v, want 2 entries", got)
	}
}

func TestResidualColorKeys_NilSettings(t *testing.T) {
	if got := ResidualColorKeys(nil); len(got) != 0 {
		t.Errorf("nil -> %v", got)
	}
}

func TestCleanup_DeletesPeacockKeys(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color":             "#5a3b8c",
		"peacock.affectActivityBar": true,
		"editor.tabSize":            2.0,
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
			"editor.background":      "#000000",
		},
	}}
	Cleanup(s)

	if _, ok := s.Raw["peacock.color"]; ok {
		t.Error("peacock.color should be deleted")
	}
	if _, ok := s.Raw["peacock.affectActivityBar"]; ok {
		t.Error("peacock.affectActivityBar should be deleted")
	}
	if s.Raw["editor.tabSize"].(float64) != 2 {
		t.Error("editor.tabSize should be preserved")
	}
	cc := s.Raw["workbench.colorCustomizations"].(map[string]any)
	if _, ok := cc["activityBar.background"]; ok {
		t.Error("activityBar.background should be deleted")
	}
	if cc["editor.background"] != "#000000" {
		t.Error("editor.background should be preserved")
	}
}

func TestCleanup_RemovesEmptyColorCustomizations(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
		},
	}}
	Cleanup(s)
	if _, ok := s.Raw["workbench.colorCustomizations"]; ok {
		t.Error("empty colorCustomizations should be removed")
	}
}

func TestCleanup_NoSettings(t *testing.T) {
	if Cleanup(nil) {
		t.Error("nil should return false")
	}
}

func TestCleanup_Empty(t *testing.T) {
	s := &Settings{Raw: map[string]any{}}
	if Cleanup(s) {
		t.Error("empty raw should return false (no change)")
	}
}

func TestCleanup_ReportsEmpty(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#5a3b8c",
	}}
	changed := Cleanup(s)
	if !changed {
		t.Error("should report changed")
	}
	if !s.IsEmpty() {
		t.Error("IsEmpty should be true after removing the only key")
	}
}

func TestWriteOrDelete_Delete(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Settings{Path: path, Raw: map[string]any{}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("settings.json should be deleted")
	}
	if _, err := os.Stat(vdir); !os.IsNotExist(err) {
		t.Error(".vscode should be deleted (empty)")
	}
}

func TestWriteOrDelete_WriteNonEmpty(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	s := &Settings{Path: path, Raw: map[string]any{"editor.tabSize": 2.0}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(data) == 0 {
		t.Error("file should not be empty")
	}
}

func TestWriteOrDelete_KeepsNonEmptyVSCodeDir(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, "launch.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Settings{Path: path, Raw: map[string]any{}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("settings.json should be deleted")
	}
	if _, err := os.Stat(vdir); err != nil {
		t.Error(".vscode should NOT be deleted (still has launch.json)")
	}
}
