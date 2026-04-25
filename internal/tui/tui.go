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
