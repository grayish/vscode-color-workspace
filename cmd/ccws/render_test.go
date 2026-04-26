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

func TestRenderError_Truncates(t *testing.T) {
	keys := make([]string, 17)
	for i := range keys {
		keys[i] = "k" + string(rune('0'+i%10))
	}
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	renderError(w, &runner.GuardError{Guard: 2, Path: "/tmp/x", Keys: keys})
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
	ge := &runner.GuardError{Guard: 2, Path: "/tmp/x", Keys: []string{"a", "b"}}
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

func TestRenderPreconfigured_PlainOutput(t *testing.T) {
	var buf bytes.Buffer
	w := tui.NewWriter(&buf, false)
	res := &runner.Result{
		WorkspaceFile: "/tmp/foo.code-workspace",
		Preconfigured: true,
		PeacockKeys:   []string{"settings.peacock.color", "settings.workbench.colorCustomizations.activityBar.background", "settings.workbench.colorCustomizations.titleBar.activeBackground"},
	}
	renderPreconfigured(w, res)
	got := buf.String()
	for _, want := range []string{
		"  warn   workspace already configured\n",
		"         workspace     /tmp/foo.code-workspace\n",
		"         peacock keys  3 existing\n",
		"         hint          use --force to overwrite (other flags ignored)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fragment %q in output:\n%s", want, got)
		}
	}
}
