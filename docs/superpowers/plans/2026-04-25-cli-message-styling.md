# CLI message styling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace plain `fmt.Print*` CLI output with badge + detail-row rendering using a new `internal/tui` package; decouple `runner.GuardError` from presentation.

**Architecture:** Library-pure `internal/tui` exports primitives (`OK`, `Warn`, `Error`, `Details`, `Bullets`, `ShortenPath`) over a `Writer` that auto-detects color (TTY + `NO_COLOR`). Domain-to-tui dispatch lives in new `cmd/ccws/render.go` (uses `errors.As` for `*runner.GuardError`). `GuardError` becomes data-only — drop `Message`, add `Path`.

**Tech Stack:** Go 1.25, `github.com/charmbracelet/lipgloss` (already transitive via huh), `github.com/mattn/go-isatty` (already transitive).

---

## Output format (locked, used by all tasks)

**Layout constants** (no-color and color modes both adhere):
- Leading indent: 2 spaces
- Badge column width: 5 chars (= len("error"), the longest label)
- Separator (badge → content): 2 spaces
- Continuation indent: 9 spaces (2 + 5 + 2)
- Bullet indent: continuation indent + 2 = 11 spaces; glyph `• ` (with trailing space)
- Truncation suffix line: `…(N more)` at bullet indent

**No-color mode** (deterministic byte output, used in tests):

| Call | Output |
|---|---|
| `OK("wrote foo")` | `"  ok     wrote foo\n"` |
| `Warn("a")` | `"  warn   a\n"` |
| `Error("e")` | `"  error  e\n"` |
| `Details([{"color","#abc"}])` | `"         color  #abc\n"` |
| `Details([{"keys",""}])` (empty value = header) | `"         keys\n"` |
| `Bullets(["a","b"], 8)` | `"           • a\n           • b\n"` |
| `Bullets(17 items, 8)` | 8 bullet lines + `"           …(9 more)\n"` |

**Color mode:** same layout, badge cell wrapped in lipgloss style (bg+fg+bold, `Width(5)`). Continuation/bullet lines stay plain spaces (no styling on indents).

**Multiple warnings:** rendered with one blank line between each `warn` block.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/tui/tui.go` | NEW | `Writer` struct, `NewWriter/NewStdout/NewStderr`, `OK/Warn/Error/Details/Bullets/ShortenPath` |
| `internal/tui/tui_test.go` | NEW | Deterministic snapshot tests for all primitives |
| `cmd/ccws/render.go` | NEW | `renderError`, `renderGuard`, `guardDescription`, `renderSuccess`, `renderWarnings` |
| `cmd/ccws/render_test.go` | NEW | Tests for guard/success/warning rendering with injected buffer |
| `internal/runner/runner.go` | MODIFY | Drop `GuardError.Message`, add `Path`, rewrite `Error()`, update guard sites |
| `internal/runner/runner_test.go` | MODIFY | Assert `Path` and `Keys` on guard errors |
| `cmd/ccws/main.go` | MODIFY | Replace `fmt.Fprintln(os.Stderr, err)` with `renderError` |
| `cmd/ccws/root.go` | MODIFY | Replace `fmt.Printf` success/warning lines with `renderSuccess/renderWarnings` |
| `cmd/ccws/interactive.go` | MODIFY | Same wiring + use `guardDescription` in `confirmGuard` |
| `go.mod` | MODIFY | Promote `lipgloss` and `go-isatty` from indirect → direct |
| `CLAUDE.md` | MODIFY | Add `tui → (stdlib + lipgloss + isatty)` to DAG section |

---

## Task 1: Bootstrap `internal/tui` package + Writer struct

**Files:**
- Create: `internal/tui/tui.go`
- Create: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing test**

`internal/tui/tui_test.go`:

```go
package tui

import (
	"bytes"
	"testing"
)

func TestNewWriter_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/...`
Expected: FAIL with `package internal/tui: no Go files` or build error referencing `NewWriter`.

- [ ] **Step 3: Write minimal implementation**

`internal/tui/tui.go`:

```go
// Package tui renders CLI output: badge headers, detail rows, bullet lists.
package tui

import (
	"io"
)

// Writer renders styled CLI output to an io.Writer.
// Color rendering is enabled per-instance (see NewStdout/NewStderr/NewWriter).
type Writer struct {
	out   io.Writer
	color bool
}

// NewWriter returns a Writer over out. color enables lipgloss styling; pass
// false for tests, plain logs, or non-TTY output.
func NewWriter(out io.Writer, color bool) *Writer {
	return &Writer{out: out, color: color}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "tui: bootstrap package with Writer struct"
```

---

## Task 2: `OK`, `Warn`, `Error` badge methods (no-color mode)

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/tui_test.go`:

```go
func TestOK_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.OK("wrote foo")
	got := buf.String()
	want := "  ok     wrote foo\n"
	if got != want {
		t.Errorf("OK output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWarn_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Warn("a")
	got := buf.String()
	want := "  warn   a\n"
	if got != want {
		t.Errorf("Warn output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestError_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Error("e")
	got := buf.String()
	want := "  error  e\n"
	if got != want {
		t.Errorf("Error output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL with `w.OK undefined` (or similar).

- [ ] **Step 3: Implement badge methods**

Append to `internal/tui/tui.go`:

```go
import (
	"fmt"
	"io"
	"strings"
)

const (
	leadingIndent  = "  "
	badgeWidth     = 5    // len("error"), longest of ok/warn/error
	badgeSeparator = "  "
)

// continuationIndent is the leading whitespace for rows under a badge.
// Width = leadingIndent + badgeWidth + badgeSeparator = 2+5+2 = 9.
var continuationIndent = strings.Repeat(" ", len(leadingIndent)+badgeWidth+len(badgeSeparator))

// OK writes a green "ok" badge line.
func (w *Writer) OK(title string)    { w.badge("ok", title) }
// Warn writes a yellow "warn" badge line.
func (w *Writer) Warn(title string)  { w.badge("warn", title) }
// Error writes a red "error" badge line.
func (w *Writer) Error(title string) { w.badge("error", title) }

func (w *Writer) badge(label, title string) {
	cell := w.renderBadge(label)
	fmt.Fprintf(w.out, "%s%s%s%s\n", leadingIndent, cell, badgeSeparator, title)
}

// renderBadge returns the badge cell, padded to badgeWidth.
// In no-color mode this is just the left-aligned label; in color mode
// (added in Task 5) it is wrapped in lipgloss style.
func (w *Writer) renderBadge(label string) string {
	if !w.color {
		return label + strings.Repeat(" ", badgeWidth-len(label))
	}
	// color path filled in by Task 5
	return label + strings.Repeat(" ", badgeWidth-len(label))
}
```

(Merge the new `import` block with the existing one; the file should have exactly one `import (...)` group.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS (all three tests).

- [ ] **Step 5: Run gofmt + vet**

Run: `task lint`
Expected: PASS (no output, exit 0).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "tui: add OK/Warn/Error badge methods (no-color)"
```

---

## Task 3: `Details` method (label + value rows)

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/tui_test.go`:

```go
func TestDetails_LabelValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Details([]Detail{
		{Label: "color", Value: "#abc"},
		{Label: "file", Value: "~/x"},
	})
	got := buf.String()
	want := "         color  #abc\n         file  ~/x\n"
	if got != want {
		t.Errorf("Details mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDetails_HeaderRow(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Details([]Detail{{Label: "keys", Value: ""}})
	got := buf.String()
	want := "         keys\n"
	if got != want {
		t.Errorf("Details header mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL with `Detail undefined` or `Details undefined`.

- [ ] **Step 3: Implement Details**

Append to `internal/tui/tui.go`:

```go
// Detail is one row under a badge. Empty Value renders as a header line
// (used to introduce a Bullets list, e.g. label "keys" above bullets).
type Detail struct {
	Label string
	Value string
}

// Details writes detail rows at the continuation indent. Each row is
// "<continuationIndent><label>  <value>" (or just "<continuationIndent><label>"
// when Value is empty). Labels are not column-aligned across rows.
func (w *Writer) Details(rows []Detail) {
	for _, r := range rows {
		if r.Value == "" {
			fmt.Fprintf(w.out, "%s%s\n", continuationIndent, r.Label)
		} else {
			fmt.Fprintf(w.out, "%s%s  %s\n", continuationIndent, r.Label, r.Value)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "tui: add Details for label+value rows"
```

---

## Task 4: `Bullets` method with truncation

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/tui_test.go`:

```go
func TestBullets_NoTruncation(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets([]string{"a", "b", "c"}, 8)
	got := buf.String()
	want := "           • a\n           • b\n           • c\n"
	if got != want {
		t.Errorf("Bullets mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBullets_Truncates(t *testing.T) {
	items := make([]string, 17)
	for i := range items {
		items[i] = fmt.Sprintf("k%d", i)
	}
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(items, 8)
	got := buf.String()
	var want strings.Builder
	for i := 0; i < 8; i++ {
		want.WriteString(fmt.Sprintf("           • k%d\n", i))
	}
	want.WriteString("           …(9 more)\n")
	if got != want.String() {
		t.Errorf("Bullets truncation mismatch:\ngot:  %q\nwant: %q", got, want.String())
	}
}

func TestBullets_Empty(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(nil, 8)
	if buf.Len() != 0 {
		t.Errorf("Bullets(nil) should produce no output, got %q", buf.String())
	}
}

func TestBullets_ExactlyAtLimit(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h"} // exactly 8
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(items, 8)
	got := buf.String()
	if strings.Contains(got, "more") {
		t.Errorf("expected no truncation marker for items==max, got %q", got)
	}
	lines := strings.Count(got, "\n")
	if lines != 8 {
		t.Errorf("expected 8 lines, got %d in %q", lines, got)
	}
}
```

`fmt` and `strings` are already imported by earlier tests; if not, add them to the imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL with `w.Bullets undefined`.

- [ ] **Step 3: Implement Bullets**

Append to `internal/tui/tui.go`:

```go
// bulletIndent is continuation indent + 2 spaces for the bullet glyph.
var bulletIndent = continuationIndent + "  "

// Bullets writes up to max items as bulleted lines. When len(items) > max,
// the first max items are written and a final "…(N more)" line is appended.
// max <= 0 disables truncation.
func (w *Writer) Bullets(items []string, max int) {
	shown := items
	if max > 0 && len(items) > max {
		shown = items[:max]
	}
	for _, it := range shown {
		fmt.Fprintf(w.out, "%s• %s\n", bulletIndent, it)
	}
	if max > 0 && len(items) > max {
		fmt.Fprintf(w.out, "%s…(%d more)\n", bulletIndent, len(items)-max)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS (all 4 bullet tests + earlier tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "tui: add Bullets with truncation marker"
```

---

## Task 5: Color mode + `NewStdout`/`NewStderr`

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`
- Modify: `go.mod` (promote lipgloss + isatty to direct)

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/tui_test.go`:

```go
func TestBadge_ColorEmitsANSI(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, true)
	w.Error("boom")
	got := buf.String()
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("color=true output should contain ANSI escape, got %q", got)
	}
	if !strings.Contains(got, "boom") {
		t.Errorf("title text missing from output: %q", got)
	}
}

func TestNewStdout_HonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := NewStdout()
	if w.color {
		t.Error("NO_COLOR=1 should disable color")
	}
}

func TestNewStdout_HonorsTermDumb(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	w := NewStdout()
	if w.color {
		t.Error("TERM=dumb should disable color")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL — color path is still inert (Task 2 noted "color path filled in by Task 5"), and `NewStdout` is undefined.

- [ ] **Step 3: Promote dependencies to direct**

Run:
```bash
go get github.com/charmbracelet/lipgloss
go get github.com/mattn/go-isatty
```

Verify `go.mod` now lists both without the `// indirect` comment.

- [ ] **Step 4: Implement color path + NewStdout/NewStderr**

Replace the `renderBadge` body in `internal/tui/tui.go`. Add the new constructors and styles.

```go
import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	styleOK = lipgloss.NewStyle().
		Background(lipgloss.Color("10")).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Width(badgeWidth)

	styleWarn = lipgloss.NewStyle().
		Background(lipgloss.Color("11")).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Width(badgeWidth)

	styleError = lipgloss.NewStyle().
		Background(lipgloss.Color("9")).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Width(badgeWidth)
)

// NewStdout returns a Writer over os.Stdout, with color enabled when stdout is
// a TTY, NO_COLOR is unset, and TERM != "dumb".
func NewStdout() *Writer {
	return &Writer{out: os.Stdout, color: shouldColor(os.Stdout.Fd())}
}

// NewStderr is NewStdout for os.Stderr.
func NewStderr() *Writer {
	return &Writer{out: os.Stderr, color: shouldColor(os.Stderr.Fd())}
}

func shouldColor(fd uintptr) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(fd)
}
```

Then update `renderBadge` to actually use the styles:

```go
func (w *Writer) renderBadge(label string) string {
	if !w.color {
		return label + strings.Repeat(" ", badgeWidth-len(label))
	}
	switch label {
	case "ok":
		return styleOK.Render(label)
	case "warn":
		return styleWarn.Render(label)
	case "error":
		return styleError.Render(label)
	default:
		return label + strings.Repeat(" ", badgeWidth-len(label))
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS (all tests including the three new color tests).

- [ ] **Step 6: Run lint**

Run: `task lint`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go go.mod go.sum
git commit -m "tui: add color mode (lipgloss) + NewStdout/NewStderr"
```

---

## Task 6: `ShortenPath` helper

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/tui_test.go`:

```go
func TestShortenPath(t *testing.T) {
	tests := []struct {
		name string
		home string
		in   string
		want string
	}{
		{"prefix replaced", "/Users/x", "/Users/x/p", "~/p"},
		{"exact home", "/Users/x", "/Users/x", "~"},
		{"non-prefix unchanged", "/Users/x", "/tmp/p", "/tmp/p"},
		{"sibling not replaced", "/Users/x", "/Users/xy/p", "/Users/xy/p"},
		{"home unset", "", "/tmp/p", "/tmp/p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			got := ShortenPath(tt.in)
			if got != tt.want {
				t.Errorf("ShortenPath(%q) with HOME=%q = %q, want %q",
					tt.in, tt.home, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL with `ShortenPath undefined`.

- [ ] **Step 3: Implement ShortenPath**

Append to `internal/tui/tui.go`:

```go
// ShortenPath replaces a leading $HOME with "~". If HOME is unset or p does
// not begin with $HOME (treating $HOME as a directory boundary), p is returned
// unchanged. Examples (HOME=/Users/x):
//   "/Users/x"      → "~"
//   "/Users/x/p"    → "~/p"
//   "/Users/xy/p"   → "/Users/xy/p"  (sibling, not a child of HOME)
//   "/tmp/p"        → "/tmp/p"
func ShortenPath(p string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS (all 5 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "tui: add ShortenPath ($HOME → ~)"
```

---

## Task 7: Refactor `runner.GuardError` to data-only

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing test changes**

Edit `internal/runner/runner_test.go`. In `TestRun_Guard1_Triggers` (around line 81), append after the existing `gerr.Guard != 1` check:

```go
	wantPath := filepath.Join(tmp, "myproj.code-workspace")
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
```

In `TestRun_Guard2_Triggers` (around line 129), append the same block but with:

```go
	wantPath := filepath.Join(target, ".vscode", "settings.json")
```

Add `"strings"` to the imports of `runner_test.go` if not present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/...`
Expected: FAIL because `gerr.Path` does not exist.

- [ ] **Step 3: Refactor `GuardError`**

In `internal/runner/runner.go`, replace lines 16-25:

```go
// GuardError indicates a safety guard triggered. Exit code 2.
// Carries data only; presentation is the CLI layer's responsibility.
type GuardError struct {
	Guard int
	Path  string   // workspace file (Guard 1) or settings.json (Guard 2)
	Keys  []string // conflicting / residual keys
}

// Error returns a single-line summary used by %v, log lines, and errors.As
// fallbacks. The full multi-line presentation is rendered by cmd/ccws.
func (e *GuardError) Error() string {
	return fmt.Sprintf("guard %d: %d conflicting keys in %s", e.Guard, len(e.Keys), e.Path)
}
```

Then update the two guard sites (lines ~81-87 and ~99-105):

```go
	// Guard 1 (replaces lines 81-88):
	if ws != nil && !opts.Force {
		if keys := workspace.ExistingPeacockKeys(ws); len(keys) > 0 {
			return nil, &GuardError{Guard: 1, Path: wsPath, Keys: keys}
		}
	}

	// Guard 2 (replaces lines 97-107):
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
```

The `strings` import in `runner.go` may no longer be needed — remove it if `goimports`/`go vet` complains.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/...`
Expected: PASS.

- [ ] **Step 5: Confirm full build still passes**

Run: `task build`
Expected: PASS — but note that `cmd/ccws/main.go` will still print the new short `Error()` message until Task 9 wires `renderError`. That's fine; the build succeeds.

- [ ] **Step 6: Run lint**

Run: `task lint`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "runner: GuardError becomes data-only (drop Message, add Path)"
```

---

## Task 8: Create `cmd/ccws/render.go`

**Files:**
- Create: `cmd/ccws/render.go`
- Create: `cmd/ccws/render_test.go`

- [ ] **Step 1: Write the failing tests**

`cmd/ccws/render_test.go`:

```go
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

func TestRenderError_Plain(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderError(w, errors.New("boom"))
	got := buf.String()
	want := "  error  boom\n"
	if got != want {
		t.Errorf("renderError plain:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderError_Guard1(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	ge := &runner.GuardError{
		Guard: 1,
		Path:  "/tmp/foo.code-workspace",
		Keys:  []string{"settings.peacock.color", "settings.workbench.colorCustomizations.activityBar.background"},
	}
	renderError(w, ge)
	got := buf.String()
	for _, want := range []string{
		"  error  guard 1: existing peacock settings would be overwritten\n",
		"         file  /tmp/foo.code-workspace\n",
		"         keys\n",
		"           • settings.peacock.color\n",
		"           • settings.workbench.colorCustomizations.activityBar.background\n",
		"         hint  rerun with --force to overwrite\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fragment %q in output:\n%s", want, got)
		}
	}
}

func TestRenderError_Guard1_Truncates(t *testing.T) {
	keys := make([]string, 17)
	for i := range keys {
		keys[i] = "k" + string(rune('0'+i%10))
	}
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderError(w, &runner.GuardError{Guard: 1, Path: "/tmp/x", Keys: keys})
	got := buf.String()
	if !strings.Contains(got, "…(9 more)") {
		t.Errorf("expected '…(9 more)' truncation, got:\n%s", got)
	}
}

func TestRenderError_Guard2_Title(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderError(w, &runner.GuardError{Guard: 2, Path: "/tmp/.vscode/settings.json", Keys: []string{"editor.background"}})
	got := buf.String()
	if !strings.Contains(got, "guard 2: non-peacock keys would remain") {
		t.Errorf("Guard 2 title missing, got:\n%s", got)
	}
	if !strings.Contains(got, "remove those keys manually or rerun with --force") {
		t.Errorf("Guard 2 hint missing, got:\n%s", got)
	}
}

func TestRenderSuccess_WithSrcLabel(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	res := &runner.Result{
		WorkspaceFile: "/tmp/foo.code-workspace",
		ColorHex:      "#abcdef",
	}
	renderSuccess(w, res, "from --color")
	got := buf.String()
	wantFragments := []string{
		"  ok     wrote /tmp/foo.code-workspace\n",
		"         color  #abcdef (from --color)\n",
	}
	for _, want := range wantFragments {
		if !strings.Contains(got, want) {
			t.Errorf("missing fragment %q in output:\n%s", want, got)
		}
	}
}

func TestRenderSuccess_EmptySrcLabel(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	res := &runner.Result{
		WorkspaceFile: "/tmp/foo.code-workspace",
		ColorHex:      "#abcdef",
	}
	renderSuccess(w, res, "")
	got := buf.String()
	if !strings.Contains(got, "         color  #abcdef\n") {
		t.Errorf("expected color row without parens, got:\n%s", got)
	}
	if strings.Contains(got, "(") {
		t.Errorf("expected no parens with empty srcLabel, got:\n%s", got)
	}
}

func TestRenderWarnings_Multiple(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderWarnings(w, []string{"first", "second"})
	got := buf.String()
	want := "  warn   first\n\n  warn   second\n"
	if got != want {
		t.Errorf("renderWarnings:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderWarnings_Empty(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderWarnings(w, nil)
	if buf.Len() != 0 {
		t.Errorf("renderWarnings(nil) should produce no output, got %q", buf.String())
	}
}

func TestGuardDescription_Plain(t *testing.T) {
	ge := &runner.GuardError{Guard: 1, Path: "/tmp/x", Keys: []string{"a", "b"}}
	got := guardDescription(ge)
	if strings.Contains(got, "\x1b[") {
		t.Errorf("guardDescription should be plain text (no ANSI), got %q", got)
	}
	for _, want := range []string{"file  /tmp/x", "• a", "• b", "rerun with --force"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fragment %q in: %s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ccws/...`
Expected: FAIL — `renderError`, `renderSuccess`, `renderWarnings`, `guardDescription` undefined.

- [ ] **Step 3: Implement render.go**

`cmd/ccws/render.go`:

```go
package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

const maxBulletsShown = 8

// renderError dispatches by error type. Called from main.go for the top-level
// error and from interactive.go for non-recoverable errors.
func renderError(w *tui.Writer, err error) {
	var ge *runner.GuardError
	if errors.As(err, &ge) {
		renderGuard(w, ge)
		return
	}
	w.Error(err.Error())
}

// renderGuard composes badge + details + bullets + hint for a *GuardError.
func renderGuard(w *tui.Writer, ge *runner.GuardError) {
	w.Error(guardTitle(ge))
	w.Details([]tui.Detail{{Label: "file", Value: tui.ShortenPath(ge.Path)}})
	w.Details([]tui.Detail{{Label: "keys"}}) // header for the bullet list
	w.Bullets(ge.Keys, maxBulletsShown)
	w.Details([]tui.Detail{{Label: "hint", Value: guardHint(ge)}})
}

// guardDescription returns the same body content as renderGuard but as plain
// text (no badge, no ANSI). Used by the huh confirm dialog in interactive
// mode, where huh draws its own border.
func guardDescription(ge *runner.GuardError) string {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	w.Details([]tui.Detail{
		{Label: "file", Value: tui.ShortenPath(ge.Path)},
		{Label: "keys"},
	})
	w.Bullets(ge.Keys, maxBulletsShown)
	w.Details([]tui.Detail{{Label: "hint", Value: guardHint(ge)}})
	return buf.String()
}

func guardTitle(ge *runner.GuardError) string {
	switch ge.Guard {
	case 1:
		return "guard 1: existing peacock settings would be overwritten"
	case 2:
		return "guard 2: non-peacock keys would remain in .vscode/settings.json"
	default:
		return fmt.Sprintf("guard %d", ge.Guard)
	}
}

func guardHint(ge *runner.GuardError) string {
	if ge.Guard == 2 {
		return "remove those keys manually or rerun with --force"
	}
	return "rerun with --force to overwrite"
}

// renderSuccess writes the success block: ok badge + file + color rows.
// Empty srcLabel suppresses the "(...)" suffix on the color row — used by
// interactive mode where the source is implicit.
func renderSuccess(w *tui.Writer, res *runner.Result, srcLabel string) {
	w.OK("wrote " + tui.ShortenPath(res.WorkspaceFile))
	value := res.ColorHex
	if srcLabel != "" {
		value += " (" + srcLabel + ")"
	}
	w.Details([]tui.Detail{{Label: "color", Value: value}})
}

// renderWarnings writes one warn block per message, with a blank line between.
func renderWarnings(w *tui.Writer, warnings []string) {
	for i, msg := range warnings {
		if i > 0 {
			fmt.Fprintln(w.Out())
		}
		w.Warn(msg)
	}
}
```

`renderWarnings` calls `w.Out()` which doesn't exist yet — add it to `internal/tui/tui.go`:

```go
// Out returns the underlying writer. Used by callers that need to interleave
// raw output (e.g., blank-line separators) with badge methods.
func (w *Writer) Out() io.Writer { return w.out }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ccws/... ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Run lint**

Run: `task lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ccws/render.go cmd/ccws/render_test.go internal/tui/tui.go
git commit -m "ccws: add render.go with badge-based error/success/warning output"
```

---

## Task 9: Wire `main.go`

**Files:**
- Modify: `cmd/ccws/main.go`

- [ ] **Step 1: Replace the error print**

Edit `cmd/ccws/main.go` to:

```go
package main

import (
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

func main() {
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		renderError(tui.NewStderr(), err)
		os.Exit(errToExit(err))
	}
}
```

The `fmt` import is no longer needed — remove it.

- [ ] **Step 2: Build**

Run: `task build`
Expected: PASS (`./ccws` produced).

- [ ] **Step 3: Manual smoke — generic error**

Run: `./ccws /tmp/does-not-exist 2>&1`
Expected output to stderr (color enabled in TTY):

```
  error  target does not exist: /tmp/does-not-exist
```

Exit code 1 (`echo $?` should print `1`).

- [ ] **Step 4: Manual smoke — guard error**

Set up: pick any directory with a sibling `.code-workspace` file containing peacock settings (the user's own example will work).

Run: `./ccws <that-dir> 2>&1`
Expected: full guard-1 block (badge + file + keys + truncation if >8 + hint), exit code 2.

- [ ] **Step 5: Manual smoke — pipe (no color)**

Run: `./ccws /tmp/does-not-exist 2>&1 | cat`
Expected: same content but no ANSI escapes (no `\x1b[`).

- [ ] **Step 6: Run all tests**

Run: `task test:race`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/ccws/main.go
git commit -m "ccws: route top-level errors through tui badge renderer"
```

---

## Task 10: Wire `root.go`

**Files:**
- Modify: `cmd/ccws/root.go`

- [ ] **Step 1: Replace success/warning prints**

Edit the `RunE` body in `cmd/ccws/root.go`:

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
			renderSuccess(tui.NewStdout(), res, sourceLabel(res.ColorSource))
			renderWarnings(tui.NewStderr(), res.Warnings)
			return nil
		},
```

Update the imports — remove `fmt` and `os` if no longer used; add `tui`:

```go
import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)
```

(Check whether `errors` and `os` are still referenced elsewhere in the file — `errToExit` uses both. They stay.)

- [ ] **Step 2: Build + test**

Run: `task build && task test`
Expected: PASS.

- [ ] **Step 3: Manual smoke — success**

Run in a clean temp dir:
```bash
mkdir -p /tmp/ccws-smoke && rm -rf /tmp/ccws-smoke/*
./ccws --no-open --color random /tmp/ccws-smoke
```

Expected (color in TTY):
```
  ok     wrote /tmp/ccws-smoke.code-workspace
         color  #XXXXXX (random)
```

Cleanup: `rm -rf /tmp/ccws-smoke /tmp/ccws-smoke.code-workspace`

- [ ] **Step 4: Run lint**

Run: `task lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ccws/root.go
git commit -m "ccws: route success/warning output through tui renderer"
```

---

## Task 11: Wire `interactive.go`

**Files:**
- Modify: `cmd/ccws/interactive.go`

- [ ] **Step 1: Replace prints + use guardDescription**

Edit `cmd/ccws/interactive.go`:

```go
package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/interactive"
	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

func interactiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive [target-dir]",
		Short: "Walk through options interactively (huh form).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args)
		},
	}
}

func runInteractive(args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	choices, err := interactive.Run(abs)
	if err != nil {
		return err
	}

	opts := interactive.ApplyToOptions(*choices, choices.TargetDir)

	for attempt := 0; attempt < 2; attempt++ {
		res, err := runner.New(nil).Run(opts)
		if err == nil {
			// Interactive mode: source is implicit (user just chose it),
			// pass "" to suppress "(from ...)" suffix.
			renderSuccess(tui.NewStdout(), res, "")
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

func confirmGuard(ge *runner.GuardError) (bool, error) {
	title := fmt.Sprintf("Guard %d triggered", ge.Guard)
	desc := guardDescription(ge)
	var proceed bool
	err := huh.NewConfirm().
		Title(title).
		Description(desc).
		Affirmative("Override").
		Negative("Abort").
		Value(&proceed).
		Run()
	return proceed, err
}
```

The `os` and `strings` imports may be removable — check and clean.

- [ ] **Step 2: Build + test**

Run: `task build && task test`
Expected: PASS.

- [ ] **Step 3: Manual smoke — interactive guard**

Set up the same scenario as Task 9 step 4 (a directory with a conflicting `.code-workspace`).

Run: `./ccws interactive <that-dir>`
Walk through the form, then expect a huh confirm with "Guard 1 triggered" title and a description body that contains the file/keys/hint structure (no ANSI badges since huh draws its own chrome).

Choose Abort → exit code 1, error message routed through `renderError` (badge red `error  aborted (guard 1)`).

- [ ] **Step 4: Run lint**

Run: `task lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ccws/interactive.go
git commit -m "ccws: interactive uses tui renderer + guardDescription for confirm body"
```

---

## Task 12: Update CLAUDE.md DAG section

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add tui to the import DAG**

In `CLAUDE.md`, under the "Package import rule" section (around line 60), update the DAG block. Replace:

```
color → (stdlib only)
peacock → (stdlib only)
jsonc → hujson
workspace, vscodesettings → peacock, jsonc
runner → color, workspace, vscodesettings
interactive → runner, vscodesettings
cmd/ccws → runner, interactive
```

with:

```
color → (stdlib only)
peacock → (stdlib only)
jsonc → hujson
tui → lipgloss, isatty
workspace, vscodesettings → peacock, jsonc
runner → color, workspace, vscodesettings
interactive → runner, vscodesettings
cmd/ccws → runner, interactive, tui
```

- [ ] **Step 2: Run final CI**

Run: `task ci`
Expected: PASS (lint + test:race).

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add tui package to CLAUDE.md DAG"
```

---

## Final verification

- [ ] **Step 1: Full CI run**

```bash
task ci
```

Expected: lint clean, tests pass with race detector.

- [ ] **Step 2: Verify the user's reported case**

Reproduce the original bug — run `ccws .` (or whichever directory) where the workspace file already has the 17 peacock keys the user listed.

Expected: a single styled block — red `error` badge, `file`, `keys` (truncated to 8 + `…(9 more)`), `hint`. Exit code 2.

- [ ] **Step 3: Verify NO_COLOR**

```bash
NO_COLOR=1 ./ccws . 2>&1 | head -20
```

Expected: same layout, no ANSI escapes anywhere in the output.

- [ ] **Step 4: Verify pipe behavior**

```bash
./ccws . 2>&1 | cat | head -20
```

Expected: same as step 3 (non-TTY → no color).
