# Default-launch on existing peacock workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Change ccws's default so that an existing `<parent>/<folder>.code-workspace` containing peacock keys is opened with `code` (warn + exit 0) instead of being rejected by Guard 1 (exit 2). `--force` retains the overwrite path. Interactive mode gains a Phase A pre-check with three options.

**Architecture:** Runner gains a non-error short-circuit. `runner.Result` grows two fields (`Preconfigured bool`, `PeacockKeys []string`). `runner.Run` reads the workspace file before resolving color and returns early when the workspace already has peacock keys and `Force == false`. `cmd/ccws/render.go` adds `renderPreconfigured`. `cmd/ccws/interactive.go` adds a `detectPreconfigured` helper and a 3-option `huh.Select` before the form. Guard 2 path is unchanged.

**Tech Stack:** Go 1.25, `github.com/charmbracelet/huh` (existing), existing `internal/tui` primitives (`Writer`, `Warn`, `Details`, `ShortenPath`).

Spec: `docs/superpowers/specs/2026-04-27-default-launch-on-existing-workspace-design.md`.

---

## Output format (used by render tests)

`renderPreconfigured(w, res)` no-color output, given `WorkspaceFile="/tmp/foo.code-workspace"`, `PeacockKeys=[k1,k2,k3]`:

```
  warn   workspace already configured
         workspace     /tmp/foo.code-workspace
         peacock keys  3 existing
         hint          use --force to overwrite (other flags ignored)
```

(Continuation indent = 9 spaces, label aligned by Detail rows. `tui.ShortenPath` applied to workspace path before render — when `$HOME=/tmp`, output becomes `~/foo.code-workspace`.)

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/runner/runner.go` | MODIFY | Add `Preconfigured` + `PeacockKeys` to `Result`. Reorder `Run` so workspace.Read runs before ResolveColor. Add short-circuit branch returning `Preconfigured: true` and skipping color resolve / Guard 2 / write / cleanup. Opener still called when `!NoOpen`. |
| `internal/runner/runner_test.go` | MODIFY | Delete `TestRun_Guard1_Triggers` (path no longer exists). Rename `TestRun_Force_BypassesGuard1` → `TestRun_Force_BypassesPreconfigured` (assertion unchanged: --force writes the new color). Add `TestRun_Preconfigured_PeacockKeysPresent`, `TestRun_Preconfigured_NoOpen`, `TestRun_Preconfigured_OpenerError`. |
| `cmd/ccws/render.go` | MODIFY | Add `renderPreconfigured(w *tui.Writer, res *runner.Result)`. |
| `cmd/ccws/render_test.go` | MODIFY | Add `TestRenderPreconfigured_PlainOutput` (and a hint-line assertion). |
| `cmd/ccws/root.go` | MODIFY | RunE branches on `res.Preconfigured` → `renderPreconfigured(tui.NewStderr(), ...)`. Otherwise existing `renderSuccess` path. `renderWarnings` runs in both. |
| `cmd/ccws/interactive.go` | MODIFY | Add `detectPreconfigured` helper. Add Phase A 3-option `huh.Select` before the form. Plug Force from Phase A choice into Phase B. Handle `res.Preconfigured` in the post-Run branch (edge case where target dir was changed in form). |
| `cmd/ccws/interactive_test.go` | NEW | Tests for `detectPreconfigured` (peacock keys present / no peacock keys / no file / unreadable). |
| `README.md` | MODIFY | Update step 3 of the flow, Safety guards section, Exit codes footnote, add migration note. |
| `CLAUDE.md` | MODIFY | Update Safety guards bullets and Exit codes line. |

---

## Task 1: Add `Preconfigured` + `PeacockKeys` fields to `runner.Result`; write the failing short-circuit test

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Add fields to `Result`**

Edit `internal/runner/runner.go` — `Result` struct currently at lines 30-36. Replace with:

```go
// Result is the output of a successful Run.
type Result struct {
	WorkspaceFile   string
	ColorHex        string
	ColorSource     ColorSource
	SettingsCleaned bool
	Preconfigured   bool     // true when ws already had peacock keys and Force=false; nothing was written
	PeacockKeys     []string // existing peacock keys detected on Preconfigured short-circuit (sorted, dotted paths)
	Warnings        []string
}
```

- [ ] **Step 2: Write the failing test**

Append to `internal/runner/runner_test.go`:

```go
func TestRun_Preconfigured_PeacockKeysPresent(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	existing := `{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#111111"}}`
	if err := os.WriteFile(wsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if len(res.PeacockKeys) == 0 {
		t.Error("PeacockKeys should be non-empty")
	}
	if res.ColorHex != "" {
		t.Errorf("ColorHex = %q, want empty (no color resolved on short-circuit)", res.ColorHex)
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
```

- [ ] **Step 3: Run the new test, verify it fails**

Run: `task test -- -run TestRun_Preconfigured_PeacockKeysPresent ./internal/runner/...`
(or `go test -run TestRun_Preconfigured_PeacockKeysPresent ./internal/runner/...`)

Expected: FAIL — `Run` currently returns `*GuardError{Guard: 1}`, so `err != nil` and the assertions never run. Error like:
```
Run: guard 1: 1 conflicting keys in /tmp/.../myproj.code-workspace
```

- [ ] **Step 4: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "$(cat <<'EOF'
runner: add Preconfigured/PeacockKeys fields + failing short-circuit test

Adds the Result fields the new default-launch behavior depends on, plus
TestRun_Preconfigured_PeacockKeysPresent which currently fails because
Run still returns GuardError{Guard:1}. Implementation lands in the next
commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Implement runner short-circuit (reorder + branch)

**Files:**
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Replace the `Run` body to read workspace before resolving color and short-circuit when peacock keys are present**

Edit `internal/runner/runner.go` lines 52-144 (the `Run` method). Replace with:

```go
// Run executes the full pipeline.
func (r *Runner) Run(opts Options) (*Result, error) {
	info, err := os.Stat(opts.TargetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("target does not exist: %s", opts.TargetDir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target is not a directory: %s", opts.TargetDir)
	}
	abs, err := filepath.Abs(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")

	ws, err := workspace.Read(wsPath)
	if err != nil {
		return nil, err
	}

	// Short-circuit: existing peacock workspace, no force → skip everything,
	// just open. Guard 2 is intentionally not checked on this path.
	if ws != nil && !opts.Force {
		if keys := workspace.ExistingPeacockKeys(ws); len(keys) > 0 {
			res := &Result{
				WorkspaceFile: wsPath,
				Preconfigured: true,
				PeacockKeys:   keys,
			}
			if !opts.NoOpen {
				if err := r.Opener.Open(wsPath); err != nil {
					if errors.Is(err, ErrCodeNotFound) {
						res.Warnings = append(res.Warnings, "code CLI not on PATH; open manually: "+wsPath)
					} else {
						res.Warnings = append(res.Warnings, "failed to open with code: "+err.Error())
					}
				}
			}
			return res, nil
		}
	}

	c, src, err := ResolveColor(abs, opts.ColorInput)
	if err != nil {
		return nil, err
	}

	settingsPath := filepath.Join(abs, ".vscode", "settings.json")
	srcSettings, err := vscodesettings.Read(settingsPath)
	if err != nil {
		return nil, err
	}
	willClean := !opts.KeepSource && srcSettings != nil
	if willClean && !opts.Force {
		if keys := vscodesettings.ResidualColorKeys(srcSettings); len(keys) > 0 {
			return nil, &GuardError{Guard: 2, Path: settingsPath, Keys: keys}
		}
	}

	palette := color.Palette(c, opts.Palette)
	colorHex := c.Hex()

	if ws == nil {
		ws = &workspace.Workspace{}
	}
	workspace.EnsureFolder(ws, "./"+folderName)
	workspace.ApplyPeacock(ws, colorHex, palette)
	if err := workspace.Write(wsPath, ws); err != nil {
		return nil, err
	}

	cleaned := false
	if willClean {
		if vscodesettings.Cleanup(srcSettings) {
			if err := vscodesettings.WriteOrDelete(srcSettings); err != nil {
				return nil, err
			}
			cleaned = true
		}
	}

	var warnings []string
	if isGitRepo(parent) {
		warnings = append(warnings,
			fmt.Sprintf("parent directory %s is a git repository; workspace file may be committed", parent))
	}

	if !opts.NoOpen {
		if err := r.Opener.Open(wsPath); err != nil {
			if errors.Is(err, ErrCodeNotFound) {
				warnings = append(warnings, "code CLI not on PATH; open manually: "+wsPath)
			} else {
				warnings = append(warnings, "failed to open with code: "+err.Error())
			}
		}
	}

	return &Result{
		WorkspaceFile:   wsPath,
		ColorHex:        colorHex,
		ColorSource:     src,
		SettingsCleaned: cleaned,
		Warnings:        warnings,
	}, nil
}
```

The `*GuardError{Guard: 1}` return at the top of the method is gone; it's replaced by the short-circuit branch. The Guard 2 branch and the rest of the flow is unchanged structurally — only the order shifts (workspace read + short-circuit moved above color resolve).

- [ ] **Step 2: Run the new test, verify it passes**

Run: `task test -- -run TestRun_Preconfigured_PeacockKeysPresent ./internal/runner/...`

Expected: PASS.

- [ ] **Step 3: Run all runner tests, observe Guard 1 test failure**

Run: `task test -- ./internal/runner/...`

Expected: FAIL on `TestRun_Guard1_Triggers` (errors.As no longer finds a `*GuardError` because the path now returns `&Result{Preconfigured: true}` with `err == nil`). Other tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/runner/runner.go
git commit -m "$(cat <<'EOF'
runner: short-circuit when ws already has peacock keys

When <parent>/<folder>.code-workspace exists with peacock-managed keys
and Force is false, skip color resolve, Guard 2 check, write, and
cleanup. Open the workspace and return Result{Preconfigured: true}.
Workspace.Read is moved above ResolveColor so the short-circuit avoids
the color-resolve work entirely. Guard 2 path unchanged.

The old Guard 1 error path is removed; a follow-up commit deletes the
now-broken TestRun_Guard1_Triggers test.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Update existing runner tests for the removed Guard 1 path

**Files:**
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Delete `TestRun_Guard1_Triggers`**

Edit `internal/runner/runner_test.go` — delete the entire `TestRun_Guard1_Triggers` function (lines 82-116 in pre-change file). The condition it tested is now covered by `TestRun_Preconfigured_PeacockKeysPresent`.

- [ ] **Step 2: Rename `TestRun_Force_BypassesGuard1` → `TestRun_Force_BypassesPreconfigured`**

In the same file, rename the function. The body is unchanged — `--force` still produces a successful overwrite with the new color, only the reason for needing `--force` has shifted from "guard error" to "preconfigured short-circuit".

```go
func TestRun_Force_BypassesPreconfigured(t *testing.T) {
	// ...body unchanged...
}
```

- [ ] **Step 3: Run all runner tests**

Run: `task test -- ./internal/runner/...`

Expected: PASS. All runner tests green.

- [ ] **Step 4: Commit**

```bash
git add internal/runner/runner_test.go
git commit -m "$(cat <<'EOF'
runner: drop Guard 1 test, rename Force test for new semantics

TestRun_Guard1_Triggers tested an error path that no longer exists;
TestRun_Preconfigured_PeacockKeysPresent (added in the prior commit)
covers the same trigger condition. Force test is renamed to reflect
that --force now bypasses the Preconfigured short-circuit, not a
guard error.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add NoOpen and Opener-error tests for the short-circuit path

**Files:**
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Add `TestRun_Preconfigured_NoOpen`**

Append:

```go
func TestRun_Preconfigured_NoOpen(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#111111"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
```

- [ ] **Step 2: Add `TestRun_Preconfigured_OpenerError`**

Append:

```go
func TestRun_Preconfigured_OpenerError(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#111111"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
```

- [ ] **Step 3: Run the two new tests**

Run: `task test -- -run 'TestRun_Preconfigured_(NoOpen|OpenerError)' ./internal/runner/...`

Expected: PASS for both.

- [ ] **Step 4: Run race detector across the runner package**

Run: `task test:race -- ./internal/runner/...`
(or `go test -race ./internal/runner/...`)

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner_test.go
git commit -m "$(cat <<'EOF'
runner: cover NoOpen + opener-error in preconfigured short-circuit

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Add `renderPreconfigured` to cmd/ccws/render.go (test first)

**Files:**
- Modify: `cmd/ccws/render_test.go`
- Modify: `cmd/ccws/render.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/ccws/render_test.go`:

```go
func TestRenderPreconfigured_PlainOutput(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	res := &runner.Result{
		WorkspaceFile: "/tmp/foo.code-workspace",
		Preconfigured: true,
		PeacockKeys:   []string{"settings.peacock.color", "settings.workbench.colorCustomizations.activityBar.background", "settings.workbench.colorCustomizations.titleBar.activeBackground"},
	}
	renderPreconfigured(w, res)
	got := buf.String()
	for _, want := range []string{
		"  warn   workspace already configured\n",
		"         workspace     /tmp/foo.code-workspace\n",
		"         peacock keys  3 existing\n",
		"         hint          use --force to overwrite (other flags ignored)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fragment %q in output:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `task test -- -run TestRenderPreconfigured_PlainOutput ./cmd/ccws/...`

Expected: FAIL — `renderPreconfigured` is not defined. Compile error.

- [ ] **Step 3: Implement `renderPreconfigured`**

Append to `cmd/ccws/render.go` (after `renderWarnings`):

```go
// renderPreconfigured writes the warn block for the short-circuit case
// where the workspace already has peacock keys and ccws skipped the write.
func renderPreconfigured(w *tui.Writer, res *runner.Result) {
	w.Warn("workspace already configured")
	w.Details([]tui.Detail{
		{Label: "workspace", Value: tui.ShortenPath(res.WorkspaceFile)},
		{Label: "peacock keys", Value: fmt.Sprintf("%d existing", len(res.PeacockKeys))},
		{Label: "hint", Value: "use --force to overwrite (other flags ignored)"},
	})
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `task test -- -run TestRenderPreconfigured_PlainOutput ./cmd/ccws/...`

Expected: PASS.

- [ ] **Step 5: Run all render tests + lint**

Run: `task test -- ./cmd/ccws/... && task lint`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ccws/render.go cmd/ccws/render_test.go
git commit -m "$(cat <<'EOF'
ccws: add renderPreconfigured for soft Guard 1 notice

Warn badge + workspace path + key count + force hint, mirroring the
existing renderSuccess/renderError shape. Used by root.go and (via
runner short-circuit) interactive.go.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Wire `renderPreconfigured` into root.go

**Files:**
- Modify: `cmd/ccws/root.go`

- [ ] **Step 1: Branch RunE on `res.Preconfigured`**

Edit `cmd/ccws/root.go` — the RunE body currently at lines 42-59. Replace with:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			opts := runner.Defaults()
			opts.TargetDir = target
			opts.ColorInput = flagColor
			opts.NoOpen = flagNoOpen
			opts.Force = flagForce
			res, err := runner.New(nil).Run(opts)
			if err != nil {
				return err
			}
			if res.Preconfigured {
				renderPreconfigured(tui.NewStderr(), res)
			} else {
				renderSuccess(tui.NewStdout(), res, sourceLabel(res.ColorSource))
			}
			renderWarnings(tui.NewStderr(), res.Warnings)
			return nil
		},
```

- [ ] **Step 2: Build and run the lint**

Run: `task build && task lint`

Expected: build succeeds (binary at `./ccws`), lint passes.

- [ ] **Step 3: Manual smoke test — ccws on a fresh folder**

```bash
TMPDIR=$(mktemp -d) && \
  mkdir -p "$TMPDIR/proj" && \
  ./ccws --no-open "$TMPDIR/proj"
```

Expected: `ok` badge with workspace path and color row. `$TMPDIR/proj.code-workspace` created.

- [ ] **Step 4: Manual smoke test — ccws on the same folder again (preconfigured)**

```bash
./ccws --no-open "$TMPDIR/proj"
```

Expected: `warn` badge, "workspace already configured", workspace path, "1 existing" or more peacock keys, hint about `--force`. No file modification (verify with `stat` or re-running `cat`).

- [ ] **Step 5: Manual smoke test — `--force` overrides**

```bash
./ccws --no-open --force --color '#abcdef' "$TMPDIR/proj"
```

Expected: `ok` badge with new color `#abcdef`. Workspace file updated.

- [ ] **Step 6: Commit**

```bash
git add cmd/ccws/root.go
git commit -m "$(cat <<'EOF'
ccws: route Preconfigured result through renderPreconfigured

When runner short-circuits on existing peacock workspace, render the
warn block to stderr and exit 0. Successful new/overwrite runs still
go through renderSuccess on stdout. Warnings render as before.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `detectPreconfigured` helper + unit tests

**Files:**
- Modify: `cmd/ccws/interactive.go`
- Create: `cmd/ccws/interactive_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/ccws/interactive_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests, verify compile failure**

Run: `task test -- -run TestDetectPreconfigured ./cmd/ccws/...`

Expected: FAIL — `detectPreconfigured` is not defined.

- [ ] **Step 3: Implement `detectPreconfigured`**

Edit `cmd/ccws/interactive.go`. Add the helper between `interactiveCmd` and `runInteractive`:

```go
// detectPreconfigured returns the workspace file path and existing peacock
// keys when target/<...>.code-workspace already has peacock keys; otherwise
// returns ("", nil, nil). A read error (other than "not exist") is returned
// to the caller.
func detectPreconfigured(target string) (string, []string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", nil, err
	}
	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")
	ws, err := workspace.Read(wsPath)
	if err != nil {
		return "", nil, err
	}
	if ws == nil {
		return "", nil, nil
	}
	keys := workspace.ExistingPeacockKeys(ws)
	if len(keys) == 0 {
		return "", nil, nil
	}
	return wsPath, keys, nil
}
```

Add `"github.com/sang-bin/vscode-color-workspace/internal/workspace"` to the imports if not already there.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `task test -- -run TestDetectPreconfigured ./cmd/ccws/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ccws/interactive.go cmd/ccws/interactive_test.go
git commit -m "$(cat <<'EOF'
ccws: add detectPreconfigured helper for interactive Phase A

Pure helper that reads the workspace file at <parent>/<folder>.code-workspace
and returns the path + peacock keys if any exist. Used by the upcoming
Phase A pre-check.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Phase A 3-option select + Phase B integration

**Files:**
- Modify: `cmd/ccws/interactive.go`

- [ ] **Step 1: Replace `runInteractive` with the Phase A + Phase B flow**

Edit `cmd/ccws/interactive.go`. Replace the current `runInteractive` function (lines 27-67) with:

```go
func runInteractive(args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	// Phase A: pre-check for an existing peacock-configured workspace.
	wsPath, keys, err := detectPreconfigured(abs)
	if err != nil {
		return err
	}
	forcePreselected := false
	if len(keys) > 0 {
		var choice string
		desc := fmt.Sprintf("%s\n%d peacock keys present", tui.ShortenPath(wsPath), len(keys))
		if err := huh.NewSelect[string]().
			Title("Workspace already configured").
			Description(desc).
			Options(
				huh.NewOption("Open existing workspace", "open"),
				huh.NewOption("Overwrite (start fresh)", "overwrite"),
				huh.NewOption("Cancel", "cancel"),
			).
			Value(&choice).
			Run(); err != nil {
			return err
		}
		switch choice {
		case "open":
			opts := runner.Defaults()
			opts.TargetDir = abs
			res, err := runner.New(nil).Run(opts)
			if err != nil {
				return err
			}
			renderPreconfigured(tui.NewStderr(), res)
			renderWarnings(tui.NewStderr(), res.Warnings)
			return nil
		case "cancel":
			return nil
		case "overwrite":
			forcePreselected = true
			// fall through to Phase B
		}
	}

	// Phase B: regular form flow.
	choices, err := interactive.Run(abs)
	if err != nil {
		return err
	}
	opts := interactive.ApplyToOptions(*choices, choices.TargetDir)
	if forcePreselected {
		opts.Force = true
	}

	for attempt := 0; attempt < 2; attempt++ {
		res, err := runner.New(nil).Run(opts)
		if err == nil {
			if res.Preconfigured {
				renderPreconfigured(tui.NewStderr(), res)
			} else {
				renderSuccess(tui.NewStdout(), res, "")
			}
			renderWarnings(tui.NewStderr(), res.Warnings)
			return nil
		}
		var ge *runner.GuardError
		if !errors.As(err, &ge) {
			return err
		}
		if attempt > 0 {
			return err
		}
		ok, cerr := confirmGuard(ge)
		if cerr != nil {
			return cerr
		}
		if !ok {
			return fmt.Errorf("aborted (guard %d)", ge.Guard)
		}
		opts.Force = true
	}
	return nil
}
```

- [ ] **Step 2: Update imports**

Make sure `cmd/ccws/interactive.go` imports include all of:
- `"errors"` (existing)
- `"fmt"` (existing)
- `"path/filepath"` (existing)
- `"github.com/charmbracelet/huh"` (existing)
- `"github.com/spf13/cobra"` (existing)
- `"github.com/sang-bin/vscode-color-workspace/internal/interactive"` (existing)
- `"github.com/sang-bin/vscode-color-workspace/internal/runner"` (existing)
- `"github.com/sang-bin/vscode-color-workspace/internal/tui"` (existing)
- `"github.com/sang-bin/vscode-color-workspace/internal/workspace"` (added in Task 7)

- [ ] **Step 3: Build and lint**

Run: `task build && task lint`

Expected: build succeeds, lint passes.

- [ ] **Step 4: Run all package tests**

Run: `task test`

Expected: PASS across all packages.

- [ ] **Step 5: Manual smoke — interactive on a preconfigured folder, "Open existing"**

```bash
TMPDIR=$(mktemp -d) && \
  mkdir -p "$TMPDIR/proj" && \
  ./ccws --no-open "$TMPDIR/proj"   # creates the workspace
./ccws interactive "$TMPDIR/proj"
# At the Phase A select, pick "Open existing workspace"
```

Expected: Phase A select appears with workspace path and key count. After picking "Open existing", the form is skipped and a `warn` block is printed (and `code` is invoked). Exit 0.

- [ ] **Step 6: Manual smoke — "Overwrite" path goes through the full form**

```bash
./ccws interactive "$TMPDIR/proj"
# Pick "Overwrite (start fresh)" — full form should appear
# Walk through, finish
```

Expected: Form runs to completion. `ok` badge with the new color. Workspace file updated.

- [ ] **Step 7: Manual smoke — "Cancel" exits cleanly**

```bash
./ccws interactive "$TMPDIR/proj" ; echo "exit=$?"
# Pick "Cancel"
```

Expected: No form, no output (or minimal), exit 0.

- [ ] **Step 8: Manual smoke — interactive on a non-preconfigured folder skips Phase A**

```bash
TMPDIR2=$(mktemp -d) && mkdir -p "$TMPDIR2/fresh"
./ccws interactive "$TMPDIR2/fresh"
# Form appears immediately (no Phase A select)
```

Expected: Phase A is bypassed; the existing form runs as before.

- [ ] **Step 9: Commit**

```bash
git add cmd/ccws/interactive.go
git commit -m "$(cat <<'EOF'
ccws: interactive Phase A pre-check for existing peacock workspace

Before showing the huh form, detect an existing peacock-configured
workspace and offer three options: Open existing (short-circuit via
runner), Overwrite (continue with form, Force=true preselected), or
Cancel (exit 0). The post-Run branch in Phase B also handles
Preconfigured as an edge case (user may change target dir in the form).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Update README.md and CLAUDE.md

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update step 3 of the flow in README.md**

Edit `README.md`. Replace step 3 of the "Running `ccws` in `/home/me/code/myproj` will:" list with:

```
3. Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file). **If the workspace file already contains peacock keys, ccws skips the write, prints a warning, and just opens it. Pass `--force` to overwrite.**
```

- [ ] **Step 2: Update Safety guards section in README.md**

Replace the `## Safety guards` section with:

```markdown
## Safety guards

- **Guard 1 (soft) — existing peacock keys in the workspace file.** ccws prints a warning, opens the workspace as-is, and exits 0. `.vscode/settings.json` is not touched on this path. Pass `--force` to overwrite (this also re-runs cleanup against `.vscode/settings.json`).
- **Guard 2 — non-peacock `workbench.colorCustomizations` would remain in `.vscode/settings.json`.** ccws refuses to proceed and exits with code 2. Remove those keys manually or pass `--force`.

`ccws interactive` shows Guard 1 as a 3-option pre-check (Open existing / Overwrite / Cancel) before the form, and Guard 2 as a confirmation prompt during the run.
```

- [ ] **Step 3: Update Exit codes table in README.md**

Replace the `## Exit codes` table with:

```markdown
## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success (including the soft Guard 1 case where ccws opens an existing peacock workspace) |
| 1 | input error (invalid color, missing folder, parse failure) |
| 2 | Guard 2 triggered (non-peacock keys would remain in `.vscode/settings.json` after cleanup) |
| 3 | filesystem error |

> **Behavior change since v0.1:** prior versions exited with code 2 when the target's workspace file already contained peacock keys. As of this version, ccws prints a warning, opens the existing workspace, and exits 0. Shell scripts that depended on the old exit-2 path for this case must now check stderr for the "workspace already configured" notice or always pass `--force`.
```

- [ ] **Step 4: Update Safety guards section in CLAUDE.md**

Edit `CLAUDE.md`. Replace the existing Safety guards block:

```markdown
## Safety guards (project-specific terminology)

Used in error messages, tests, and commit messages:

- **Guard 1 (soft)** — existing Peacock keys in the target `.code-workspace`. Default: warn + open existing, exit 0. Skips Guard 2 check on this path. `--force` overwrites and re-runs cleanup.
- **Guard 2** — non-Peacock keys would remain in `.vscode/settings.json` after cleanup. Default: exit 2. `--force` bypasses.

Either Guard 2 (or any other unhandled error) → CLI exit code 2/1/3 per `cmd/ccws/root.go:errToExit`. Interactive mode handles Guard 1 in a Phase A pre-check (`huh.Select`: Open existing / Overwrite / Cancel) and Guard 2 with a confirm prompt during the run.
```

- [ ] **Step 5: Update Exit codes section in CLAUDE.md**

Replace the existing Exit codes block:

```markdown
## Exit codes

`0` success (including soft Guard 1) · `1` input error · `2` Guard 2 triggered · `3` filesystem error. Mapping lives in `cmd/ccws/root.go:errToExit`.
```

- [ ] **Step 6: Run lint to make sure no markdown linting hooks complain**

Run: `task lint`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: README + CLAUDE.md for soft Guard 1 default

Document the new default-launch behavior on existing peacock
workspaces, the migration note for shell scripts that depended on
exit 2, and the interactive Phase A pre-check.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Final CI verification

**Files:** none (verification only)

- [ ] **Step 1: Run `task ci`**

Run: `task ci`

Expected: lint + test:race PASS across all packages.

- [ ] **Step 2: Build a fresh binary**

Run: `task build`

Expected: `./ccws` produced.

- [ ] **Step 3: End-to-end smoke (non-interactive)**

```bash
TMPDIR=$(mktemp -d) && mkdir -p "$TMPDIR/proj"
./ccws --no-open "$TMPDIR/proj"                              # ok badge
./ccws --no-open "$TMPDIR/proj"                              # warn badge (preconfigured)
./ccws --no-open --force --color '#abcdef' "$TMPDIR/proj"    # ok badge with new color
./ccws --no-open --color red "$TMPDIR/proj"                  # warn badge (--color silently ignored)
```

Expected: badges as annotated above. Workspace file modified only on the 1st and 3rd commands. Exit 0 for all four.

- [ ] **Step 4: End-to-end smoke (NO_COLOR + pipe)**

```bash
NO_COLOR=1 ./ccws --no-open "$TMPDIR/proj" 2>&1 | head
./ccws --no-open "$TMPDIR/proj" 2>&1 | cat
```

Expected: no ANSI escapes in output. Badges appear as plain "warn" / "ok" text.

- [ ] **Step 5: Confirm no lingering Guard 1 references in error messages or tests**

```bash
grep -rn "Guard 1\|guard 1" internal/ cmd/ README.md CLAUDE.md
```

Expected: only references in CLAUDE.md ("Guard 1 (soft)") and README.md ("Guard 1 (soft)") that match the new wording. No `*GuardError{Guard: 1}` literals in code. No old test names.

If grep finds stale references, fix them and amend the relevant commit (or add a small follow-up commit).

- [ ] **Step 6: Done**

All tasks complete. Branch ready for review/merge.

---

## Self-review (executed)

**Spec coverage:**

| Spec section | Covered by |
|---|---|
| §2 Trigger matrix | Tasks 1-4 (runner short-circuit and tests), Task 6 (root.go branch) |
| §3 Render shape | Task 5 (`renderPreconfigured` + test) |
| §4 Result fields + Run reorder | Tasks 1-2 |
| §5 CLI render + exit codes | Tasks 5-6 (renderPreconfigured wired into root.go; errToExit unchanged) |
| §6 Interactive Phase A | Tasks 7-8 |
| §7 Tests (runner / render / interactive) | Tasks 1-5, 7 |
| §7 Manual smoke list | Task 6 (steps 3-5) and Task 10 (steps 3-4) |
| §8 Migration note | Task 9 step 3 |
| §9 Doc changes (README + CLAUDE.md) | Task 9 |

**Placeholder scan:** all steps contain concrete code, paths, and commands. No "TBD" or "implement appropriate handling" placeholders.

**Type / signature consistency:**
- `Result.Preconfigured bool` and `Result.PeacockKeys []string` — defined in Task 1 step 1, used in Task 1 step 2 test, Task 4 tests, Task 5 test, Task 6 RunE, Task 8 Phase B post-Run.
- `detectPreconfigured(target string) (string, []string, error)` — declared in Task 7 helper, called in Task 7 tests and Task 8 Phase A.
- `renderPreconfigured(w *tui.Writer, res *runner.Result)` — declared in Task 5 step 3, called in Task 6 (root.go) and Task 8 (interactive Phase A and Phase B post-Run).
- `runner.New(nil).Run(opts)` short-circuit semantics: when `opts.Force == false` and ws has peacock keys, returns `(&Result{Preconfigured: true, ...}, nil)` — used by Task 8 Phase A's "open" branch.
