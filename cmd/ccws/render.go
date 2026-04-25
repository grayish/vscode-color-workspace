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
