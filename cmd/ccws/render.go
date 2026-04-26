package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

const maxBulletsShown = 8

// renderError dispatches by error type.
func renderError(w *tui.Writer, err error) {
	var ge *runner.GuardError
	if errors.As(err, &ge) {
		w.Error(guardTitle(ge))
		writeGuardBody(w, ge)
		return
	}
	w.Error(err.Error())
}

// guardDescription returns the guard body as plain text. The huh confirm
// dialog draws its own border, so no badge or ANSI.
func guardDescription(ge *runner.GuardError) string {
	var buf bytes.Buffer
	writeGuardBody(tui.NewWriter(&buf, false), ge)
	return buf.String()
}

func writeGuardBody(w *tui.Writer, ge *runner.GuardError) {
	w.Details([]tui.Detail{
		{Label: "file", Value: tui.ShortenPath(ge.Path)},
		{Label: "keys"},
	})
	w.Bullets(ge.Keys, maxBulletsShown)
	w.Details([]tui.Detail{{Label: "hint", Value: guardHint(ge)}})
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

// renderSuccess writes the success block. Empty srcLabel suppresses the
// "(...)" suffix on the color row.
func renderSuccess(w *tui.Writer, res *runner.Result, srcLabel string) {
	w.OK("wrote " + tui.ShortenPath(res.WorkspaceFile))
	value := res.ColorHex
	if srcLabel != "" {
		value += " (" + srcLabel + ")"
	}
	w.Details([]tui.Detail{{Label: "color", Value: value}})
}

func renderWarnings(w *tui.Writer, warnings []string) {
	for i, msg := range warnings {
		if i > 0 {
			w.Newline()
		}
		w.Warn(msg)
	}
}

// renderPreconfigured writes the warn block for the short-circuit case
// where the workspace already has peacock keys and ccws skipped the write.
func renderPreconfigured(w *tui.Writer, res *runner.Result) {
	w.Warn("workspace already configured")
	const colWidth = 12 // len("peacock keys"), the longest label
	w.Details([]tui.Detail{
		{Label: fmt.Sprintf("%-*s", colWidth, "workspace"), Value: tui.ShortenPath(res.WorkspaceFile)},
		{Label: fmt.Sprintf("%-*s", colWidth, "peacock keys"), Value: fmt.Sprintf("%d existing", len(res.PeacockKeys))},
		{Label: fmt.Sprintf("%-*s", colWidth, "hint"), Value: "use --force to overwrite (other flags ignored)"},
	})
}
