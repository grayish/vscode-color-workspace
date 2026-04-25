// Package tui renders CLI output: badge headers, detail rows, bullet lists.
package tui

import (
	"fmt"
	"io"
	"strings"
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

const (
	leadingIndent  = "  "
	badgeWidth     = 5 // len("error"), longest of ok/warn/error
	badgeSeparator = "  "
)

// continuationIndent is the leading whitespace for rows under a badge.
// Width = leadingIndent + badgeWidth + badgeSeparator = 2+5+2 = 9.
var continuationIndent = strings.Repeat(" ", len(leadingIndent)+badgeWidth+len(badgeSeparator))

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
// Color styling is added in Task 5.
func (w *Writer) renderBadge(label string) string {
	return label + strings.Repeat(" ", max(0, badgeWidth-len(label)))
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
