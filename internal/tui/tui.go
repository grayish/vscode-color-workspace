// Package tui renders CLI output: badge headers, detail rows, bullet lists.
package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
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

// NewStdout returns a Writer over os.Stdout, with color enabled when stdout is
// a TTY, NO_COLOR is unset, and TERM != "dumb".
func NewStdout() *Writer {
	return &Writer{out: os.Stdout, color: shouldColor(os.Stdout.Fd())}
}

// NewStderr is NewStdout for os.Stderr.
func NewStderr() *Writer {
	return &Writer{out: os.Stderr, color: shouldColor(os.Stderr.Fd())}
}

// Newline writes a single blank line. Used between adjacent badge blocks.
func (w *Writer) Newline() { fmt.Fprintln(w.out) }

func shouldColor(fd uintptr) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(fd)
}

const (
	leadingIndent  = "  "
	badgeWidth     = 5 // len("error"), longest of ok/warn/error
	badgeSeparator = "  "
)

// continuationIndent is the leading whitespace for rows under a badge.
// Width = leadingIndent + badgeWidth + badgeSeparator = 2+5+2 = 9.
var continuationIndent = strings.Repeat(" ", len(leadingIndent)+badgeWidth+len(badgeSeparator))

// renderer forces lipgloss to emit ANSI escapes regardless of where the
// resulting string is eventually written (e.g. a *bytes.Buffer in tests).
// Style.Render(s) string does not actually write to renderer.Writer; the
// io.Discard target is purely a placeholder.
var renderer = lipgloss.NewRenderer(io.Discard, termenv.WithProfile(termenv.ANSI), termenv.WithTTY(true))

var (
	styleOK = renderer.NewStyle().
		Background(lipgloss.Color("10")).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Width(badgeWidth)

	styleWarn = renderer.NewStyle().
			Background(lipgloss.Color("11")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Width(badgeWidth)

	styleError = renderer.NewStyle().
			Background(lipgloss.Color("9")).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Width(badgeWidth)
)

// OK writes a green "ok" badge line.
func (w *Writer) OK(title string) { w.badge(styleOK, "ok", title) }

// Warn writes a yellow "warn" badge line.
func (w *Writer) Warn(title string) { w.badge(styleWarn, "warn", title) }

// Error writes a red "error" badge line.
func (w *Writer) Error(title string) { w.badge(styleError, "error", title) }

func (w *Writer) badge(style lipgloss.Style, label, title string) {
	cell := w.renderBadge(style, label)
	fmt.Fprintf(w.out, "%s%s%s%s\n", leadingIndent, cell, badgeSeparator, title)
}

// renderBadge returns the badge cell, padded to badgeWidth.
func (w *Writer) renderBadge(style lipgloss.Style, label string) string {
	if !w.color {
		return label + strings.Repeat(" ", max(0, badgeWidth-len(label)))
	}
	return style.Render(label)
}

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

// bulletIndent is continuation indent + 2 spaces for the bullet glyph.
var bulletIndent = continuationIndent + "  "

// Bullets writes up to limit items as bulleted lines. When len(items) > limit,
// the first limit items are written and a final "…(N more)" line is appended.
// limit <= 0 disables truncation.
func (w *Writer) Bullets(items []string, limit int) {
	truncated := limit > 0 && len(items) > limit
	shown := items
	if truncated {
		shown = items[:limit]
	}
	for _, it := range shown {
		fmt.Fprintf(w.out, "%s• %s\n", bulletIndent, it)
	}
	if truncated {
		fmt.Fprintf(w.out, "%s…(%d more)\n", bulletIndent, len(items)-limit)
	}
}

// ShortenPath replaces a leading $HOME with "~". Trailing slashes on $HOME are
// trimmed before comparison, so HOME=/Users/x and HOME=/Users/x/ behave the
// same. If HOME is unset (or trims to empty, e.g. HOME=/) or p does not begin
// with $HOME (treating $HOME as a directory boundary), p is returned unchanged.
// Examples (HOME=/Users/x):
//
//	"/Users/x"      → "~"
//	"/Users/x/p"    → "~/p"
//	"/Users/xy/p"   → "/Users/xy/p"  (sibling, not a child of HOME)
//	"/tmp/p"        → "/tmp/p"
func ShortenPath(p string) string {
	home := strings.TrimRight(os.Getenv("HOME"), "/")
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
