package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveColor_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"peacock.color": "#111111"}`)
	got, src, err := ResolveColor(dir, "#222222")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hex() != "#222222" {
		t.Errorf("got %s, want #222222", got.Hex())
	}
	if src != SourceFlag {
		t.Errorf("source = %v, want SourceFlag", src)
	}
}

func TestResolveColor_InheritFromSettings(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"peacock.color": "#5a3b8c"}`)
	got, src, err := ResolveColor(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hex() != "#5a3b8c" {
		t.Errorf("got %s, want #5a3b8c", got.Hex())
	}
	if src != SourceSettings {
		t.Errorf("source = %v, want SourceSettings", src)
	}
}

func TestResolveColor_Random(t *testing.T) {
	dir := t.TempDir()
	got, src, err := ResolveColor(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom", src)
	}
	if !strings.HasPrefix(got.Hex(), "#") {
		t.Errorf("expected hex format, got %s", got.Hex())
	}
}

func TestResolveColor_InvalidFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ResolveColor(dir, "not-a-color"); err == nil {
		t.Error("expected error for bad input")
	}
}

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
