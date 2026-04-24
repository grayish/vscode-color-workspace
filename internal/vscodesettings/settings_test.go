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
