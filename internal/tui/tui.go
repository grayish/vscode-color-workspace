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
	out    io.Writer
	color  bool
	styles badgeStyles
}

// NewWriter returns a Writer over out. color enables lipgloss styling; pass
// false for tests, plain logs, or non-TTY output.
func NewWriter(out io.Writer, color bool) *Writer {
	w := &Writer{out: out, color: color}
	if color {
		r := lipgloss.NewRenderer(out, termenv.WithProfile(termenv.ANSI), termenv.WithTTY(true))
		w.styles = newBadgeStyles(r)
	}
	return w
}

// NewStdout returns a Writer over os.Stdout, with color enabled when stdout is
// a TTY, NO_COLOR is unset, and TERM != "dumb".
func NewStdout() *Writer {
	return NewWriter(os.Stdout, shouldColor(os.Stdout.Fd()))
}

// NewStderr is NewStdout for os.Stderr.
func NewStderr() *Writer {
	return NewWriter(os.Stderr, shouldColor(os.Stderr.Fd()))
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

const (
	leadingIndent  = "  "
	badgeWidth     = 5 // len("error"), longest of ok/warn/error
	badgeSeparator = "  "
)

// continuationIndent is the leading whitespace for rows under a badge.
// Width = leadingIndent + badgeWidth + badgeSeparator = 2+5+2 = 9.
var continuationIndent = strings.Repeat(" ", len(leadingIndent)+badgeWidth+len(badgeSeparator))

// badgeStyles holds pre-built lipgloss styles for a given Renderer.
type badgeStyles struct {
	ok    lipgloss.Style
	warn  lipgloss.Style
	error lipgloss.Style
}

// newBadgeStyles creates badge styles bound to r so ANSI output is emitted
// regardless of whether the underlying writer is a TTY.
func newBadgeStyles(r *lipgloss.Renderer) badgeStyles {
	return badgeStyles{
		ok: r.NewStyle().
			Background(lipgloss.Color("10")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Width(badgeWidth),
		warn: r.NewStyle().
			Background(lipgloss.Color("11")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Width(badgeWidth),
		error: r.NewStyle().
			Background(lipgloss.Color("9")).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Width(badgeWidth),
	}
}

// OK writes a green "ok" badge line.
func (w *Writer) OK(title string) { w.badge("ok", title) }

// Warn writes a yellow "warn" badge line.
func (w *Writer) Warn(title string) { w.badge("warn", title) }

// Error writes a red "error" badge line.
func (w *Writer) Error(title string) { w.badge("error", title) }

func (w *Writer) badge(label, title string) {
	cell := w.renderBadge(label)
	fmt.Fprintf(w.out, "%s%s%s%s\n", leadingIndent, cell, badgeSeparator, title)
}

// renderBadge returns the badge cell, padded to badgeWidth.
func (w *Writer) renderBadge(label string) string {
	if !w.color {
		return label + strings.Repeat(" ", max(0, badgeWidth-len(label)))
	}
	switch label {
	case "ok":
		return w.styles.ok.Render(label)
	case "warn":
		return w.styles.warn.Render(label)
	case "error":
		return w.styles.error.Render(label)
	default:
		return label + strings.Repeat(" ", max(0, badgeWidth-len(label)))
	}
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

// Bullets writes up to max items as bulleted lines. When len(items) > max,
// the first max items are written and a final "…(N more)" line is appended.
// max <= 0 disables truncation.
func (w *Writer) Bullets(items []string, max int) {
	truncated := max > 0 && len(items) > max
	shown := items
	if truncated {
		shown = items[:max]
	}
	for _, it := range shown {
		fmt.Fprintf(w.out, "%s• %s\n", bulletIndent, it)
	}
	if truncated {
		fmt.Fprintf(w.out, "%s…(%d more)\n", bulletIndent, len(items)-max)
	}
}
