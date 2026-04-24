package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_Missing(t *testing.T) {
	ws, err := Read(filepath.Join(t.TempDir(), "nope.code-workspace"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ws != nil {
		t.Errorf("expected nil for missing file, got %+v", ws)
	}
}

func TestRead_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.code-workspace")
	content := `{
		// settings below
		"folders": [{ "path": "./foo" }],
		"settings": {
			"peacock.color": "#5a3b8c",
			"editor.fontSize": 14
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := Read(path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace")
	}
	if len(ws.Folders) != 1 || ws.Folders[0].Path != "./foo" {
		t.Errorf("folders = %+v", ws.Folders)
	}
	if ws.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", ws.Settings["peacock.color"])
	}
}

func TestWrite_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bar.code-workspace")
	ws := &Workspace{
		Folders: []Folder{{Path: "./bar"}},
		Settings: map[string]any{
			"peacock.color": "#5a3b8c",
			"workbench.colorCustomizations": map[string]any{
				"activityBar.background": "#5a3b8c",
			},
		},
	}
	if err := Write(path, ws); err != nil {
		t.Fatalf("err = %v", err)
	}
	ws2, err := Read(path)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if ws2.Folders[0].Path != "./bar" {
		t.Errorf("folders mismatch after roundtrip")
	}
	if ws2.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color missing")
	}
}
