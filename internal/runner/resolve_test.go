package runner

import (
	"errors"
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
	got, src, _, _, _, err := ResolveColor(dir, "#222222", false, false)
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
	got, src, _, _, _, err := ResolveColor(dir, "", false, false)
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
	got, src, _, _, _, err := ResolveColor(dir, "", false, false)
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
	if _, _, _, _, _, err := ResolveColor(dir, "not-a-color", false, false); err == nil {
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

	c, src, _, intent, _, err := ResolveColor(linkedPath, "", false, false)
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

func TestResolveColor_A1_SingleWorktreeMain_FallsThrough(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
	}, nil)

	c, src, _, _, _, err := ResolveColor(mainPath, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	// Single-worktree main with --force should NOT enter Case A. With no
	// settings.json peacock.color present, expect SourceRandom.
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom (A1 fall-through)", src)
	}
	// The random color must not match the existing main color (overwhelmingly
	// unlikely; if it does, the test re-runs will catch any structural error).
	original := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if c == original {
		t.Errorf("got same color as existing main color (%v); A1 should regenerate", c)
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

	_, src, warns, intent, _, err := ResolveColor(mainPath, "", false, false)
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

	_, src, warns, intent, _, err := ResolveColor(linkedBPath, "", false, false)
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

func TestResolveColor_WorktreeCaseB_MainTargetNoColor(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
	}, nil)

	_, src, warns, intent, _, err := ResolveColor(mainPath, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom", src)
	}
	if intent != nil {
		t.Errorf("intent = %v, want nil", intent)
	}
	if len(warns) != 0 {
		t.Errorf("warns = %v, want empty (Case B is silent)", warns)
	}
}

func TestBuildPropagateTargets_SkipsMainAndUncolored(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	bug := filepath.Join(base, "myproj-bugfix")
	hot := filepath.Join(base, "myproj-hotfix")
	for _, p := range []string{mainPath, feat, bug, hot} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// feat: in family (peacock.color present)
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#7a5bac")
	// bug: in family (peacock.color present)
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-bugfix.code-workspace"), "#4a2b6c")
	// hot: .code-workspace exists but no peacock keys — should be skipped
	if err := os.WriteFile(filepath.Join(base, "myproj-hotfix.code-workspace"),
		[]byte(`{"folders":[{"path":"./myproj-hotfix"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	worktrees := []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: bug, GitDir: filepath.Join(mainPath, ".git/worktrees/bugfix"), IsMain: false},
		{Path: hot, GitDir: filepath.Join(mainPath, ".git/worktrees/hotfix"), IsMain: false},
	}
	anchor := color.Color{R: 0xaa, G: 0xbb, B: 0xcc}

	targets, skipped := buildPropagateTargets(worktrees, anchor)

	if len(targets) != 2 {
		t.Fatalf("targets count = %d, want 2 (feat, bug); got %v", len(targets), targets)
	}
	gotPaths := map[string]bool{}
	for _, tgt := range targets {
		gotPaths[tgt.WorkspacePath] = true
		if tgt.DerivedColor == anchor {
			t.Errorf("derived color = anchor for %s; expected non-zero offset", tgt.WorkspacePath)
		}
	}
	if !gotPaths[filepath.Join(base, "myproj-feat-x.code-workspace")] {
		t.Errorf("missing feat-x in targets")
	}
	if !gotPaths[filepath.Join(base, "myproj-bugfix.code-workspace")] {
		t.Errorf("missing bugfix in targets")
	}

	if len(skipped) != 1 {
		t.Fatalf("skipped count = %d, want 1 (hotfix); got %v", len(skipped), skipped)
	}
	if skipped[0].WorkspacePath != filepath.Join(base, "myproj-hotfix.code-workspace") {
		t.Errorf("skipped path = %s, want hotfix", skipped[0].WorkspacePath)
	}
	if skipped[0].Reason == "" {
		t.Errorf("skipped reason is empty")
	}
}

func TestBuildPropagateTargets_SkipsMissingWorkspaceFile(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	for _, p := range []string{mainPath, feat} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// feat has NO .code-workspace file at all

	worktrees := []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}
	anchor := color.Color{R: 0xaa, G: 0xbb, B: 0xcc}

	targets, skipped := buildPropagateTargets(worktrees, anchor)

	if len(targets) != 0 {
		t.Errorf("targets = %v, want empty", targets)
	}
	if len(skipped) != 1 {
		t.Fatalf("skipped count = %d, want 1", len(skipped))
	}
	if !strings.Contains(skipped[0].Reason, "no .code-workspace") {
		t.Errorf("skipped reason = %q, want substring 'no .code-workspace'", skipped[0].Reason)
	}
}

func TestBuildPropagateTargets_SkipsOnParseError(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	for _, p := range []string{mainPath, feat} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Malformed JSON → workspace.Read returns parse error
	if err := os.WriteFile(filepath.Join(base, "myproj-feat-x.code-workspace"),
		[]byte(`{"settings": {`), 0o644); err != nil {
		t.Fatal(err)
	}
	worktrees := []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}
	anchor := color.Color{R: 0xaa, G: 0xbb, B: 0xcc}

	targets, skipped := buildPropagateTargets(worktrees, anchor)

	if len(targets) != 0 {
		t.Errorf("targets = %v, want empty (parse error → no targets)", targets)
	}
	if len(skipped) != 1 {
		t.Fatalf("skipped count = %d, want 1", len(skipped))
	}
	if !strings.Contains(skipped[0].Reason, "parse error") {
		t.Errorf("skipped reason = %q, want substring 'parse error'", skipped[0].Reason)
	}
	wantPath := filepath.Join(base, "myproj-feat-x.code-workspace")
	if skipped[0].WorkspacePath != wantPath {
		t.Errorf("skipped path = %q, want %q", skipped[0].WorkspacePath, wantPath)
	}
}

func TestResolveColor_WorktreeCaseC_LinkedFirst_ReturnsIntent(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkedPath, 0755); err != nil {
		t.Fatal(err)
	}

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, warns, intent, _, err := ResolveColor(linkedPath, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree", src)
	}
	if intent == nil {
		t.Fatal("intent = nil, want AnchorIntent")
	}
	wantWsPath := filepath.Join(base, "myproj.code-workspace")
	if intent.WorkspacePath != wantWsPath {
		t.Errorf("intent.WorkspacePath = %q, want %q", intent.WorkspacePath, wantWsPath)
	}
	// Linked color should be the anchor with a non-zero lightness offset
	// (the linked worktree's IdentityHash is non-zero, so offset != 0).
	if c == intent.AnchorColor {
		t.Errorf("linked color = anchor; expected a lightness offset")
	}
	if len(warns) == 0 || !strings.Contains(warns[0], "family anchor created") {
		t.Errorf("warns = %v, want anchor-created notice", warns)
	}
}

func TestResolveColor_A2_MainForce_NoColor_BuildsIntent(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	for _, p := range []string{mainPath, feat} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#7a5bac")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, _, _, propagate, err := ResolveColor(mainPath, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree", src)
	}
	if propagate == nil {
		t.Fatal("propagate intent = nil, want non-nil")
	}
	if propagate.AnchorColor != c {
		t.Errorf("intent.AnchorColor = %v, want %v (= ColorHex)", propagate.AnchorColor, c)
	}
	wantAnchor := filepath.Join(base, "myproj.code-workspace")
	if propagate.AnchorPath != wantAnchor {
		t.Errorf("intent.AnchorPath = %q, want %q", propagate.AnchorPath, wantAnchor)
	}
	if len(propagate.Targets) != 1 {
		t.Errorf("targets = %v, want 1 (feat-x)", propagate.Targets)
	}
	original := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if c == original {
		t.Errorf("got same color as existing main color (%v); A2 should regenerate when no --color given", c)
	}
}

func TestResolveColor_A2_MainForce_WithColor_UsesFlag(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	for _, p := range []string{mainPath, feat} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#7a5bac")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, _, _, propagate, err := ResolveColor(mainPath, "#aabbcc", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree (A2 with --color uses flag as anchor)", src)
	}
	want := color.Color{R: 0xaa, G: 0xbb, B: 0xcc}
	if c != want {
		t.Errorf("color = %v, want #aabbcc", c)
	}
	if propagate == nil {
		t.Fatal("propagate = nil")
	}
	if propagate.AnchorColor != want {
		t.Errorf("anchor = %v, want #aabbcc", propagate.AnchorColor)
	}
}

func TestResolveColor_A2_NoForce_FallsThrough(t *testing.T) {
	// Without --force, the runner short-circuits on existing peacock keys.
	// At the resolve layer, force=false on multi-worktree main with main color
	// must not trigger A2 (no propagation, no SourceWorktree label).
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	for _, p := range []string{mainPath, feat} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#7a5bac")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	_, _, _, _, propagate, err := ResolveColor(mainPath, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if propagate != nil {
		t.Errorf("propagate intent = %v, want nil (force=false)", propagate)
	}
}

func TestFormatPropagatedWarning_AllSuccess(t *testing.T) {
	intent := &PropagateIntent{
		AnchorPath:  "/code/myproj.code-workspace",
		AnchorColor: color.Color{R: 0x5a, G: 0x3b, B: 0x8c},
		Targets: []PropagateTarget{
			{WorkspacePath: "/code/myproj-feat-x.code-workspace", DerivedColor: color.Color{R: 0x67, G: 0x47, B: 0xa4}},
			{WorkspacePath: "/code/myproj-bugfix.code-workspace", DerivedColor: color.Color{R: 0x4a, G: 0x2b, B: 0x6c}},
		},
		Skipped: []SkippedLinked{
			{WorkspacePath: "/code/myproj-hotfix.code-workspace", Reason: "no peacock keys"},
		},
	}
	got := formatPropagatedWarning(intent, nil)

	for _, want := range []string{
		"family propagated from main worktree",
		"/code/myproj.code-workspace",
		"#5a3b8c",
		"/code/myproj-feat-x.code-workspace",
		"#6747a4",
		"/code/myproj-bugfix.code-workspace",
		"/code/myproj-hotfix.code-workspace",
		"no peacock keys",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q\n%s", want, got)
		}
	}

	appliedAt := strings.Index(got, "applied")
	skippedAt := strings.Index(got, "skipped")
	if appliedAt < 0 || skippedAt < 0 {
		t.Fatalf("expected both 'applied' and 'skipped' in output, got %q", got)
	}
	if appliedAt > skippedAt {
		t.Errorf("section order: 'applied' (idx=%d) must come before 'skipped' (idx=%d)\n%s",
			appliedAt, skippedAt, got)
	}
}

func TestFormatPropagatedWarning_PartialFailure(t *testing.T) {
	intent := &PropagateIntent{
		AnchorPath:  "/code/myproj.code-workspace",
		AnchorColor: color.Color{R: 0x5a, G: 0x3b, B: 0x8c},
		Targets: []PropagateTarget{
			{WorkspacePath: "/code/myproj-feat-x.code-workspace", DerivedColor: color.Color{R: 0x67, G: 0x47, B: 0xa4}},
		},
	}
	failed := []PropagateFailure{
		{WorkspacePath: "/code/myproj-bugfix.code-workspace", Err: errors.New("permission denied")},
	}
	got := formatPropagatedWarning(intent, failed)

	for _, want := range []string{
		"applied",
		"/code/myproj-feat-x.code-workspace",
		"failed",
		"/code/myproj-bugfix.code-workspace",
		"permission denied",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q\n%s", want, got)
		}
	}

	appliedAt := strings.Index(got, "applied")
	failedAt := strings.Index(got, "failed")
	if appliedAt < 0 || failedAt < 0 {
		t.Fatalf("expected both 'applied' and 'failed' in output, got %q", got)
	}
	if appliedAt > failedAt {
		t.Errorf("section order: 'applied' (idx=%d) must come before 'failed' (idx=%d)\n%s",
			appliedAt, failedAt, got)
	}
}

func TestFormatPropagatedWarning_NoLinkedInFamily(t *testing.T) {
	intent := &PropagateIntent{
		AnchorPath:  "/code/myproj.code-workspace",
		AnchorColor: color.Color{R: 0x5a, G: 0x3b, B: 0x8c},
	}
	got := formatPropagatedWarning(intent, nil)
	if !strings.Contains(got, "no linked worktrees in family") {
		t.Errorf("warning missing empty-family hint\n%s", got)
	}
}
