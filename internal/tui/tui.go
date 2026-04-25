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
