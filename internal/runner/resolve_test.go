package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/gitworktree"
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

// withFakeWorktrees overrides listWorktreesFn for the duration of a test.
func withFakeWorktrees(t *testing.T, worktrees []gitworktree.Worktree, err error) {
	t.Helper()
	orig := listWorktreesFn
	t.Cleanup(func() { listWorktreesFn = orig })
	listWorktreesFn = func(string) ([]gitworktree.Worktree, error) {
		return worktrees, err
	}
}

// writeWorkspaceWithColor writes a minimal .code-workspace at path with
// peacock.color = hex.
func writeWorkspaceWithColor(t *testing.T, path, hex string) {
	t.Helper()
	body := fmt.Sprintf(`{"settings":{"peacock.color":%q}}`, hex)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveColor_WorktreeCaseA_LinkedTarget(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkedPath, 0755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, _, intent, err := ResolveColor(linkedPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree", src)
	}
	if intent != nil {
		t.Errorf("intent = %v, want nil (Case A has no side effect)", intent)
	}
	// Linked color should be a lightness shift of #5a3b8c — not equal to it.
	want := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if c == want {
		t.Errorf("linked color = main color (%v); expected lightness offset", c)
	}
}

func TestResolveColor_WorktreeCaseA_MainTarget(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
	}, nil)

	c, src, _, _, err := ResolveColor(mainPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree", src)
	}
	want := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if c != want {
		t.Errorf("main color = %v, want %v (offset 0)", c, want)
	}
}

func TestResolveColor_WorktreeCaseD_TargetMain(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkedPath, 0755); err != nil {
		t.Fatal(err)
	}
	// linked has a color; main does not
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#4a8b5c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	_, src, warns, intent, err := ResolveColor(mainPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom (Case D falls back)", src)
	}
	if intent != nil {
		t.Errorf("intent = %v, want nil (Case D writes nothing)", intent)
	}
	if len(warns) == 0 {
		t.Fatal("warns empty; want family-disabled warning")
	}
	if !strings.Contains(warns[0], "family disabled") {
		t.Errorf("warning text = %q, want substring %q", warns[0], "family disabled")
	}
	if !strings.Contains(warns[0], "#4a8b5c") {
		t.Errorf("warning text = %q, want linked hex", warns[0])
	}
}

func TestResolveColor_WorktreeCaseD_TargetOtherLinked(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedAPath := filepath.Join(base, "myproj-feat-x")
	linkedBPath := filepath.Join(base, "myproj-bugfix")
	for _, p := range []string{mainPath, linkedAPath, linkedBPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#4a8b5c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedAPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: linkedBPath, GitDir: filepath.Join(mainPath, ".git/worktrees/bugfix"), IsMain: false},
	}, nil)

	_, src, warns, intent, err := ResolveColor(linkedBPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom", src)
	}
	if intent != nil {
		t.Errorf("intent = %v, want nil", intent)
	}
	if len(warns) == 0 || !strings.Contains(warns[0], "family disabled") {
		t.Errorf("warns = %v, want a family-disabled notice", warns)
	}
}
