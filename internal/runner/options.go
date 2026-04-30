// Package runner orchestrates the full ccws flow: resolve color, generate
// palette, place the workspace file with guards, clean up source, open.
package runner

import "github.com/sang-bin/vscode-color-workspace/internal/color"

// Options is the full runner input.
type Options struct {
	TargetDir  string
	ColorInput string // raw --color flag value; empty = auto
	NoOpen     bool
	Force      bool
	KeepSource bool
	Debug      bool // when true, the runner writes [debug] lines to stderr
	Palette    color.Options
}

// Defaults returns sensible default Options. TargetDir must be filled by caller.
func Defaults() Options {
	return Options{
		Palette: color.DefaultOptions(),
	}
}
