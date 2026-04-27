package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
)

func TestResolveColor_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"peacock.color": "#111111"}`)
	got, src, _, _, err := ResolveColor(dir, "#222222")
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
	got, src, _, _, err := ResolveColor(dir, "")
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
	got, src, _, _, err := ResolveColor(dir, "")
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
	if _, _, _, _, err := ResolveColor(dir, "not-a-color"); err == nil {
		t.Error("expected error for bad input")
	}
}

func TestReadWorkspacePeacockColor_Present(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "myproj.code-workspace")
	body := `{"settings": {"peacock.color": "#5a3b8c"}}`
	if err := os.WriteFile(wsPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readWorkspacePeacockColor(wsPath)
	if err != nil {
		t.Fatalf("readWorkspacePeacockColor: %v", err)
	}
	if got == nil {
		t.Fatal("got nil, want color")
	}
	want := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if *got != want {
		t.Errorf("got %v, want %v", *got, want)
	}
}

func TestReadWorkspacePeacockColor_Missing(t *testing.T) {
	got, err := readWorkspacePeacockColor("/nonexistent/path.code-workspace")
	if err != nil {
		t.Fatalf("readWorkspacePeacockColor(missing): %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestReadWorkspacePeacockColor_NoColor(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "myproj.code-workspace")
	body := `{"settings": {"editor.fontSize": 14}}`
	if err := os.WriteFile(wsPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readWorkspacePeacockColor(wsPath)
	if err != nil {
		t.Fatalf("readWorkspacePeacockColor: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
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
