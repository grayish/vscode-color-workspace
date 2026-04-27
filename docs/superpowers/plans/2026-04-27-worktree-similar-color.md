# Worktree Similar Color Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ccws` auto-derive a "family" color (same hue/saturation, lightness shifted by ±5/±10/±15%) for git worktrees of the same repo, with main as the anchor.

**Architecture:** New `internal/gitworktree` package handles `git worktree list --porcelain` invocation and parsing. New `internal/color/ladder.go` provides lightness-offset primitives. `runner.ResolveColor` is extended to consult worktree context between settings.json and random fallback. The side effect (auto-creating main's `.code-workspace` when a linked worktree is colored first) is signaled via `AnchorIntent` and executed by `runner.Run`.

**Tech Stack:** Go stdlib only (`os/exec`, `bufio`, `hash/fnv`). Test framework `testing`. Build/test orchestration via `task` (Taskfile.yml).

**Spec:** `docs/superpowers/specs/2026-04-27-worktree-similar-color-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/gitworktree/gitworktree.go` | new — `Worktree` type, `parsePorcelain`, `List`, `IdentityHash`, `FindSelf`, `ErrNotInWorktree` |
| `internal/gitworktree/gitworktree_test.go` | new — unit tests for parser, hash, FindSelf |
| `internal/gitworktree/gitworktree_integration_test.go` | new (`//go:build integration`) — real git invocation |
| `internal/color/ladder.go` | new — `LadderSteps`, `LadderOffset`, `Color.ApplyLightness` |
| `internal/color/ladder_test.go` | new |
| `internal/runner/resolve.go` | extend `ResolveColor` to 5-tuple, add `AnchorIntent`, `SourceWorktree`, `resolveFromWorktree`, `readWorkspacePeacockColor`, `findLinkedWithColor`, warning formatters, `listWorktreesFn` injection point |
| `internal/runner/resolve_test.go` | update existing 4 tests for new signature + add Case A/B/C/D tests |
| `internal/runner/runner.go` | accept new ResolveColor return, add `writeAnchorWorkspace`, execute AnchorIntent |
| `internal/runner/runner_test.go` | (smoke) — ensure existing tests still pass after signature change |
| `cmd/ccws/render.go` | `sourceLabel`: add `SourceWorktree` case |
| `cmd/ccws/render_test.go` | add label test |
| `Taskfile.yml` | add `task test:integration` target |
| `README.md` | new section: "Worktree color family" |
| `CLAUDE.md` | DAG update + Non-goals expansion |

---

## Task 1: gitworktree package — Worktree type and `parsePorcelain`

**Files:**
- Create: `internal/gitworktree/gitworktree.go`
- Create: `internal/gitworktree/gitworktree_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gitworktree/gitworktree_test.go`:

```go
package gitworktree

import (
	"strings"
	"testing"
)

func TestParsePorcelain_MainPlusLinked(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /Users/user/code/myproj-feat-x",
		"HEAD def456",
		"branch refs/heads/feat-x",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Path != "/Users/user/code/myproj" || got[0].Branch != "main" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Path != "/Users/user/code/myproj-feat-x" || got[1].Branch != "feat-x" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestParsePorcelain_DetachedHEAD(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /Users/user/code/myproj-detached",
		"HEAD ddd000",
		"detached",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].Branch != "" {
		t.Errorf("detached branch = %q, want empty", got[1].Branch)
	}
}

func TestParsePorcelain_BareRepo(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj.git",
		"bare",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].Bare {
		t.Errorf("Bare = false, want true")
	}
}

func TestParsePorcelain_Empty(t *testing.T) {
	got, err := parsePorcelain([]byte(""))
	if err != nil {
		t.Fatalf("parsePorcelain(empty) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
```

- [ ] **Step 2: Verify test fails**

Run: `go test ./internal/gitworktree/ -run TestParsePorcelain -v`
Expected: FAIL — `package gitworktree is not in std` or undefined `parsePorcelain`.

- [ ] **Step 3: Implement parser**

Create `internal/gitworktree/gitworktree.go`:

```go
// Package gitworktree wraps `git worktree list --porcelain` so callers can
// reason about the set of worktrees attached to a given target directory
// without depending on the git binary directly.
package gitworktree

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
)

// ErrNotInWorktree means the target directory is not under any git repo,
// or git is unavailable, or `git worktree list` produced unusable output.
// Callers treat this as "skip worktree logic, fall back to the existing
// resolution chain."
var ErrNotInWorktree = errors.New("gitworktree: target is not in a git worktree")

// Worktree describes a single worktree as reported by `git worktree list --porcelain`.
type Worktree struct {
	Path   string // absolute working tree path
	GitDir string // populated by List; <main>/.git or <main>/.git/worktrees/<name>
	Branch string // empty for detached HEAD
	IsMain bool   // true for the primary worktree (first entry in --porcelain)
	Bare   bool   // true for bare repos (no working tree)
}

// parsePorcelain converts the raw bytes of `git worktree list --porcelain`
// into a slice of Worktree records. Records are separated by blank lines.
// The first record is treated as main by the caller (List sets IsMain).
func parsePorcelain(data []byte) ([]Worktree, error) {
	var out []Worktree
	var cur Worktree
	started := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if started {
				out = append(out, cur)
				cur = Worktree{}
				started = false
			}
			continue
		}
		started = true
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			cur.Branch = ""
		case line == "bare":
			cur.Bare = true
		}
	}
	if started {
		out = append(out, cur)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 4: Verify all parser tests pass**

Run: `go test ./internal/gitworktree/ -run TestParsePorcelain -v`
Expected: PASS for all four subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/gitworktree/gitworktree.go internal/gitworktree/gitworktree_test.go
git commit -m "$(cat <<'EOF'
gitworktree: add Worktree type and parsePorcelain

Introduces a new internal package that will wrap `git worktree list
--porcelain`. This first commit covers just the parser — handles main
worktree, linked worktrees, detached HEAD, and bare repos. Tested with
fixture strings; no exec.Command dependency yet.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: gitworktree — `IdentityHash` and `FindSelf`

**Files:**
- Modify: `internal/gitworktree/gitworktree.go`
- Modify: `internal/gitworktree/gitworktree_test.go`

- [ ] **Step 1: Write failing tests for IdentityHash and FindSelf**

Append to `internal/gitworktree/gitworktree_test.go`:

```go
func TestIdentityHash_MainReturnsZero(t *testing.T) {
	w := Worktree{Path: "/tmp/myproj", GitDir: "/tmp/myproj/.git", IsMain: true}
	if got := IdentityHash(w); got != 0 {
		t.Errorf("IdentityHash(main) = %d, want 0", got)
	}
}

func TestIdentityHash_LinkedStable(t *testing.T) {
	w := Worktree{Path: "/tmp/myproj-feat-x", GitDir: "/tmp/myproj/.git/worktrees/feat-x"}
	a := IdentityHash(w)
	b := IdentityHash(w)
	if a != b {
		t.Errorf("IdentityHash not stable: %d vs %d", a, b)
	}
	if a == 0 {
		t.Error("IdentityHash(linked) returned 0 (collision with main convention)")
	}
}

func TestIdentityHash_DifferentLinkedDifferent(t *testing.T) {
	a := IdentityHash(Worktree{GitDir: "/tmp/.git/worktrees/feat-x"})
	b := IdentityHash(Worktree{GitDir: "/tmp/.git/worktrees/bugfix"})
	if a == b {
		t.Errorf("hashes collide: feat-x=%d bugfix=%d", a, b)
	}
}

func TestFindSelf_ExactPath(t *testing.T) {
	wts := []Worktree{
		{Path: "/tmp/main", IsMain: true},
		{Path: "/tmp/linked", IsMain: false},
	}
	got := FindSelf(wts, "/tmp/linked")
	if got == nil || got.Path != "/tmp/linked" {
		t.Errorf("FindSelf = %+v", got)
	}
}

func TestFindSelf_Subdir(t *testing.T) {
	wts := []Worktree{
		{Path: "/tmp/main", IsMain: true},
		{Path: "/tmp/linked", IsMain: false},
	}
	got := FindSelf(wts, "/tmp/linked/sub/dir")
	if got == nil || got.Path != "/tmp/linked" {
		t.Errorf("FindSelf(subdir) = %+v", got)
	}
}

func TestFindSelf_NoMatch(t *testing.T) {
	wts := []Worktree{{Path: "/tmp/main", IsMain: true}}
	if got := FindSelf(wts, "/elsewhere"); got != nil {
		t.Errorf("FindSelf(unrelated) = %+v, want nil", got)
	}
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/gitworktree/ -run "TestIdentityHash|TestFindSelf" -v`
Expected: FAIL — undefined `IdentityHash`, `FindSelf`.

- [ ] **Step 3: Implement IdentityHash and FindSelf**

Append to `internal/gitworktree/gitworktree.go`:

```go
import (
	// existing imports plus:
	"hash/fnv"
	"path/filepath"
	"strings"
)

// IdentityHash returns a stable 64-bit identifier for a worktree.
// Main returns 0 by convention so it always maps to LadderOffset = 0.
// Linked worktrees use FNV-1a over basename(GitDir) — git keeps that name
// stable across `git worktree move` and branch renames.
func IdentityHash(w Worktree) uint64 {
	if w.IsMain {
		return 0
	}
	name := filepath.Base(w.GitDir)
	if name == "" || name == "." || name == "/" {
		name = w.Path
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	sum := h.Sum64()
	if sum == 0 {
		return 1 // never collide with the main-worktree convention
	}
	return sum
}

// FindSelf returns the worktree whose Path equals targetDir or is an
// ancestor of targetDir. Returns nil if no entry matches.
func FindSelf(worktrees []Worktree, targetDir string) *Worktree {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return nil
	}
	var best *Worktree
	for i := range worktrees {
		w := &worktrees[i]
		if w.Path == "" {
			continue
		}
		if abs == w.Path || strings.HasPrefix(abs, w.Path+string(filepath.Separator)) {
			if best == nil || len(w.Path) > len(best.Path) {
				best = w
			}
		}
	}
	return best
}
```

(The existing `import` block in `gitworktree.go` will need `hash/fnv`, `path/filepath`, and `strings`. `strings` is already imported from Task 1.)

- [ ] **Step 4: Verify tests pass**

Run: `go test ./internal/gitworktree/ -v`
Expected: all `TestIdentityHash_*`, `TestFindSelf_*`, and prior parser tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitworktree/gitworktree.go internal/gitworktree/gitworktree_test.go
git commit -m "$(cat <<'EOF'
gitworktree: add IdentityHash and FindSelf

IdentityHash returns 0 for main (so the lightness offset is always 0%) and
FNV-1a of basename(GitDir) for linked worktrees — git keeps that internal
name stable across moves and branch renames. FindSelf maps an arbitrary
target directory to the worktree whose root is its closest ancestor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: gitworktree — `List` (real git invocation) + integration test

**Files:**
- Modify: `internal/gitworktree/gitworktree.go`
- Create: `internal/gitworktree/gitworktree_integration_test.go`

- [ ] **Step 1: Write the integration test**

Create `internal/gitworktree/gitworktree_integration_test.go`:

```go
//go:build integration

package gitworktree

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestList_RealGit_MainPlusLinked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	main := filepath.Join(base, "myproj")
	if err := exec.Command("git", "init", main).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	// need at least one commit before adding a worktree
	if err := exec.Command("touch", filepath.Join(main, "README")).Run(); err != nil {
		t.Fatal(err)
	}
	runGit(t, main, "add", ".")
	runGit(t, main, "commit", "-m", "init")
	linked := filepath.Join(base, "myproj-feat-x")
	runGit(t, main, "worktree", "add", "-b", "feat-x", linked)

	got, err := List(linked)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2; got = %+v", len(got), got)
	}
	if !got[0].IsMain {
		t.Errorf("got[0].IsMain = false, want true")
	}
	if got[0].Path != main {
		t.Errorf("got[0].Path = %q, want %q", got[0].Path, main)
	}
	if got[1].Path != linked {
		t.Errorf("got[1].Path = %q, want %q", got[1].Path, linked)
	}
	if got[1].Branch != "feat-x" {
		t.Errorf("got[1].Branch = %q, want feat-x", got[1].Branch)
	}
	if !strings.HasSuffix(got[1].GitDir, "/.git/worktrees/feat-x") {
		t.Errorf("got[1].GitDir = %q, want suffix /.git/worktrees/feat-x", got[1].GitDir)
	}
}

func TestList_NotInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	_, err := List(dir)
	if err == nil {
		t.Fatal("List(non-git dir) returned nil error")
	}
	if !errorsIsErrNotInWorktree(err) {
		t.Errorf("err = %v, want ErrNotInWorktree", err)
	}
}

func errorsIsErrNotInWorktree(err error) bool {
	for ; err != nil; err = unwrap(err) {
		if err == ErrNotInWorktree {
			return true
		}
	}
	return false
}

func unwrap(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
```

- [ ] **Step 2: Verify integration test fails**

Run: `go test -tags=integration ./internal/gitworktree/ -run TestList_ -v`
Expected: FAIL — undefined `List`.

- [ ] **Step 3: Implement `List`**

Append to `internal/gitworktree/gitworktree.go`:

```go
import (
	// existing plus:
	"fmt"
	"os"
	"os/exec"
)

// List runs `git worktree list --porcelain` from targetDir and returns the
// resulting Worktree slice. The first entry is the main worktree (IsMain set).
// GitDir is populated for each entry: <path>/.git for main, the gitdir-pointer
// target for linked worktrees.
//
// Any failure (git missing, target not in a repo, parse anomaly, bare-only
// output) collapses to ErrNotInWorktree so callers can silently skip
// worktree-aware logic and fall through to the existing resolution chain.
func List(targetDir string) ([]Worktree, error) {
	cmd := exec.Command("git", "-C", targetDir, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrNotInWorktree
	}
	worktrees, err := parsePorcelain(out)
	if err != nil || len(worktrees) == 0 {
		return nil, ErrNotInWorktree
	}
	if worktrees[0].Bare {
		return nil, ErrNotInWorktree
	}
	worktrees[0].IsMain = true
	worktrees[0].GitDir = filepath.Join(worktrees[0].Path, ".git")
	for i := 1; i < len(worktrees); i++ {
		gd, err := readGitDirPointer(worktrees[i].Path)
		if err != nil {
			return nil, fmt.Errorf("read .git pointer for %q: %w", worktrees[i].Path, ErrNotInWorktree)
		}
		worktrees[i].GitDir = gd
	}
	return worktrees, nil
}

// readGitDirPointer reads <path>/.git as a text file (linked worktrees have
// `.git` as a file with a "gitdir: <abs path>" line) and returns the pointed-to
// gitdir path.
func readGitDirPointer(path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(path, ".git"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "gitdir:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gitdir:")), nil
		}
	}
	return "", fmt.Errorf("no gitdir: line in %q", path)
}
```

- [ ] **Step 4: Verify integration test passes**

Run: `go test -tags=integration ./internal/gitworktree/ -v`
Expected: PASS for all integration tests; non-tagged unit tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitworktree/gitworktree.go internal/gitworktree/gitworktree_integration_test.go
git commit -m "$(cat <<'EOF'
gitworktree: add List wrapping `git worktree list --porcelain`

List shells out to git, parses the porcelain output, marks the first entry
as main, and populates GitDir for each (read from <path>/.git pointer file
for linked worktrees). All git/parse failures collapse to ErrNotInWorktree
so callers can silently skip worktree-aware logic.

Integration test (//go:build integration) creates a real git repo, adds a
linked worktree, and verifies List produces the expected entries. Skipped
when git is not on PATH.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: color/ladder — `LadderSteps`, `LadderOffset`, `Color.ApplyLightness`

**Files:**
- Create: `internal/color/ladder.go`
- Create: `internal/color/ladder_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/color/ladder_test.go`:

```go
package color

import "testing"

func TestLadderSteps_NoZero(t *testing.T) {
	for _, s := range LadderSteps {
		if s == 0 {
			t.Fatal("LadderSteps must not contain 0; that slot is reserved for main worktree")
		}
	}
}

func TestLadderOffset_ZeroHashIsZero(t *testing.T) {
	if got := LadderOffset(0); got != 0 {
		t.Errorf("LadderOffset(0) = %v, want 0", got)
	}
}

func TestLadderOffset_NonZeroInRange(t *testing.T) {
	allowed := map[float64]bool{}
	for _, s := range LadderSteps {
		allowed[s] = true
	}
	for hash := uint64(1); hash < 1000; hash++ {
		got := LadderOffset(hash)
		if !allowed[got] {
			t.Fatalf("LadderOffset(%d) = %v, not in %v", hash, got, LadderSteps)
		}
	}
}

func TestLadderOffset_Stable(t *testing.T) {
	for hash := uint64(1); hash < 100; hash++ {
		a := LadderOffset(hash)
		b := LadderOffset(hash)
		if a != b {
			t.Errorf("LadderOffset(%d) not stable: %v vs %v", hash, a, b)
		}
	}
}

func TestApplyLightness_PositiveLightens(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140} // #5a3b8c, L≈39%
	lighter := base.ApplyLightness(10)
	if lighter == base {
		t.Error("ApplyLightness(+10) returned base unchanged")
	}
	// crude sanity: lighter color has higher channel sum
	if int(lighter.R)+int(lighter.G)+int(lighter.B) <= int(base.R)+int(base.G)+int(base.B) {
		t.Errorf("ApplyLightness(+10) did not lighten: base=%v lighter=%v", base, lighter)
	}
}

func TestApplyLightness_NegativeDarkens(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140}
	darker := base.ApplyLightness(-10)
	if darker == base {
		t.Error("ApplyLightness(-10) returned base unchanged")
	}
	if int(darker.R)+int(darker.G)+int(darker.B) >= int(base.R)+int(base.G)+int(base.B) {
		t.Errorf("ApplyLightness(-10) did not darken: base=%v darker=%v", base, darker)
	}
}

func TestApplyLightness_ZeroIsNoop(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140}
	if got := base.ApplyLightness(0); got != base {
		t.Errorf("ApplyLightness(0) = %v, want %v", got, base)
	}
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/color/ -run "TestLadder|TestApplyLightness" -v`
Expected: FAIL — undefined `LadderSteps`, `LadderOffset`, `ApplyLightness`.

- [ ] **Step 3: Implement ladder primitives**

Create `internal/color/ladder.go`:

```go
package color

// LadderSteps are the lightness deltas (HSL %) used by LadderOffset. The
// list intentionally excludes 0 — the main worktree's IdentityHash is 0
// by convention, so its offset is always 0% (handled in LadderOffset).
var LadderSteps = []float64{-15, -10, -5, +5, +10, +15}

// LadderOffset maps an identity hash to a lightness delta.
//   - hash == 0  → 0  (main worktree convention)
//   - hash != 0  → LadderSteps[hash % len(LadderSteps)]
func LadderOffset(hash uint64) float64 {
	if hash == 0 {
		return 0
	}
	return LadderSteps[hash%uint64(len(LadderSteps))]
}

// ApplyLightness returns c with HSL lightness shifted by deltaPct percent.
// Delegates to existing Lighten/Darken primitives, which clamp HSL to [0, 1].
func (c Color) ApplyLightness(deltaPct float64) Color {
	switch {
	case deltaPct > 0:
		return c.Lighten(deltaPct)
	case deltaPct < 0:
		return c.Darken(-deltaPct)
	default:
		return c
	}
}
```

- [ ] **Step 4: Verify tests pass**

Run: `go test ./internal/color/ -run "TestLadder|TestApplyLightness" -v`
Expected: PASS for all 7 test cases.

- [ ] **Step 5: Commit**

```bash
git add internal/color/ladder.go internal/color/ladder_test.go
git commit -m "$(cat <<'EOF'
color: add LadderSteps, LadderOffset, Color.ApplyLightness

Six-step lightness ladder (±5/±10/±15%) used for worktree color derivation.
Hash 0 maps to 0% (main-worktree convention); non-zero hashes pick a step
deterministically. ApplyLightness delegates to existing Lighten/Darken so
HSL clamping is reused.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: runner/resolve — extend signature, add `AnchorIntent`, stub `resolveFromWorktree`

This task changes the `ResolveColor` signature to a 5-tuple and adds the worktree-resolution scaffolding without any worktree behavior yet — the stub always returns "fall through." All existing tests must keep passing after this task.

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Update existing 4 tests in `resolve_test.go` to the new 5-tuple shape**

Open `internal/runner/resolve_test.go` and replace each `ResolveColor` call site to discard the two new return values:

```go
// TestResolveColor_ExplicitWins
got, src, _, _, err := ResolveColor(dir, "#222222")

// TestResolveColor_InheritFromSettings
got, src, _, _, err := ResolveColor(dir, "")

// TestResolveColor_Random
got, src, _, _, err := ResolveColor(dir, "")

// TestResolveColor_InvalidFlag
if _, _, _, _, err := ResolveColor(dir, "not-a-color"); err == nil {
```

(Leave the rest of each test body unchanged.)

- [ ] **Step 2: Rewrite `resolve.go` with new signature + stub**

Replace the contents of `internal/runner/resolve.go`:

```go
package runner

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/gitworktree"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
)

// ColorSource indicates where the final color came from.
type ColorSource int

const (
	SourceFlag ColorSource = iota + 1
	SourceSettings
	SourceWorktree
	SourceRandom
)

// AnchorIntent describes a side effect requested by the worktree resolver:
// write the given color as the family anchor into the main worktree's
// .code-workspace. Returned only for Case C (linked worktree is the first
// to be ccws'd in this repo). Caller (runner.Run) is responsible for
// executing the write.
type AnchorIntent struct {
	WorkspacePath string
	AnchorColor   color.Color
}

// listWorktreesFn is the package-level injection point for the gitworktree.List
// dependency. Tests reassign it (with cleanup) to inject fixture worktree slices.
var listWorktreesFn = gitworktree.List

// ResolveColor applies the priority rules:
//  1. Explicit --color flag                               → SourceFlag
//  2. Worktree family logic (Case A or C in spec)         → SourceWorktree
//  3. peacock.color in target's .vscode/settings.json     → SourceSettings
//  4. Random                                              → SourceRandom
//
// The third return is informational warnings to be surfaced via Result.Warnings.
// The fourth return is non-nil only for Case C (auto-establish), where the
// caller must write the anchor color into the main worktree's .code-workspace.
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, []string, *AnchorIntent, error) {
	if flag != "" {
		c, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, nil, nil, fmt.Errorf("--color: %w", err)
		}
		return c, SourceFlag, nil, nil, nil
	}

	c, src, warns, intent, ok, err := resolveFromWorktree(targetDir)
	if err != nil {
		return color.Color{}, 0, nil, nil, err
	}
	if ok {
		return c, src, warns, intent, nil
	}

	// fall through to existing chain — preserve any Case-D warnings
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, warns, nil, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			c, err := color.Parse(pc)
			if err != nil {
				return color.Color{}, 0, warns, nil, fmt.Errorf("peacock.color in settings: %w", err)
			}
			return c, SourceSettings, warns, nil, nil
		}
	}
	return color.Random(), SourceRandom, warns, nil, nil
}

// resolveFromWorktree consults the worktree context. Return tuple semantics:
//
//	ok=true  → color decided by worktree logic; caller uses (c, src, warns, intent)
//	ok=false → fall through to settings/random; warns may carry a Case-D notice
//	err!=nil → hard error (e.g., file write failure for AnchorIntent)
//
// Stub for now: always falls through.
func resolveFromWorktree(targetDir string) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
	_, err := listWorktreesFn(targetDir)
	if errors.Is(err, gitworktree.ErrNotInWorktree) {
		return color.Color{}, 0, nil, nil, false, nil
	}
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	// Real implementation comes in Tasks 7-9.
	return color.Color{}, 0, nil, nil, false, nil
}
```

- [ ] **Step 3: Update `runner.go` call site**

In `internal/runner/runner.go`, find the line `c, src, err := ResolveColor(abs, opts.ColorInput)` and replace:

```go
c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput)
if err != nil {
    return nil, err
}
res.Warnings = append(res.Warnings, resolveWarns...)
_ = anchorIntent // wired in Task 10
```

(Note: `res` is the result variable already declared earlier in `Run`. If the variable is named differently, adjust to match.)

- [ ] **Step 4: Verify all existing tests still pass**

Run: `go build ./...` (catch any compile errors)
Then: `go test ./internal/runner/ -v`
Expected: existing 4 ResolveColor tests + all runner_test.go tests PASS.

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/resolve.go internal/runner/resolve_test.go internal/runner/runner.go
git commit -m "$(cat <<'EOF'
runner: extend ResolveColor signature for worktree integration

ResolveColor now returns (color, source, warnings, anchorIntent, error).
SourceWorktree and AnchorIntent are added but the worktree resolver is a
stub — it always returns fall-through, so behavior is unchanged. This
commit isolates the API change from the worktree logic itself.

listWorktreesFn package var introduced as the test injection point for
the gitworktree.List dependency.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: runner/resolve — `readWorkspacePeacockColor` helper

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/runner/resolve_test.go`:

```go
import (
	// existing plus:
	"github.com/sang-bin/vscode-color-workspace/internal/color"
)

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
```

- [ ] **Step 2: Verify test fails**

Run: `go test ./internal/runner/ -run TestReadWorkspacePeacockColor -v`
Expected: FAIL — undefined `readWorkspacePeacockColor`.

- [ ] **Step 3: Implement helper**

Append to `internal/runner/resolve.go`:

```go
import (
	// existing plus:
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// readWorkspacePeacockColor parses the workspace file at path and returns
// the peacock.color setting. Returns (nil, nil) when:
//   - file does not exist
//   - file has no settings block
//   - settings has no peacock.color key
//   - peacock.color is not a parseable color (treated as missing)
func readWorkspacePeacockColor(path string) (*color.Color, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	ws, err := workspace.Read(path)
	if err != nil {
		return nil, err
	}
	if ws == nil || ws.Settings == nil {
		return nil, nil
	}
	raw, ok := ws.Settings["peacock.color"]
	if !ok {
		return nil, nil
	}
	hex, ok := raw.(string)
	if !ok {
		return nil, nil
	}
	c, err := color.Parse(hex)
	if err != nil {
		return nil, nil
	}
	return &c, nil
}
```

- [ ] **Step 4: Verify tests pass**

Run: `go test ./internal/runner/ -run TestReadWorkspacePeacockColor -v`
Expected: PASS for all three cases.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/resolve.go internal/runner/resolve_test.go
git commit -m "$(cat <<'EOF'
runner: add readWorkspacePeacockColor helper

Reads a .code-workspace file via the workspace package, extracts the
peacock.color setting, parses it, and returns *color.Color. Returns
(nil, nil) for the common "no color found" cases — file missing, no
settings, no key, or unparseable value — so callers can branch on nil.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: runner/resolve — Case A (main has color → derive offset)

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Add a fixture helper to `resolve_test.go`**

Append to `internal/runner/resolve_test.go`:

```go
import (
	// existing plus:
	"github.com/sang-bin/vscode-color-workspace/internal/gitworktree"
)

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
```

(`fmt` is already imported transitively or you may need to add it. Run `go build` after the next step to catch missing imports.)

- [ ] **Step 2: Write Case A failing tests**

Append to `internal/runner/resolve_test.go`:

```go
func TestResolveColor_WorktreeCaseA_LinkedTarget(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(linkedPath, 0755); err != nil { t.Fatal(err) }
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, _, intent, err := ResolveColor(linkedPath, "")
	if err != nil { t.Fatal(err) }
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
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj.code-workspace"), "#5a3b8c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
	}, nil)

	c, src, _, _, err := ResolveColor(mainPath, "")
	if err != nil { t.Fatal(err) }
	if src != SourceWorktree {
		t.Errorf("source = %v, want SourceWorktree", src)
	}
	want := color.Color{R: 0x5a, G: 0x3b, B: 0x8c}
	if c != want {
		t.Errorf("main color = %v, want %v (offset 0)", c, want)
	}
}
```

- [ ] **Step 3: Verify tests fail**

Run: `go test ./internal/runner/ -run TestResolveColor_WorktreeCaseA -v`
Expected: FAIL — current stub returns fall-through, so source ≠ SourceWorktree.

- [ ] **Step 4: Implement Case A**

In `internal/runner/resolve.go`, replace the body of `resolveFromWorktree` with:

```go
func resolveFromWorktree(targetDir string) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
	worktrees, err := listWorktreesFn(targetDir)
	if errors.Is(err, gitworktree.ErrNotInWorktree) {
		return color.Color{}, 0, nil, nil, false, nil
	}
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	self := gitworktree.FindSelf(worktrees, targetDir)
	if self == nil {
		return color.Color{}, 0, nil, nil, false, nil
	}
	main := worktrees[0]
	mainWsPath, err := workspaceFilePath(main.Path)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	mainColor, err := readWorkspacePeacockColor(mainWsPath)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}

	// Case A: main has a color — anchor + offset
	if mainColor != nil {
		offset := color.LadderOffset(gitworktree.IdentityHash(*self))
		return mainColor.ApplyLightness(offset), SourceWorktree, nil, nil, true, nil
	}

	// Cases B/C/D land in subsequent tasks — for now, fall through.
	return color.Color{}, 0, nil, nil, false, nil
}
```

- [ ] **Step 5: Verify tests pass**

Run: `go test ./internal/runner/ -run TestResolveColor_WorktreeCaseA -v`
Expected: PASS for both Case A tests.

Run: `go test ./internal/runner/ -v`
Expected: all existing tests still PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/runner/resolve.go internal/runner/resolve_test.go
git commit -m "$(cat <<'EOF'
runner: implement worktree Case A (main has color → derive offset)

When the main worktree's .code-workspace already has a peacock.color, treat
it as the family anchor and apply LadderOffset(IdentityHash(target)) on top.
Main itself gets offset 0 (IdentityHash returns 0 for IsMain), so its color
is unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: runner/resolve — Case D (linked has color, main empty → warn + fall through)

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write Case D failing tests**

Append to `internal/runner/resolve_test.go`:

```go
func TestResolveColor_WorktreeCaseD_TargetMain(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(linkedPath, 0755); err != nil { t.Fatal(err) }
	// linked has a color; main does not
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#4a8b5c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	_, src, warns, intent, err := ResolveColor(mainPath, "")
	if err != nil { t.Fatal(err) }
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
		if err := os.MkdirAll(p, 0755); err != nil { t.Fatal(err) }
	}
	writeWorkspaceWithColor(t, filepath.Join(base, "myproj-feat-x.code-workspace"), "#4a8b5c")

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedAPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		{Path: linkedBPath, GitDir: filepath.Join(mainPath, ".git/worktrees/bugfix"), IsMain: false},
	}, nil)

	_, src, warns, intent, err := ResolveColor(linkedBPath, "")
	if err != nil { t.Fatal(err) }
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
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/runner/ -run TestResolveColor_WorktreeCaseD -v`
Expected: FAIL — current resolveFromWorktree falls through silently for "main empty" (no warn, no Case-D logic yet).

- [ ] **Step 3: Implement findLinkedWithColor + Case D**

In `internal/runner/resolve.go`, replace the body of `resolveFromWorktree` with the version from Task 7 plus the new Case D branch:

```go
func resolveFromWorktree(targetDir string) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
	worktrees, err := listWorktreesFn(targetDir)
	if errors.Is(err, gitworktree.ErrNotInWorktree) {
		return color.Color{}, 0, nil, nil, false, nil
	}
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	self := gitworktree.FindSelf(worktrees, targetDir)
	if self == nil {
		return color.Color{}, 0, nil, nil, false, nil
	}
	main := worktrees[0]
	mainWsPath, err := workspaceFilePath(main.Path)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	mainColor, err := readWorkspacePeacockColor(mainWsPath)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}

	// Case A
	if mainColor != nil {
		offset := color.LadderOffset(gitworktree.IdentityHash(*self))
		return mainColor.ApplyLightness(offset), SourceWorktree, nil, nil, true, nil
	}

	// main has no color — check whether any other linked worktree has one
	linked, linkedColor, err := findLinkedWithColor(worktrees, self)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}

	// Case D: linked has color but main does not — refuse to derive a family
	if linked != nil {
		warn := formatFamilyDisabledWarning(linked, linkedColor, main, mainWsPath)
		return color.Color{}, 0, []string{warn}, nil, false, nil
	}

	// Cases B/C land in Task 9 — for now, fall through.
	return color.Color{}, 0, nil, nil, false, nil
}

// findLinkedWithColor returns the first non-main worktree (excluding self)
// whose .code-workspace has a peacock.color, along with that color.
func findLinkedWithColor(worktrees []gitworktree.Worktree, self *gitworktree.Worktree) (*gitworktree.Worktree, *color.Color, error) {
	for i := range worktrees {
		w := &worktrees[i]
		if w.IsMain || w.Path == self.Path {
			continue
		}
		wsPath, err := workspaceFilePath(w.Path)
		if err != nil {
			return nil, nil, err
		}
		c, err := readWorkspacePeacockColor(wsPath)
		if err != nil {
			return nil, nil, err
		}
		if c != nil {
			return w, c, nil
		}
	}
	return nil, nil, nil
}

func formatFamilyDisabledWarning(linked *gitworktree.Worktree, linkedColor *color.Color, main gitworktree.Worktree, mainWsPath string) string {
	return fmt.Sprintf(
		"worktree family disabled\n"+
			"  reason     main worktree is uncolored, but linked has color\n"+
			"  linked     %s  %s\n"+
			"  main       %s  (no color)\n"+
			"  hint       set main color first: ccws --color '%s' %s",
		filepath.Base(linked.Path), linkedColor.Hex(),
		main.Path,
		linkedColor.Hex(), main.Path,
	)
}
```

- [ ] **Step 4: Verify tests pass**

Run: `go test ./internal/runner/ -run TestResolveColor_WorktreeCaseD -v`
Expected: PASS for both Case D tests.

Run: `go test ./internal/runner/ -v`
Expected: all tests PASS (Case A still works).

- [ ] **Step 5: Commit**

```bash
git add internal/runner/resolve.go internal/runner/resolve_test.go
git commit -m "$(cat <<'EOF'
runner: implement worktree Case D (linked colored, main empty → disable family)

When a linked worktree already has a peacock.color but the main worktree
does not, ccws assumes the user explicitly set the linked color and refuses
to silently derive a family from it. Emits a warn-level notice that includes
the linked color so the user can copy-paste a `ccws --color '#xxx' <main>`
command, then falls through to the existing settings/random chain.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: runner/resolve — Case B (main target, no color anywhere) and Case C (linked first, auto-establish)

**Files:**
- Modify: `internal/runner/resolve.go`
- Modify: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write Case B and Case C failing tests**

Append to `internal/runner/resolve_test.go`:

```go
func TestResolveColor_WorktreeCaseB_MainTargetNoColor(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
	}, nil)

	_, src, warns, intent, err := ResolveColor(mainPath, "")
	if err != nil { t.Fatal(err) }
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

func TestResolveColor_WorktreeCaseC_LinkedFirst_ReturnsIntent(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(linkedPath, 0755); err != nil { t.Fatal(err) }

	withFakeWorktrees(t, []gitworktree.Worktree{
		{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
		{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
	}, nil)

	c, src, warns, intent, err := ResolveColor(linkedPath, "")
	if err != nil { t.Fatal(err) }
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
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/runner/ -run "TestResolveColor_WorktreeCaseB|TestResolveColor_WorktreeCaseC" -v`
Expected:
- Case B: PASS already (current fall-through behavior matches).
- Case C: FAIL — no anchor-establish logic yet, intent is nil, src is SourceRandom.

If Case B happens to fail because of the absence of `linked == nil && self.IsMain` short-circuit, that's fine — Step 4 makes it explicit.

- [ ] **Step 3: Implement Case B and Case C**

Replace the trailing fall-through in `resolveFromWorktree` (after the Case D block) with:

```go
	// Case B: main is target and has no color, no linked has color either —
	// fall through to existing chain (settings.json → random). No warning.
	if self.IsMain {
		return color.Color{}, 0, nil, nil, false, nil
	}

	// Case C: target is linked, no other worktree has color — auto-establish
	// main as the family anchor with a random color. The runner executes
	// the side effect (writeAnchorWorkspace) using the returned AnchorIntent.
	anchor := color.Random()
	intent := &AnchorIntent{
		WorkspacePath: mainWsPath,
		AnchorColor:   anchor,
	}
	selfWsPath, err := workspaceFilePath(self.Path)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	offset := color.LadderOffset(gitworktree.IdentityHash(*self))
	derived := anchor.ApplyLightness(offset)
	warn := formatAnchorCreatedWarning(mainWsPath, selfWsPath)
	return derived, SourceWorktree, []string{warn}, intent, true, nil
}

func formatAnchorCreatedWarning(mainWsPath, selfWsPath string) string {
	return fmt.Sprintf(
		"family anchor created for main worktree\n"+
			"  anchor at  %s\n"+
			"  applied    %s\n"+
			"  hint       run ccws on main worktree to claim color directly",
		mainWsPath, selfWsPath,
	)
}
```

(The closing `}` from the original `resolveFromWorktree` body should now sit AFTER the Case C return — adjust braces so `formatAnchorCreatedWarning` is a top-level function.)

- [ ] **Step 4: Verify tests pass**

Run: `go test ./internal/runner/ -run "TestResolveColor_WorktreeCaseB|TestResolveColor_WorktreeCaseC" -v`
Expected: PASS for both.

Run: `go test ./internal/runner/ -v`
Expected: all worktree cases (A/B/C/D) and existing tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/resolve.go internal/runner/resolve_test.go
git commit -m "$(cat <<'EOF'
runner: implement worktree Cases B (main empty) and C (linked first auto-anchor)

Case B (target=main, no color anywhere) falls silently through to the
existing settings.json → random chain. Case C (target=linked, no color
anywhere) generates a random anchor, returns an AnchorIntent describing
where to write it (main's .code-workspace), and applies the linked
worktree's lightness offset to the anchor for its own color. The runner
will execute the AnchorIntent in the next task.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: runner.Run — execute `AnchorIntent` (writeAnchorWorkspace)

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/runner/runner_test.go` (near other Run tests):

```go
func TestRun_WorktreeCaseC_WritesMainAnchor(t *testing.T) {
	base := t.TempDir()
	mainPath := filepath.Join(base, "myproj")
	linkedPath := filepath.Join(base, "myproj-feat-x")
	if err := os.MkdirAll(mainPath, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(linkedPath, 0755); err != nil { t.Fatal(err) }

	orig := listWorktreesFn
	t.Cleanup(func() { listWorktreesFn = orig })
	listWorktreesFn = func(string) ([]gitworktree.Worktree, error) {
		return []gitworktree.Worktree{
			{Path: mainPath, GitDir: filepath.Join(mainPath, ".git"), IsMain: true},
			{Path: linkedPath, GitDir: filepath.Join(mainPath, ".git/worktrees/feat-x"), IsMain: false},
		}, nil
	}

	opts := Defaults()
	opts.TargetDir = linkedPath
	opts.NoOpen = true

	r := New(nil)
	res, err := r.Run(opts)
	if err != nil { t.Fatal(err) }

	// Linked workspace must have been written.
	linkedWsPath := filepath.Join(base, "myproj-feat-x.code-workspace")
	if _, err := os.Stat(linkedWsPath); err != nil {
		t.Errorf("linked workspace not written: %v", err)
	}
	// Main workspace must have been written by AnchorIntent side effect.
	mainWsPath := filepath.Join(base, "myproj.code-workspace")
	if _, err := os.Stat(mainWsPath); err != nil {
		t.Errorf("main anchor workspace not written: %v", err)
	}
	// The result must report a worktree-sourced color and surface the warning.
	if res.ColorSource != SourceWorktree {
		t.Errorf("ColorSource = %v, want SourceWorktree", res.ColorSource)
	}
	hasAnchorWarn := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "family anchor created") {
			hasAnchorWarn = true
		}
	}
	if !hasAnchorWarn {
		t.Errorf("warnings = %v, want anchor-created notice", res.Warnings)
	}
}
```

(`Defaults`, `New`, and `r.Run` should match existing patterns in `runner_test.go`. Adjust call shapes if the test file uses different helpers — e.g., a `setupRunner(t)` factory.)

- [ ] **Step 2: Verify test fails**

Run: `go test ./internal/runner/ -run TestRun_WorktreeCaseC_WritesMainAnchor -v`
Expected: FAIL — `myproj.code-workspace` is not written; `_ = anchorIntent` in runner.go discards the intent.

- [ ] **Step 3: Implement writeAnchorWorkspace and wire it into Run**

In `internal/runner/runner.go`, replace `_ = anchorIntent // wired in Task 10` with:

```go
if anchorIntent != nil {
    if err := writeAnchorWorkspace(anchorIntent, opts); err != nil {
        return nil, fmt.Errorf("write main anchor workspace: %w", err)
    }
}
```

Append to `internal/runner/runner.go`:

```go
// writeAnchorWorkspace materialises an AnchorIntent: read or create the main
// worktree's .code-workspace, merge in the peacock palette derived from the
// anchor color, and write it back. Does NOT touch main's .vscode/settings.json
// — that side effect would be invasive on a directory the user did not target.
func writeAnchorWorkspace(intent *AnchorIntent, opts Options) error {
	ws, err := workspace.Read(intent.WorkspacePath)
	if err != nil {
		return err
	}
	if ws == nil {
		ws = &workspace.Workspace{}
	}
	folderName := strings.TrimSuffix(filepath.Base(intent.WorkspacePath), ".code-workspace")
	workspace.EnsureFolder(ws, "./"+folderName)
	palette := color.Palette(intent.AnchorColor, opts.Palette)
	workspace.ApplyPeacock(ws, intent.AnchorColor.Hex(), palette)
	return workspace.Write(intent.WorkspacePath, ws)
}
```

(If `runner.go` doesn't already import `strings`, add it. The `_ = mainDir` line keeps the variable available for any future logging without provoking an unused-variable error; remove it if the linter complains.)

- [ ] **Step 4: Verify the test passes**

Run: `go test ./internal/runner/ -run TestRun_WorktreeCaseC_WritesMainAnchor -v`
Expected: PASS — both `.code-workspace` files exist; `ColorSource == SourceWorktree`; warning includes "family anchor created".

Run: `go test ./internal/runner/ -v`
Expected: all runner tests PASS.

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "$(cat <<'EOF'
runner: execute AnchorIntent — auto-write main anchor workspace

When ResolveColor returns a non-nil AnchorIntent (Case C: linked worktree
is first to be ccws'd), Run merges the anchor's peacock palette into the
main worktree's .code-workspace before continuing with the linked target.
main's .vscode/settings.json is intentionally left untouched.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: cmd/ccws/render — `sourceLabel` for `SourceWorktree`

**Files:**
- Modify: `cmd/ccws/render.go`
- Modify: `cmd/ccws/render_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/ccws/render_test.go`:

```go
func TestSourceLabel_Worktree(t *testing.T) {
	got := sourceLabel(runner.SourceWorktree)
	if got != "from worktree family" {
		t.Errorf("sourceLabel(SourceWorktree) = %q, want %q", got, "from worktree family")
	}
}
```

(Imports already include `runner` based on existing tests; no change needed.)

- [ ] **Step 2: Verify test fails**

Run: `go test ./cmd/ccws/ -run TestSourceLabel_Worktree -v`
Expected: FAIL — `sourceLabel(SourceWorktree)` returns empty string (no case in the switch).

- [ ] **Step 3: Implement the new label**

Edit `cmd/ccws/render.go`. Find the `sourceLabel` function and add the new case:

```go
func sourceLabel(s runner.ColorSource) string {
	switch s {
	case runner.SourceFlag:
		return "from --color flag"
	case runner.SourceSettings:
		return "from .vscode/settings.json"
	case runner.SourceWorktree:
		return "from worktree family"
	case runner.SourceRandom:
		return "random"
	}
	return ""
}
```

(If the existing function has different label strings, keep them and only add the `SourceWorktree` case. The string `"from worktree family"` is the contract — keep it stable so the test stays green.)

- [ ] **Step 4: Verify test passes**

Run: `go test ./cmd/ccws/ -v`
Expected: all render tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ccws/render.go cmd/ccws/render_test.go
git commit -m "$(cat <<'EOF'
ccws: add sourceLabel case for SourceWorktree

Renders "from worktree family" beside the color hex when the color was
derived from a sibling worktree's anchor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Taskfile — add `task test:integration`

**Files:**
- Modify: `Taskfile.yml`

- [ ] **Step 1: Add the task target**

Open `Taskfile.yml` and add under `tasks:`:

```yaml
  test:integration:
    desc: Run integration tests (requires git on PATH)
    cmds:
      - go test -tags=integration -count=1 ./...
```

- [ ] **Step 2: Verify the target works**

Run: `task test:integration`
Expected: PASS (gitworktree integration tests run; non-integration tests are unaffected).

- [ ] **Step 3: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
build: add `task test:integration` target

Runs `go test -tags=integration -count=1 ./...` for tests guarded by
//go:build integration (currently the gitworktree real-git smoke test).
Kept separate from `task test` so the default test run does not require
git to be installed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Documentation — `README.md` and `CLAUDE.md`

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add the "Worktree color family" section to `README.md`**

In `README.md`, after the existing "Usage" section and before the "Safety guards" section, insert:

```markdown
## Worktree color family

When you run `ccws` inside a git worktree, it automatically picks a "family" color so sibling worktrees of the same repo look related but distinct (same hue/saturation, lightness shifted by ±5/±10/±15%).

- First `ccws` on the main worktree: random color, becomes the family anchor.
- First `ccws` on a linked worktree (main not yet colored): a random anchor is written to the main worktree's `.code-workspace` automatically, and the linked worktree gets a derived color. A warning is printed to stderr.
- If linked worktrees already have colors but main does not, ccws assumes you set them deliberately, prints a warning, and disables family logic for that run.
- Pass `--color` to bypass family logic.

The worktree identity is stable across branch renames and `git worktree move` (it uses the name git assigns under `.git/worktrees/<name>`).
```

- [ ] **Step 2: Update `CLAUDE.md`**

In `CLAUDE.md`, find the "Package import rule" section and update the DAG to include `gitworktree`:

```
color → (stdlib only)
peacock → (stdlib only)
jsonc → hujson
tui → lipgloss, isatty, termenv
gitworktree → (stdlib only)
workspace, vscodesettings → peacock, jsonc
runner → color, workspace, vscodesettings, gitworktree
interactive → runner, vscodesettings
cmd/ccws → runner, interactive, tui
```

In the "Non-goals" section, append the worktree-related items:

```
Peacock favorites, `peacock.remoteColor` / Live Share, multi-root workspaces, VSCode Profiles integration, `.code-workspace` comment preservation on rewrite, uninstall subcommand, similar-color across non-worktree clones of the same repo, user-tunable hash-to-offset mapping.
```

(Keep the existing items; only add the two new tail items.)

In the "Commands" section, add a brief note about the new task target:

```
task test:integration  # gitworktree integration tests (needs git on PATH)
```

- [ ] **Step 3: Verify lint and tests still pass**

Run: `task lint`
Expected: PASS (gofmt clean, go vet clean).

Run: `task test:race`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: README + CLAUDE.md for worktree color family

README gains a "Worktree color family" section describing the auto-derived
family behavior, the linked-first auto-anchor, and the family-disabled case.
CLAUDE.md updates the package DAG (gitworktree → stdlib only; runner gains
gitworktree) and adds two non-goals (cross-clone similarity, user-tunable
hash mapping).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Final verification — `task ci`, integration tests, manual smoke

**Files:** none (verification only)

- [ ] **Step 1: Full CI run**

Run: `task ci`
Expected: PASS (lint + race tests).

- [ ] **Step 2: Integration tests**

Run: `task test:integration`
Expected: PASS (gitworktree real-git integration test runs and passes).

- [ ] **Step 3: Manual smoke — main first, linked next**

```bash
task build
TMPROOT=$(mktemp -d)
git init "$TMPROOT/myproj"
git -C "$TMPROOT/myproj" commit --allow-empty -m init
git -C "$TMPROOT/myproj" worktree add -b feat-x "$TMPROOT/myproj-feat-x"

./bin/ccws --no-open "$TMPROOT/myproj"           # ok badge, random color
./bin/ccws --no-open "$TMPROOT/myproj-feat-x"    # ok badge; linked color is a lightness shift of main

# Inspect the two .code-workspace files. peacock.color values should share hue/saturation but differ in lightness.
grep peacock.color "$TMPROOT"/myproj.code-workspace
grep peacock.color "$TMPROOT"/myproj-feat-x.code-workspace
```

Expected: both files exist; their peacock.color values differ but read as obviously similar shades.

- [ ] **Step 4: Manual smoke — linked first (auto-anchor)**

```bash
TMPROOT=$(mktemp -d)
git init "$TMPROOT/myproj"
git -C "$TMPROOT/myproj" commit --allow-empty -m init
git -C "$TMPROOT/myproj" worktree add -b feat-x "$TMPROOT/myproj-feat-x"

./bin/ccws --no-open "$TMPROOT/myproj-feat-x"
# Expected output:
#   warn  family anchor created for main worktree
#         anchor at  <TMPROOT>/myproj.code-workspace
#         applied    <TMPROOT>/myproj-feat-x.code-workspace
#         hint       run ccws on main worktree to claim color directly
#   ok    workspace ready
#         <TMPROOT>/myproj-feat-x.code-workspace
ls "$TMPROOT"/myproj.code-workspace      # should exist (auto-created)
ls "$TMPROOT"/myproj-feat-x.code-workspace  # should exist
```

- [ ] **Step 5: Manual smoke — Case D (linked colored, main empty)**

```bash
TMPROOT=$(mktemp -d)
git init "$TMPROOT/myproj"
git -C "$TMPROOT/myproj" commit --allow-empty -m init
git -C "$TMPROOT/myproj" worktree add -b feat-x "$TMPROOT/myproj-feat-x"

# Color the linked worktree first via --color (forces explicit, bypasses family).
./bin/ccws --no-open --color '#4a8b5c' "$TMPROOT/myproj-feat-x"
# main has no color yet. Now run on main:
./bin/ccws --no-open "$TMPROOT/myproj"
# Expected output:
#   warn  worktree family disabled
#         reason     main worktree is uncolored, but linked has color
#         linked     myproj-feat-x  #4a8b5c
#         main       <TMPROOT>/myproj  (no color)
#         hint       set main color first: ccws --color '#4a8b5c' <TMPROOT>/myproj
#   ok    workspace ready
#         <TMPROOT>/myproj.code-workspace
```

main's color should be a fresh random — explicitly NOT #4a8b5c or a lightness shift of it.

- [ ] **Step 6: If everything passes, no commit needed (no code changes). If any smoke test surfaces a bug, fix it inline and commit with message describing the fix.**

---

## Self-Review Notes

Coverage map:
- Spec §2 Cases A/B/C/D → Tasks 7/9/9/8
- Spec §3 priority chain → Task 5 (signature) + Tasks 7-9 (worktree branches)
- Spec §5 packages → Task 1 (gitworktree skeleton), Task 4 (color/ladder)
- Spec §6 gitworktree API → Tasks 1-3
- Spec §7 ApplyLightness → Task 4
- Spec §8 ResolveColor + AnchorIntent → Tasks 5, 6, 7, 8, 9
- Spec §9 sourceLabel → Task 11
- Spec §10 interactive — no code changes (worktree logic feeds the existing form path automatically)
- Spec §11 testing → Tasks 1-11 (each task adds its own tests)
- Spec §12 edge cases → covered implicitly by `errors.Is(err, ErrNotInWorktree)` collapse in `gitworktree.List` (Task 3)
- Spec §13 migration notes → Task 13 (CLAUDE.md / README.md)
- Spec §14 docs → Task 13
- Spec §15 file summary → matches the File Structure table at the top of this plan

No placeholders remain. All `[ ]` steps contain concrete code, exact paths, exact commands, expected output, and a commit message.
