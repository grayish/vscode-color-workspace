# Worktree main `--force` propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the misleading "from worktree family" label on single-worktree repos and add `--force`-on-main propagation that updates anchor + all colored linked workspaces.

**Architecture:** Subdivide existing Case A in `resolveFromWorktree` into A1 (single-worktree main → fall through), A2 (multi-worktree main + force → propagate), and A3 (linked target → unchanged). A2 returns a new `PropagateIntent` describing all linked targets and their derived colors; `runner.Run` executes the writes, accumulates failures, and emits a Case-C-style multi-line warn.

**Tech Stack:** Go 1.22, existing internal packages (`gitworktree`, `color`, `workspace`, `vscodesettings`, `peacock`).

**Spec:** `docs/superpowers/specs/2026-05-01-worktree-main-force-propagation-design.md`

---

## File Map

| File | Responsibility | Change type |
|---|---|---|
| `internal/runner/resolve.go` | A1/A2/A3 branching, `PropagateIntent` types, `buildPropagateTargets`, `formatPropagatedWarning` | modify |
| `internal/runner/resolve_test.go` | A1/A2 tests, signature updates | modify |
| `internal/runner/runner.go` | `writeFamilyPropagation`, `ErrPartialPropagation`, `Run` wiring, `Result` fields | modify |
| `internal/runner/runner_test.go` | A2 integration tests | modify |
| `internal/runner/interactive.go` *(if it calls ResolveColor)* | Pass `force` through | modify |
| `cmd/ccws/root.go` | Render result before returning `ErrPartialPropagation` for exit 1 | modify |
| `README.md` | `--force` section: document main-propagation | modify |
| `CLAUDE.md` | Safety guards section: note A2 writes to other dirs | modify |

DAG unchanged; no new files.

---

## Task 1: Add `force` and `flag` parameters to ResolveColor / resolveFromWorktree

**Why first:** All subsequent tasks need these parameters. Doing the signature change in one task isolates the noise.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/interactive.go` (if it calls `ResolveColor`)

- [ ] **Step 1: Inspect interactive.go for ResolveColor callers**

Run: `grep -n "ResolveColor" internal/runner/*.go cmd/ccws/*.go`
Note all callers — they must all be updated.

- [ ] **Step 2: Update `ResolveColor` signature**

In `internal/runner/resolve.go`, change:

```go
func ResolveColor(targetDir, flag string, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, error) {
```

to:

```go
func ResolveColor(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, error) {
```

Inside `ResolveColor`, change the call to `resolveFromWorktree`:

```go
c, src, warns, intent, ok, err := resolveFromWorktree(targetDir, debug)
```

to:

```go
c, src, warns, intent, ok, err := resolveFromWorktree(targetDir, flag, force, debug)
```

- [ ] **Step 3: Update `resolveFromWorktree` signature**

Change:

```go
func resolveFromWorktree(targetDir string, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
```

to:

```go
func resolveFromWorktree(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
```

Body unchanged this task — `flag` and `force` are unused for now. Add `_ = flag; _ = force` only if Go complains (it shouldn't; unused params are allowed).

- [ ] **Step 4: Update callers in runner.go**

In `internal/runner/runner.go`, find:

```go
c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput, opts.Debug)
```

Replace with:

```go
c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput, opts.Force, opts.Debug)
```

- [ ] **Step 5: Update interactive.go callers (if any)**

If `grep` in Step 1 found any `ResolveColor` calls in `interactive.go`, update them analogously to pass `opts.Force` (or `false` if force is not relevant in that flow).

- [ ] **Step 6: Update tests in resolve_test.go**

In `internal/runner/resolve_test.go`, find every `ResolveColor(...)` call in tests. Each currently looks like:

```go
got, src, _, _, err := ResolveColor(dir, "#222222", false)
```

Update to insert a `false` for the new `force` parameter (between flag and debug):

```go
got, src, _, _, err := ResolveColor(dir, "#222222", false, false)
```

Apply this pattern to every call — search results from Step 1 list them all.

- [ ] **Step 7: Run tests, expect green**

```bash
task test
```

Expected: all tests pass. No behavior change yet.

- [ ] **Step 8: Run lint**

```bash
task lint
```

Expected: clean.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: add force/flag params to ResolveColor signature

Pure signature change — no behavior change yet. Subsequent tasks use
these to drive A1 (single-worktree main fall-through) and A2 (multi-
worktree main propagate) branches.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add propagation types, errors, and Result fields

Declare the data shapes used by Tasks 4-8. Unused for now — they compile but don't change behavior.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Add types to resolve.go**

In `internal/runner/resolve.go`, append after the existing `AnchorIntent` block (around line 41):

```go
// PropagateIntent describes A2 side effects: write the anchor color to main's
// .code-workspace, then write each derived color to the corresponding linked
// worktree's .code-workspace. The runner executes the writes; resolve only
// computes the targets and skip list.
type PropagateIntent struct {
	AnchorPath  string // ws(main)
	AnchorColor color.Color
	Targets     []PropagateTarget
	Skipped     []SkippedLinked
}

// PropagateTarget is a linked worktree with its derived color.
type PropagateTarget struct {
	WorkspacePath string
	DerivedColor  color.Color
}

// SkippedLinked is a linked worktree that was not in the family (no peacock
// keys, no .code-workspace, parse error, etc.). The reason is short text
// suitable for display.
type SkippedLinked struct {
	WorkspacePath string
	Reason        string
}

// PropagateFailure is a linked worktree where the write attempt failed at
// runtime (permission denied, disk full, etc.).
type PropagateFailure struct {
	WorkspacePath string
	Err           error
}
```

- [ ] **Step 2: Add types and sentinel to runner.go**

In `internal/runner/runner.go`, near the top of the file (after imports, before existing types):

```go
// ErrPartialPropagation is returned by Run when A2 family propagation
// completed with one or more linked write failures. The accompanying
// *Result is populated; the caller should render Result.Warnings, then
// surface this error to set exit code 1.
var ErrPartialPropagation = errors.New("runner: family propagation had failures")

// PropagateResult carries the outcome of writeFamilyPropagation.
type PropagateResult struct {
	Applied []string
	Failed  []PropagateFailure
}
```

- [ ] **Step 3: Add fields to Result struct**

In `internal/runner/runner.go`, find the `Result` type:

```go
type Result struct {
	WorkspaceFile   string
	ColorHex        string
	ColorSource     ColorSource
	SettingsCleaned bool
	Preconfigured   bool
	PeacockKeys     []string
	Warnings        []string
}
```

Add three new fields at the end:

```go
type Result struct {
	WorkspaceFile   string
	ColorHex        string
	ColorSource     ColorSource
	SettingsCleaned bool
	Preconfigured   bool
	PeacockKeys     []string
	Warnings        []string
	PropagatedTo    []string             // A2: linked ws paths written successfully
	SkippedLinked   []SkippedLinked      // A2: linked ws paths skipped (with reason)
	FailedLinked    []PropagateFailure   // A2: linked ws paths where write failed
}
```

- [ ] **Step 4: Verify imports in runner.go**

Ensure `"errors"` is in the import block (it likely already is — check).

- [ ] **Step 5: Run tests, expect green**

```bash
task test
```

Expected: all pass. New types are declared but unused — Go allows this at package scope.

- [ ] **Step 6: Run lint**

```bash
task lint
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: declare PropagateIntent types and ErrPartialPropagation

Types and sentinel error are unused this commit; they will be wired up
in subsequent tasks (buildPropagateTargets, writeFamilyPropagation,
Run integration).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: A1 — single-worktree main fall-through (the bug fix)

Add the `len(worktrees) == 1 && self.IsMain` short-circuit so single-worktree main with existing color falls through to settings/random instead of hitting Case A. This fixes the misleading "from worktree family" label on regular repos.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Update existing test to assert new A1 behavior**

In `internal/runner/resolve_test.go`, find `TestResolveColor_WorktreeCaseA_MainTarget` (around line 176) and replace its body:

```go
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

	c, src, _, _, err := ResolveColor(mainPath, "", true, false)
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
```

Also rename the test function (was `TestResolveColor_WorktreeCaseA_MainTarget`).

- [ ] **Step 2: Run the test, expect failure**

```bash
go test ./internal/runner/ -run TestResolveColor_A1_SingleWorktreeMain_FallsThrough -v
```

Expected: FAIL — current code returns `SourceWorktree`.

- [ ] **Step 3: Implement A1 in resolveFromWorktree**

In `internal/runner/resolve.go`, find the Case A block (currently around lines 162-168):

```go
// Case A: main has a color — anchor + offset
if mainColor != nil {
	offset := color.LadderOffset(gitworktree.IdentityHash(*self))
	derived := mainColor.ApplyLightness(offset)
	dbg(debug, "  Case A: mainColor=%s offset=%v derived=%s", mainColor.Hex(), offset, derived.Hex())
	return derived, SourceWorktree, nil, nil, true, nil
}
```

Replace with:

```go
// Case A1: target is the only worktree (regular git repo, no linked).
// "Family" doesn't apply — fall through to settings/random.
if mainColor != nil && self.IsMain && len(worktrees) == 1 {
	dbg(debug, "  Case A1: single-worktree main — skip worktree logic")
	return color.Color{}, 0, nil, nil, false, nil
}

// Case A: main has a color — anchor + offset (A3 in spec terminology)
if mainColor != nil {
	offset := color.LadderOffset(gitworktree.IdentityHash(*self))
	derived := mainColor.ApplyLightness(offset)
	dbg(debug, "  Case A: mainColor=%s offset=%v derived=%s", mainColor.Hex(), offset, derived.Hex())
	return derived, SourceWorktree, nil, nil, true, nil
}
```

- [ ] **Step 4: Run the test, expect pass**

```bash
go test ./internal/runner/ -run TestResolveColor_A1_SingleWorktreeMain_FallsThrough -v
```

Expected: PASS.

- [ ] **Step 5: Run all tests, expect green**

```bash
task test
```

Expected: all pass. The A3 (linked-target) test still works — A1 only triggers when `self.IsMain && len == 1`.

- [ ] **Step 6: Run lint**

```bash
task lint
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: skip worktree logic for single-worktree main (A1)

Single-worktree git repos have no family — git worktree list returns
the repo itself as a single 'main' entry. Falling through to the
settings/random chain produces a correct SourceSettings/SourceRandom
label instead of the misleading 'from worktree family' on --force re-runs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `buildPropagateTargets` helper

Compute the per-linked classification (Target vs SkippedLinked) given an anchor color and the worktree list. Pure function, easy to unit-test.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/runner/resolve_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/runner/ -run TestBuildPropagateTargets -v
```

Expected: FAIL — `buildPropagateTargets` does not exist.

- [ ] **Step 3: Implement `buildPropagateTargets`**

Append to `internal/runner/resolve.go` (private, near other resolve helpers):

```go
// buildPropagateTargets classifies every linked worktree into either a
// PropagateTarget (will be written) or a SkippedLinked entry (skipped with
// a short reason). The main worktree is excluded from both lists. The anchor
// color is what the caller has decided to apply to main.
func buildPropagateTargets(worktrees []gitworktree.Worktree, anchor color.Color) ([]PropagateTarget, []SkippedLinked) {
	var targets []PropagateTarget
	var skipped []SkippedLinked
	for i := range worktrees {
		w := &worktrees[i]
		if w.IsMain {
			continue
		}
		wsPath, err := workspaceFilePath(w.Path)
		if err != nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: w.Path,
				Reason:        "could not derive workspace path: " + err.Error(),
			})
			continue
		}
		ws, err := workspace.Read(wsPath)
		if err != nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "parse error: " + err.Error(),
			})
			continue
		}
		if ws == nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "no .code-workspace",
			})
			continue
		}
		if len(workspace.ExistingPeacockKeys(ws)) == 0 {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "no peacock keys",
			})
			continue
		}
		offset := color.LadderOffset(gitworktree.IdentityHash(*w))
		derived := anchor.ApplyLightness(offset)
		targets = append(targets, PropagateTarget{
			WorkspacePath: wsPath,
			DerivedColor:  derived,
		})
	}
	return targets, skipped
}
```

Make sure `workspace` is in the imports (it already is — `internal/workspace`).

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/runner/ -run TestBuildPropagateTargets -v
```

Expected: both tests PASS.

- [ ] **Step 5: Run all tests**

```bash
task test
```

- [ ] **Step 6: Run lint**

```bash
task lint
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: add buildPropagateTargets for A2 classification

Pure function that splits linked worktrees into propagation targets
(have peacock keys → derive offset color) vs skipped (no .code-workspace,
no peacock keys, or parse error).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: A2 — emit PropagateIntent from resolveFromWorktree

Add the A2 case (multi-worktree main + force) and extend signatures so callers receive a `*PropagateIntent`. Resolve does not yet drive any writes — that's Task 8.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Write failing tests for A2**

Append to `internal/runner/resolve_test.go`:

```go
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
```

Note: these tests assume `ResolveColor` returns 6 values now (added `*PropagateIntent`). The next steps update the signature.

- [ ] **Step 2: Update `resolveFromWorktree` signature**

Change:

```go
func resolveFromWorktree(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
```

to:

```go
func resolveFromWorktree(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, *PropagateIntent, bool, error) {
```

Update all `return` statements inside to insert a `nil` for the new slot, e.g.:

```go
return color.Color{}, 0, nil, nil, false, nil
```
becomes:
```go
return color.Color{}, 0, nil, nil, nil, false, nil
```

- [ ] **Step 3: Restructure `ResolveColor` to let resolveFromWorktree see the flag**

The current `ResolveColor` returns `SourceFlag` immediately when `flag != ""` — that bypasses the worktree logic. For A2, we need `resolveFromWorktree` to see the flag (so it can use it as the anchor) BEFORE deciding to return `SourceFlag`. Restructure:

Replace the existing `ResolveColor` body so the flag handling is split into two phases — validate up front, then either let A2 consume it or fall back to `SourceFlag` after worktree logic skips:

```go
func ResolveColor(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, *PropagateIntent, error) {
	dbg(debug, "ResolveColor: targetDir=%q flag=%q force=%v", targetDir, flag, force)

	var parsedFlag color.Color
	if flag != "" {
		p, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, nil, nil, nil, fmt.Errorf("--color: %w", err)
		}
		parsedFlag = p
	}

	c, src, warns, anchorIntent, propagateIntent, ok, err := resolveFromWorktree(targetDir, flag, force, debug)
	if err != nil {
		return color.Color{}, 0, nil, nil, nil, err
	}
	if ok {
		dbg(debug, "ResolveColor: worktree logic decided source=%v color=%s", src, c.Hex())
		return c, src, warns, anchorIntent, propagateIntent, nil
	}
	dbg(debug, "ResolveColor: worktree logic skipped — falling through")

	if flag != "" {
		dbg(debug, "ResolveColor: source=Flag color=%s", parsedFlag.Hex())
		return parsedFlag, SourceFlag, warns, nil, nil, nil
	}

	// fall through to settings.json → random — preserve any Case-D warnings
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, warns, nil, nil, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			parsed, perr := color.Parse(pc)
			if perr != nil {
				return color.Color{}, 0, warns, nil, nil, fmt.Errorf("peacock.color in settings: %w", perr)
			}
			dbg(debug, "ResolveColor: source=Settings color=%s", parsed.Hex())
			return parsed, SourceSettings, warns, nil, nil, nil
		}
	}
	rc := color.Random()
	dbg(debug, "ResolveColor: source=Random color=%s", rc.Hex())
	return rc, SourceRandom, warns, nil, nil, nil
}
```

This replaces the entire body of `ResolveColor`. Note the signature now has 6 return slots (`*PropagateIntent` added) and the flag-handling has been moved AFTER the worktree call.

- [ ] **Step 4: Add A2 case in `resolveFromWorktree`**

Inside `resolveFromWorktree`, after the A1 short-circuit (which you added in Task 3) and BEFORE the existing Case A:

```go
// Case A2: target is main of a multi-worktree repo and --force given.
// Regenerate anchor and propagate to all colored linked worktrees.
if mainColor != nil && self.IsMain && len(worktrees) > 1 && force {
	var anchor color.Color
	if flag != "" {
		parsed, perr := color.Parse(flag)
		if perr != nil {
			return color.Color{}, 0, nil, nil, nil, false, fmt.Errorf("--color: %w", perr)
		}
		anchor = parsed
	} else {
		anchor = color.Random()
	}
	targets, skipped := buildPropagateTargets(worktrees, anchor)
	intent := &PropagateIntent{
		AnchorPath:  mainWsPath,
		AnchorColor: anchor,
		Targets:     targets,
		Skipped:     skipped,
	}
	dbg(debug, "  Case A2: anchor=%s targets=%d skipped=%d", anchor.Hex(), len(targets), len(skipped))
	return anchor, SourceWorktree, nil, nil, intent, true, nil
}
```

This block sits between the A1 fallthrough (added in Task 3) and the existing `if mainColor != nil { ... }` Case A block (which now serves as A3, the linked-target case).

- [ ] **Step 5: Update callers in runner.go**

In `internal/runner/runner.go`, find:

```go
c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput, opts.Force, opts.Debug)
```

Replace with:

```go
c, src, resolveWarns, anchorIntent, propagateIntent, err := ResolveColor(abs, opts.ColorInput, opts.Force, opts.Debug)
```

Add `_ = propagateIntent` somewhere safe (e.g. immediately after) to silence the unused-variable error — Task 8 wires it in. Alternatively, add a TODO branch:

```go
if propagateIntent != nil {
	// wired in Task 8
	_ = propagateIntent
}
```

- [ ] **Step 6: Update interactive.go (if it has callers)**

If `interactive.go` calls `ResolveColor`, add the new return slot. If it ignores the propagate intent, use `_`:

```go
c, src, warns, anchorIntent, _, err := ResolveColor(...)
```

- [ ] **Step 7: Update existing test calls in resolve_test.go**

Search every `ResolveColor(...)` call. Each previously had 5 return slots; now it has 6. Update the destructuring at every call site. Pattern:

```go
got, src, _, _, err := ResolveColor(dir, "#222222", false, false)
```

becomes:

```go
got, src, _, _, _, err := ResolveColor(dir, "#222222", false, false)
```

- [ ] **Step 8: Update any other callers**

Search for `ResolveColor(` across the repo:

```bash
grep -rn "ResolveColor(" --include="*.go"
```

Update every call site to handle the new return slot.

- [ ] **Step 9: Run all tests**

```bash
task test
```

Expected: all green, including the new A2 tests added in Step 1.

- [ ] **Step 10: Run lint**

```bash
task lint
```

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: emit PropagateIntent from A2 (multi-worktree main + force)

ResolveColor now returns *PropagateIntent describing the anchor + per-
linked derived colors. The runner does not yet execute the writes;
that lands in a follow-up so this commit is purely additive at the
resolution layer.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `formatPropagatedWarning`

Format the multi-line warn for the propagation result. Mirrors `formatAnchorCreatedWarning` (Case C). `runner.Run` will call this after `writeFamilyPropagation` so it can include write-time failures.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/runner/resolve_test.go`:

```go
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
```

You will need to add `"errors"` to the imports of `resolve_test.go` if not already present.

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/runner/ -run TestFormatPropagatedWarning -v
```

Expected: FAIL — function does not exist.

- [ ] **Step 3: Implement `formatPropagatedWarning`**

The renderer (`renderWarnings` in `cmd/ccws/render.go`) splits the warn by `\n` and `TrimLeft`s leading whitespace from each non-header line, so continuation rows with blank labels would lose their alignment. To keep the rendered output readable, **repeat the section label on every row** instead of using continuation indents.

Append to `internal/runner/resolve.go`:

```go
// formatPropagatedWarning renders the multi-line warn produced by A2.
// Sections (anchor / applied / failed / skipped) appear only when populated.
// When no linked worktrees end up in any section, a one-line hint replaces them.
//
// Each row repeats its section label because cmd/ccws/render.go strips
// leading whitespace before printing — continuation indents would render
// as orphan paths.
func formatPropagatedWarning(intent *PropagateIntent, failed []PropagateFailure) string {
	var b strings.Builder
	b.WriteString("family propagated from main worktree\n")
	fmt.Fprintf(&b, "  anchor at  %s  %s", intent.AnchorPath, intent.AnchorColor.Hex())

	if len(intent.Targets) == 0 && len(failed) == 0 && len(intent.Skipped) == 0 {
		b.WriteString("\n  (no linked worktrees in family)")
		return b.String()
	}

	for _, tgt := range intent.Targets {
		fmt.Fprintf(&b, "\n  applied    %s  %s", tgt.WorkspacePath, tgt.DerivedColor.Hex())
	}
	for _, f := range failed {
		fmt.Fprintf(&b, "\n  failed     %s  %s", f.WorkspacePath, f.Err.Error())
	}
	for _, s := range intent.Skipped {
		fmt.Fprintf(&b, "\n  skipped    %s  (%s)", s.WorkspacePath, s.Reason)
	}
	return b.String()
}
```

Add `"strings"` and `"fmt"` to imports if not already present.

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/runner/ -run TestFormatPropagatedWarning -v
```

Expected: PASS for all three.

- [ ] **Step 5: Run all tests + lint**

```bash
task test && task lint
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: format multi-line warn for A2 family propagation

Mirrors the Case C anchor-created warn style: header + section labels
(anchor / applied / failed / skipped) with continuation rows aligned
under the first label.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `writeFamilyPropagation`

Execute the writes described by a `PropagateIntent`. Main first; if main fails, return hard error. Linked failures accumulate in `PropagateResult.Failed` without aborting the loop.

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/runner/runner_test.go`:

```go
func TestWriteFamilyPropagation_AllSuccess(t *testing.T) {
	base := t.TempDir()
	mainWs := filepath.Join(base, "myproj.code-workspace")
	featWs := filepath.Join(base, "myproj-feat-x.code-workspace")
	bugWs := filepath.Join(base, "myproj-bugfix.code-workspace")
	// Pre-populate target files (workspace.Read returns nil for missing,
	// so writeFamilyPropagation should still succeed via fresh struct).
	for _, p := range []string{featWs, bugWs} {
		if err := os.WriteFile(p, []byte(`{"settings":{"peacock.color":"#000000"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	intent := &PropagateIntent{
		AnchorPath:  mainWs,
		AnchorColor: color.Color{R: 0xaa, G: 0xbb, B: 0xcc},
		Targets: []PropagateTarget{
			{WorkspacePath: featWs, DerivedColor: color.Color{R: 0xa0, G: 0xb0, B: 0xc0}},
			{WorkspacePath: bugWs, DerivedColor: color.Color{R: 0xb0, G: 0xc0, B: 0xd0}},
		},
	}
	opts := Defaults()

	res, err := writeFamilyPropagation(intent, opts)
	if err != nil {
		t.Fatalf("writeFamilyPropagation: %v", err)
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v, want empty", res.Failed)
	}
	if len(res.Applied) != 2 {
		t.Errorf("Applied count = %d, want 2", len(res.Applied))
	}
	// Verify main was written
	mainBytes, err := os.ReadFile(mainWs)
	if err != nil {
		t.Fatalf("main not written: %v", err)
	}
	if !strings.Contains(string(mainBytes), "#aabbcc") {
		t.Errorf("main file missing anchor color: %s", mainBytes)
	}
	// Verify feat was updated to derived
	featBytes, err := os.ReadFile(featWs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(featBytes), "#a0b0c0") {
		t.Errorf("feat file missing derived color: %s", featBytes)
	}
}

func TestWriteFamilyPropagation_LinkedFailureCollected(t *testing.T) {
	base := t.TempDir()
	mainWs := filepath.Join(base, "myproj.code-workspace")
	roDir := filepath.Join(base, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	roWs := filepath.Join(roDir, "myproj-bugfix.code-workspace")
	if err := os.WriteFile(roWs, []byte(`{"settings":{"peacock.color":"#000000"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so the rewrite (write-to-temp + rename) fails.
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	intent := &PropagateIntent{
		AnchorPath:  mainWs,
		AnchorColor: color.Color{R: 0xaa, G: 0xbb, B: 0xcc},
		Targets: []PropagateTarget{
			{WorkspacePath: roWs, DerivedColor: color.Color{R: 0xa0, G: 0xb0, B: 0xc0}},
		},
	}
	opts := Defaults()

	res, err := writeFamilyPropagation(intent, opts)
	if err != nil {
		t.Fatalf("writeFamilyPropagation: %v (main should still succeed)", err)
	}
	if len(res.Applied) != 0 {
		t.Errorf("Applied = %v, want empty (linked write should fail)", res.Applied)
	}
	if len(res.Failed) != 1 {
		t.Fatalf("Failed count = %d, want 1", len(res.Failed))
	}
	if res.Failed[0].WorkspacePath != roWs {
		t.Errorf("Failed[0].WorkspacePath = %q, want %q", res.Failed[0].WorkspacePath, roWs)
	}
}

func TestWriteFamilyPropagation_MainWriteFailureIsHardError(t *testing.T) {
	base := t.TempDir()
	roDir := filepath.Join(base, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainWs := filepath.Join(roDir, "myproj.code-workspace")
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	intent := &PropagateIntent{
		AnchorPath:  mainWs,
		AnchorColor: color.Color{R: 0xaa, G: 0xbb, B: 0xcc},
		Targets:     nil,
	}
	if _, err := writeFamilyPropagation(intent, Defaults()); err == nil {
		t.Errorf("writeFamilyPropagation: got nil err, want main-write failure")
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/runner/ -run TestWriteFamilyPropagation -v
```

Expected: FAIL — function does not exist.

- [ ] **Step 3: Implement `writeFamilyPropagation`**

Append to `internal/runner/runner.go` (after `writeAnchorWorkspace`):

```go
// writeFamilyPropagation executes the writes described by a PropagateIntent.
// Main is written first; if that fails the function returns a hard error
// without attempting any linked writes. Otherwise, every linked target is
// attempted; failures are collected into PropagateResult.Failed and do not
// abort the loop.
func writeFamilyPropagation(intent *PropagateIntent, opts Options) (PropagateResult, error) {
	if err := writeOneWorkspace(intent.AnchorPath, intent.AnchorColor, opts); err != nil {
		return PropagateResult{}, fmt.Errorf("write main anchor workspace: %w", err)
	}
	var res PropagateResult
	for _, tgt := range intent.Targets {
		if err := writeOneWorkspace(tgt.WorkspacePath, tgt.DerivedColor, opts); err != nil {
			res.Failed = append(res.Failed, PropagateFailure{
				WorkspacePath: tgt.WorkspacePath,
				Err:           err,
			})
			continue
		}
		res.Applied = append(res.Applied, tgt.WorkspacePath)
	}
	return res, nil
}

// writeOneWorkspace reads (or creates) the workspace at path, applies the
// peacock palette derived from c, and writes it back. Used by both
// writeAnchorWorkspace (Case C) and writeFamilyPropagation (A2).
func writeOneWorkspace(path string, c color.Color, opts Options) error {
	ws, err := workspace.Read(path)
	if err != nil {
		return err
	}
	if ws == nil {
		ws = &workspace.Workspace{}
	}
	folderName := strings.TrimSuffix(filepath.Base(path), ".code-workspace")
	workspace.EnsureFolder(ws, "./"+folderName)
	palette := color.Palette(c, opts.Palette)
	workspace.ApplyPeacock(ws, c.Hex(), palette)
	return workspace.Write(path, ws)
}
```

Refactor existing `writeAnchorWorkspace` to use the helper:

```go
func writeAnchorWorkspace(intent *AnchorIntent, opts Options) error {
	return writeOneWorkspace(intent.WorkspacePath, intent.AnchorColor, opts)
}
```

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/runner/ -run TestWriteFamilyPropagation -v
```

Expected: PASS.

- [ ] **Step 5: Run all tests + lint**

```bash
task test && task lint
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: add writeFamilyPropagation + extract writeOneWorkspace

Main anchor write is hard-fail (no point propagating without an anchor).
Linked write failures accumulate into PropagateResult.Failed without
aborting the loop, so the user sees every failure in one run.

Refactor writeAnchorWorkspace to share the per-file write logic.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Wire propagation into `runner.Run`

Connect `propagateIntent` from `ResolveColor` to `writeFamilyPropagation`, populate the new `Result` fields, synthesize the warn via `formatPropagatedWarning`, and return `ErrPartialPropagation` on partial failure.

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Write failing integration tests**

Append to `internal/runner/runner_test.go`:

```go
func TestRun_A2_PropagatesToFamilyMembers(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	bug := filepath.Join(base, "myproj-bugfix")
	for _, p := range []string{mainPath, feat, bug} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mainWs := filepath.Join(base, "myproj.code-workspace")
	featWs := filepath.Join(base, "myproj-feat-x.code-workspace")
	bugWs := filepath.Join(base, "myproj-bugfix.code-workspace")
	for _, p := range []string{mainWs, featWs, bugWs} {
		if err := os.WriteFile(p, []byte(`{"settings":{"peacock.color":"#000000"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: bug, GitDir: filepath.Join(mainPath, ".git/worktrees/bugfix"), IsMain: false},
	}, nil)

	opts := Defaults()
	opts.TargetDir = mainPath
	opts.NoOpen = true
	opts.Force = true
	opts.ColorInput = "#aabbcc"

	res, err := New(&FakeOpener{}).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.EqualFold(res.ColorHex, "#aabbcc") {
		t.Errorf("ColorHex = %s, want #aabbcc", res.ColorHex)
	}
	if res.ColorSource != SourceWorktree {
		t.Errorf("ColorSource = %v, want SourceWorktree", res.ColorSource)
	}
	if len(res.PropagatedTo) != 2 {
		t.Errorf("PropagatedTo = %v, want 2", res.PropagatedTo)
	}
	// Verify both linked workspaces were written with derived (non-anchor) colors
	for _, p := range []string{featWs, bugWs} {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), `"peacock.color":"#000000"`) {
			t.Errorf("%s: peacock.color not updated; got %s", p, body)
		}
	}
}

func TestRun_A2_SkipsUncoloredLinked(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	hot := filepath.Join(base, "myproj-hotfix")
	for _, p := range []string{mainPath, feat, hot} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mainWs := filepath.Join(base, "myproj.code-workspace")
	featWs := filepath.Join(base, "myproj-feat-x.code-workspace")
	hotWs := filepath.Join(base, "myproj-hotfix.code-workspace")
	for _, p := range []string{mainWs, featWs} {
		if err := os.WriteFile(p, []byte(`{"settings":{"peacock.color":"#000000"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// hotfix has .code-workspace WITHOUT peacock keys
	if err := os.WriteFile(hotWs, []byte(`{"folders":[{"path":"./myproj-hotfix"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hotBefore, _ := os.ReadFile(hotWs)

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: hot, GitDir: filepath.Join(mainPath, ".git/worktrees/hotfix"), IsMain: false},
	}, nil)

	opts := Defaults()
	opts.TargetDir = mainPath
	opts.NoOpen = true
	opts.Force = true
	opts.ColorInput = "#aabbcc"

	res, err := New(&FakeOpener{}).Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.SkippedLinked) != 1 || res.SkippedLinked[0].WorkspacePath != hotWs {
		t.Errorf("SkippedLinked = %v, want hotfix", res.SkippedLinked)
	}
	hotAfter, _ := os.ReadFile(hotWs)
	if string(hotBefore) != string(hotAfter) {
		t.Errorf("hotfix .code-workspace was modified; expected untouched")
	}
}

func TestRun_A2_PropagationPartialFailure_ReturnsErrPartial(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	feat := filepath.Join(base, "myproj-feat-x")
	bugDir := filepath.Join(base, "ro")
	for _, p := range []string{mainPath, feat, bugDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bug := filepath.Join(bugDir, "myproj-bugfix")
	if err := os.MkdirAll(bug, 0o755); err != nil {
		t.Fatal(err)
	}
	mainWs := filepath.Join(base, "myproj.code-workspace")
	featWs := filepath.Join(base, "myproj-feat-x.code-workspace")
	bugWs := filepath.Join(bugDir, "myproj-bugfix.code-workspace")
	for _, p := range []string{mainWs, featWs, bugWs} {
		if err := os.WriteFile(p, []byte(`{"settings":{"peacock.color":"#000000"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Make bug's parent dir read-only so the linked write fails
	if err := os.Chmod(bugDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bugDir, 0o755) })

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: feat, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: bug, GitDir: filepath.Join(mainPath, ".git/worktrees/bugfix"), IsMain: false},
	}, nil)

	opts := Defaults()
	opts.TargetDir = mainPath
	opts.NoOpen = true
	opts.Force = true
	opts.ColorInput = "#aabbcc"

	res, err := New(&FakeOpener{}).Run(opts)
	if !errors.Is(err, ErrPartialPropagation) {
		t.Fatalf("err = %v, want ErrPartialPropagation", err)
	}
	if res == nil {
		t.Fatal("res = nil, want populated Result for warning rendering")
	}
	if len(res.FailedLinked) != 1 {
		t.Errorf("FailedLinked = %v, want 1", res.FailedLinked)
	}
	if len(res.PropagatedTo) != 1 {
		t.Errorf("PropagatedTo = %v, want 1 (feat succeeded)", res.PropagatedTo)
	}
}
```

If `runner_test.go` doesn't have `errors` import, add it.

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/runner/ -run TestRun_A2 -v
```

Expected: FAIL — propagation not wired.

- [ ] **Step 3: Wire propagation in `runner.Run`**

Find the section in `runner.Run` (around the existing `anchorIntent` handling):

```go
c, src, resolveWarns, anchorIntent, propagateIntent, err := ResolveColor(abs, opts.ColorInput, opts.Force, opts.Debug)
if err != nil {
	return nil, err
}
if anchorIntent != nil {
	if err := writeAnchorWorkspace(anchorIntent, opts); err != nil {
		return nil, fmt.Errorf("write main anchor workspace: %w", err)
	}
}
```

Add propagation handling immediately after, plus drop any temporary `_ = propagateIntent`:

```go
var (
	propagatedTo  []string
	skippedLinked []SkippedLinked
	failedLinked  []PropagateFailure
)
if propagateIntent != nil {
	pres, perr := writeFamilyPropagation(propagateIntent, opts)
	if perr != nil {
		return nil, perr
	}
	propagatedTo = pres.Applied
	skippedLinked = propagateIntent.Skipped
	failedLinked = pres.Failed
}
```

- [ ] **Step 4: Skip the local target write when propagation already wrote main**

When A2 fires and target is main, the propagation already wrote `<parent>/<dirname>.code-workspace`. The existing `runner.Run` code below also writes the target's workspace file, which would overwrite our propagation result with a fresh struct (potentially clobbering color settings). To avoid double-write or conflict, check `propagateIntent != nil` and skip the existing write block in that case.

Find the block in `runner.Run`:

```go
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
```

Wrap with a `propagateIntent == nil` guard:

```go
palette := color.Palette(c, opts.Palette)
colorHex := c.Hex()

if propagateIntent == nil {
	if ws == nil {
		ws = &workspace.Workspace{}
	}
	workspace.EnsureFolder(ws, "./"+folderName)
	workspace.ApplyPeacock(ws, colorHex, palette)
	if err := workspace.Write(wsPath, ws); err != nil {
		return nil, err
	}
}
```

(The propagation already wrote main with the anchor color via `writeFamilyPropagation`.)

- [ ] **Step 5: Add propagation warn synthesis + Result fields**

Continue in `runner.Run`. Find the section where warnings are appended:

```go
warnings := append([]string(nil), resolveWarns...)
if isGitRepo(parent) {
	warnings = append(warnings,
		fmt.Sprintf("parent directory %s is a git repository; workspace file may be committed", parent))
}
```

Add propagation warn before the final return, and synthesize the multi-line message:

```go
warnings := append([]string(nil), resolveWarns...)
if isGitRepo(parent) {
	warnings = append(warnings,
		fmt.Sprintf("parent directory %s is a git repository; workspace file may be committed", parent))
}
if propagateIntent != nil {
	warnings = append(warnings, formatPropagatedWarning(propagateIntent, failedLinked))
}
```

Then update the existing final return to populate the new Result fields and decide on `ErrPartialPropagation`:

```go
if !opts.NoOpen {
	if err := r.Opener.Open(wsPath); err != nil {
		// ... existing handling unchanged
	}
}

result := &Result{
	WorkspaceFile:   wsPath,
	ColorHex:        colorHex,
	ColorSource:     src,
	SettingsCleaned: cleaned,
	Warnings:        warnings,
	PropagatedTo:    propagatedTo,
	SkippedLinked:   skippedLinked,
	FailedLinked:    failedLinked,
}
if len(failedLinked) > 0 {
	return result, ErrPartialPropagation
}
return result, nil
```

(Replace the existing final `return &Result{...}, nil` with this.)

- [ ] **Step 6: Run tests**

```bash
go test ./internal/runner/ -run TestRun_A2 -v
```

Expected: all three PASS.

- [ ] **Step 7: Run all tests + lint**

```bash
task test && task lint
```

Note: any test relying on the workspace file being rewritten on every Run may now break if it triggers A2 and expects local-write behavior; this is intentional (A2 writes happen via writeFamilyPropagation).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
runner: wire A2 propagation through Run + ErrPartialPropagation

Run now executes writeFamilyPropagation when ResolveColor returns a
PropagateIntent, populates Result.PropagatedTo/SkippedLinked/FailedLinked,
synthesizes the multi-line warn (including failures), and returns
ErrPartialPropagation when any linked write failed (caller renders
Result first, then maps the sentinel to exit 1).

When A2 fires, the local-target-write block is skipped because
writeFamilyPropagation already wrote main's .code-workspace with the
anchor color.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: CLI exit code wiring

Update `cmd/ccws/root.go` so that `ErrPartialPropagation` triggers exit 1 *after* rendering the populated Result.

**Files:**
- Modify: `cmd/ccws/root.go`

- [ ] **Step 1: Update RunE in root.go**

Find the existing RunE block:

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
	opts.Debug = flagDebug
	res, err := runner.New(nil).Run(opts)
	if err != nil {
		return err
	}
	renderWarnings(tui.NewStderr(), res.Warnings)
	if res.Preconfigured {
		renderPreconfigured(tui.NewStderr(), res)
	} else {
		renderSuccess(tui.NewStdout(), res, sourceLabel(res.ColorSource))
	}
	return nil
},
```

Replace with:

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
	opts.Debug = flagDebug
	res, err := runner.New(nil).Run(opts)
	// ErrPartialPropagation is recoverable for rendering — the result is populated.
	if err != nil && !errors.Is(err, runner.ErrPartialPropagation) {
		return err
	}
	renderWarnings(tui.NewStderr(), res.Warnings)
	if res.Preconfigured {
		renderPreconfigured(tui.NewStderr(), res)
	} else {
		renderSuccess(tui.NewStdout(), res, sourceLabel(res.ColorSource))
	}
	return err // propagates ErrPartialPropagation to errToExit → exit 1
},
```

`errors` is already imported at top of root.go (used by `errors.As`).

- [ ] **Step 2: Verify exit code mapping**

Look at `errToExit` (top of root.go):

```go
func errToExit(err error) int {
	if err == nil {
		return 0
	}
	var ge *runner.GuardError
	if errors.As(err, &ge) {
		return 2
	}
	if errors.Is(err, os.ErrPermission) {
		return 3
	}
	return 1
}
```

`ErrPartialPropagation` is not a `*GuardError` and not `os.ErrPermission`, so it falls through to the default `return 1`. No code change needed.

- [ ] **Step 3: Build and run lint**

```bash
go build ./... && task lint
```

Expected: clean.

- [ ] **Step 4: Run integration test from Task 8 to confirm exit-1 behavior**

Run only as a sanity check:

```bash
go test ./internal/runner/ -run TestRun_A2_PropagationPartialFailure_ReturnsErrPartial -v
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

```bash
task test
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
ccws: render result before returning ErrPartialPropagation

A2 partial propagation populates Result and surfaces ErrPartialPropagation.
The CLI must render warnings/success normally (so the user sees what
succeeded and what failed) before letting the error propagate to errToExit
for exit code 1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Documentation updates

Update README and CLAUDE.md to reflect the new `--force` semantics.

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update README `--force` section**

Find the `--force` description (around line 42 and line 59 in current README).

Replace the current line 42 paragraph:

> Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file). **If the workspace file already contains peacock keys, ccws skips the write, prints a warning, and just opens it. Pass `--force` to overwrite.**

with:

> Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file). **If the workspace file already contains peacock keys, ccws skips the write, prints a warning, and just opens it. Pass `--force` to overwrite.** When run on a git **main** worktree with `--force`, ccws also propagates the anchor color to every linked worktree's `.code-workspace` that already has peacock keys (linked workspaces without peacock keys are skipped). Linked workspaces that fail to write are reported and exit code 1 is returned.

- [ ] **Step 2: Update CLAUDE.md Safety guards section**

Find the "Safety guards" section in `CLAUDE.md` (search for "Guard 1 (soft)") and append a sub-bullet under that section, after the Guard 2 description:

```markdown
- **A2 propagation** — `ccws --force` on a git **main** worktree of a multi-worktree repo writes not just `<parent>/<dirname>.code-workspace` but also every linked worktree's `.code-workspace` that has peacock keys. Linked workspaces without peacock keys are skipped (gating: a worktree "joins" the family by being ccws'd directly). Linked write failures accumulate; if any fail, the runner returns `ErrPartialPropagation` (exit 1). Unlike Case C (anchor auto-create), A2 modifies multiple non-target files; renderer emits a multi-line warn listing applied/failed/skipped paths.
```

- [ ] **Step 3: Verify doc changes**

```bash
git diff README.md CLAUDE.md
```

Sanity-check that the wording is correct and the sections fit.

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: --force on main now propagates anchor color across family

README documents the new behavior; CLAUDE.md notes that A2 writes to
multiple non-target .code-workspace files and may exit 1 on partial
propagation failures.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage**

| Spec section | Task |
|---|---|
| §2 A1 (single-worktree main fall-through) | Task 3 |
| §2 A2 (multi-worktree main propagate) | Tasks 4, 5, 7, 8 |
| §2 A3 (linked target unchanged) | Task 5 (preserved during refactor) |
| §3 priority chain (no behavior change for non-A2 cases) | Tasks 1, 5 (signature preserves chain) |
| §4 rendering (header / applied / failed / skipped / no-family) | Task 6 |
| §5 package structure (modify-only) | All |
| §6.1 PropagateIntent / SkippedLinked / PropagateFailure types | Task 2 |
| §6.2 ResolveColor signature | Tasks 1, 5 |
| §6.3 resolveFromWorktree branches | Tasks 3, 5 |
| §6.4 --color flag in A2 anchor | Task 5 (Step 3, ResolveColor restructure) |
| §6.5 writeFamilyPropagation | Task 7 |
| §6.6 Result fields + Exit code | Tasks 2, 8, 9 |
| §7 warn synthesis at runner.Run | Task 8 |
| §8 test strategy | Tasks 3-8 |
| §9 backward compat (Case D recovery) | Implicit in Task 5 (A2 fires when force=true even if main "had" Case D state — but Case D check happens AFTER A2 in the switch order) |
| §10 failure handling | Tasks 4 (parse → Skipped), 7 (write → Failed) |
| §11 idempotency | Inherent in the design |
| §12 doc updates | Task 10 |

**2. Placeholder scan:** No TBDs, TODOs, or "implement later" steps. Every code block has actual code.

**3. Type consistency:**
- `PropagateIntent.AnchorPath`, `AnchorColor`, `Targets`, `Skipped` — used consistently across tasks.
- `SkippedLinked.WorkspacePath`, `Reason` — consistent.
- `PropagateFailure.WorkspacePath`, `Err` — consistent.
- `PropagateResult.Applied`, `Failed` — consistent.
- `Result.PropagatedTo` (`[]string`), `Result.SkippedLinked` (`[]SkippedLinked`), `Result.FailedLinked` (`[]PropagateFailure`) — consistent.
- `ErrPartialPropagation` referenced in Tasks 2 (declare), 8 (return), 9 (errors.Is) — consistent.

**4. Spec gap check:**
- §9 "Case D recovery" relies on A2 firing in T==M when main has color and force=true. The design only enters A2 when `mainColor != nil`. In Case D, main has NO color — so A2 doesn't fire. The "auto recovery" claim in spec §9 only applies if main acquires a color first, then A2 propagates. Correction: this is a non-issue; spec §9 says "main + --force overwrites stale linked colors". For that to happen via A2, the user would first need to set main's color (e.g., `ccws --color X main` would go through the SourceFlag path and write main, then a subsequent `ccws --force main` would hit A2). This is a two-step recovery, not single-step. The spec wording oversells it slightly but the implementation is correct.

  No additional task needed; the wording in CLAUDE.md/README does not promise single-step recovery.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-01-worktree-main-force-propagation.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
