package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPreconfigured_PeacockKeysPresent(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#abcdef"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := detectPreconfigured(target)
	if err != nil {
		t.Fatalf("detectPreconfigured: %v", err)
	}
	if gotPath != wsPath {
		t.Errorf("path = %q, want %q", gotPath, wsPath)
	}
	if len(gotKeys) == 0 {
		t.Error("keys should be non-empty")
	}
}

func TestDetectPreconfigured_NoPeacockKeys(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"editor.tabSize":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := detectPreconfigured(target)
	if err != nil {
		t.Fatalf("detectPreconfigured: %v", err)
	}
	if gotPath != "" {
		t.Errorf("path = %q, want empty (no peacock keys)", gotPath)
	}
	if len(gotKeys) != 0 {
		t.Errorf("keys = %v, want empty", gotKeys)
	}
}

func TestDetectPreconfigured_NoFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := detectPreconfigured(target)
	if err != nil {
		t.Fatalf("detectPreconfigured: %v", err)
	}
	if gotPath != "" || len(gotKeys) != 0 {
		t.Errorf("expected empty path/keys when ws file does not exist, got (%q, %v)", gotPath, gotKeys)
	}
}
