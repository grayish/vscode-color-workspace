package runner

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makePreconfiguredFixture creates a temp myproj directory and a sibling
// myproj.code-workspace that already contains a peacock.color key.
// Returns (targetDir, wsFilePath).
func makePreconfiguredFixture(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	content := `{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#111111"}}`
	if err := os.WriteFile(wsPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return target, wsPath
}

func TestRun_NewProject_Random(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	r := New(opener)

	res, err := r.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if res.WorkspaceFile != wsPath {
		t.Errorf("workspace file = %q, want %q", res.WorkspaceFile, wsPath)
	}
	if _, err := os.Stat(wsPath); err != nil {
		t.Errorf("workspace not created: %v", err)
	}
	if len(opener.Calls) != 1 {
		t.Errorf("opener called %d times, want 1", len(opener.Calls))
	}
	if res.ColorSource != SourceRandom {
		t.Errorf("color source = %v, want SourceRandom", res.ColorSource)
	}
}

func TestRun_Migrate(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.MkdirAll(filepath.Join(target, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{
		"peacock.color": "#5a3b8c",
		"editor.tabSize": 2,
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c"
		}
	}`
	if err := os.WriteFile(filepath.Join(target, ".vscode", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Defaults()
	opts.TargetDir = target
	r := New(&FakeOpener{})

	res, err := r.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ColorSource != SourceSettings {
		t.Errorf("color source = %v, want SourceSettings", res.ColorSource)
	}
	data, err := os.ReadFile(filepath.Join(target, ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json read: %v", err)
	}
	if len(data) == 0 {
		t.Error("settings.json emptied unexpectedly")
	}
	if bytes.Contains(data, []byte("peacock.color")) {
		t.Error("peacock.color should have been removed")
	}
}

func TestRun_Force_BypassesPreconfigured(t *testing.T) {
	target, wsPath := makePreconfiguredFixture(t)
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222"
	opts.Force = true
	if _, err := New(&FakeOpener{}).Run(opts); err != nil {
		t.Fatalf("--force should succeed: %v", err)
	}
	data, _ := os.ReadFile(wsPath)
	if !bytes.Contains(data, []byte("#222222")) {
		t.Error("expected new color in workspace file")
	}
}

func TestRun_Guard2_Triggers(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.MkdirAll(filepath.Join(target, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c",
			"editor.background": "#000000"
		}
	}`
	if err := os.WriteFile(filepath.Join(target, ".vscode", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222"

	_, err := New(&FakeOpener{}).Run(opts)
	var gerr *GuardError
	if !errors.As(err, &gerr) {
		t.Fatalf("expected GuardError, got %T: %v", err, err)
	}
	if gerr.Guard != 2 {
		t.Errorf("guard = %d, want 2", gerr.Guard)
	}
	wantPath := filepath.Join(target, ".vscode", "settings.json")
	if gerr.Path != wantPath {
		t.Errorf("Path = %q, want %q", gerr.Path, wantPath)
	}
	if len(gerr.Keys) == 0 {
		t.Error("Keys should be non-empty")
	}
	// Error() must be a single line — used by %v / log fallback.
	if msg := gerr.Error(); msg == "" || strings.Contains(msg, "\n") {
		t.Errorf("Error() = %q, want non-empty single line", msg)
	}
}

func TestRun_NoOpen(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	opts.NoOpen = true
	if _, err := New(opener).Run(opts); err != nil {
		t.Fatal(err)
	}
	if len(opener.Calls) != 0 {
		t.Errorf("opener should not be called, got %d calls", len(opener.Calls))
	}
}

func TestRun_Preconfigured_PeacockKeysPresent(t *testing.T) {
	target, wsPath := makePreconfiguredFixture(t)
	before, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}

	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222" // should be ignored on short-circuit

	res, err := New(opener).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Preconfigured {
		t.Errorf("Preconfigured = false, want true")
	}
	if res.WorkspaceFile != wsPath {
		t.Errorf("WorkspaceFile = %q, want %q", res.WorkspaceFile, wsPath)
	}
	if len(res.PeacockKeys) != 1 || res.PeacockKeys[0] != "settings.peacock.color" {
		t.Errorf("PeacockKeys = %v, want [\"settings.peacock.color\"]", res.PeacockKeys)
	}
	if res.ColorHex != "" {
		t.Errorf("ColorHex = %q, want empty (no color resolved on short-circuit)", res.ColorHex)
	}
	if res.ColorSource != 0 {
		t.Errorf("ColorSource = %v, want zero on short-circuit", res.ColorSource)
	}
	if res.SettingsCleaned {
		t.Error("SettingsCleaned should be false on short-circuit")
	}
	after, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("workspace file should not be modified.\nbefore: %s\nafter:  %s", before, after)
	}
	if len(opener.Calls) != 1 || opener.Calls[0] != wsPath {
		t.Errorf("opener calls = %v, want [%q]", opener.Calls, wsPath)
	}
}

func TestRun_Preconfigured_NoOpen(t *testing.T) {
	target, _ := makePreconfiguredFixture(t)
	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	opts.NoOpen = true

	res, err := New(opener).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Preconfigured {
		t.Errorf("Preconfigured = false, want true")
	}
	if len(opener.Calls) != 0 {
		t.Errorf("opener should not be called with NoOpen=true, got %d calls", len(opener.Calls))
	}
}

func TestRun_Preconfigured_OpenerError(t *testing.T) {
	target, _ := makePreconfiguredFixture(t)
	opener := &FakeOpener{Err: ErrCodeNotFound}
	opts := Defaults()
	opts.TargetDir = target

	res, err := New(opener).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Preconfigured {
		t.Errorf("Preconfigured = false, want true")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning when opener fails")
	}
	if !strings.Contains(res.Warnings[0], "code CLI not on PATH") {
		t.Errorf("Warnings[0] = %q, want substring %q", res.Warnings[0], "code CLI not on PATH")
	}
}

func TestCheckPreconfigured_PeacockKeysPresent(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#abcdef"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := CheckPreconfigured(target)
	if err != nil {
		t.Fatalf("CheckPreconfigured: %v", err)
	}
	if gotPath != wsPath {
		t.Errorf("path = %q, want %q", gotPath, wsPath)
	}
	if len(gotKeys) == 0 {
		t.Error("keys should be non-empty")
	}
}

func TestCheckPreconfigured_NoPeacockKeys(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"editor.tabSize":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := CheckPreconfigured(target)
	if err != nil {
		t.Fatalf("CheckPreconfigured: %v", err)
	}
	if gotPath != "" {
		t.Errorf("path = %q, want empty (no peacock keys)", gotPath)
	}
	if len(gotKeys) != 0 {
		t.Errorf("keys = %v, want empty", gotKeys)
	}
}

func TestCheckPreconfigured_NoFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := CheckPreconfigured(target)
	if err != nil {
		t.Fatalf("CheckPreconfigured: %v", err)
	}
	if gotPath != "" || len(gotKeys) != 0 {
		t.Errorf("expected empty path/keys when ws file does not exist, got (%q, %v)", gotPath, gotKeys)
	}
}

func TestCheckPreconfigured_Unreadable(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	// Write a malformed JSON workspace file — workspace.Read should return a parse error.
	if err := os.WriteFile(wsPath, []byte(`{"folders":[`), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, gotKeys, err := CheckPreconfigured(target)
	if err == nil {
		t.Fatalf("expected parse error, got path=%q keys=%v", gotPath, gotKeys)
	}
	if gotPath != "" {
		t.Errorf("path = %q, want empty on error", gotPath)
	}
	if gotKeys != nil {
		t.Errorf("keys = %v, want nil on error", gotKeys)
	}
}

func TestRun_NoPeacockKeys_MergeStillWorks(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	// Pre-existing workspace with no peacock keys (just a folders entry and an unrelated setting).
	existing := `{"folders":[{"path":"./myproj"}],"settings":{"editor.tabSize":2}}`
	if err := os.WriteFile(wsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#abcdef"

	res, err := New(&FakeOpener{}).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Preconfigured {
		t.Errorf("Preconfigured = true, want false (no peacock keys present means short-circuit must NOT trigger)")
	}
	if res.ColorHex != "#abcdef" {
		t.Errorf("ColorHex = %q, want %q", res.ColorHex, "#abcdef")
	}
	// Workspace file must now contain the peacock color (merged in).
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("#abcdef")) {
		t.Errorf("expected merged peacock color in workspace file, content:\n%s", data)
	}
	// The pre-existing unrelated setting must still be there.
	if !bytes.Contains(data, []byte("editor.tabSize")) {
		t.Errorf("expected pre-existing editor.tabSize to be preserved, content:\n%s", data)
	}
}
