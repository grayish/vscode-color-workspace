package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
)

func guardErr() *runner.GuardError {
	return &runner.GuardError{
		Guard: 2,
		Path:  "/proj/.vscode/settings.json",
		Keys:  []string{"editor.fontSize"},
	}
}

// errToExit is the CLI's published contract: 0 success, 1 input error or A2
// partial propagation, 2 guard triggered, 3 filesystem error.
func TestErrToExit(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, 0},
		{"guard error", guardErr(), 2},
		{"wrapped guard error", fmt.Errorf("run: %w", guardErr()), 2},
		{"permission denied", os.ErrPermission, 3},
		{"wrapped permission denied", fmt.Errorf("write: %w", os.ErrPermission), 3},
		{"PathError carrying permission", &fs.PathError{Op: "open", Path: "/proj", Err: os.ErrPermission}, 3},
		{"partial propagation", runner.ErrPartialPropagation, 1},
		{"wrapped partial propagation", fmt.Errorf("a2: %w", runner.ErrPartialPropagation), 1},
		{"unclassified error", errors.New("boom"), 1},
		{"not-exist is not a permission error", fs.ErrNotExist, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errToExit(tt.err); got != tt.want {
				t.Errorf("errToExit(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// A guard that surfaces alongside a permission failure still exits 2: the
// guard is the actionable diagnosis, the filesystem cause is incidental.
func TestErrToExit_GuardOutranksPermission(t *testing.T) {
	err := errors.Join(guardErr(), os.ErrPermission)
	if got := errToExit(err); got != 2 {
		t.Errorf("errToExit(guard+permission) = %d, want 2", got)
	}
}

func TestSourceLabel(t *testing.T) {
	tests := []struct {
		src  runner.ColorSource
		want string
	}{
		{runner.SourceFlag, "from --color"},
		{runner.SourceSettings, "inherited from .vscode/settings.json"},
		{runner.SourceWorktree, "from worktree family"},
		{runner.SourceRandom, "random"},
		{runner.ColorSource(0), "?"},
		{runner.ColorSource(99), "?"},
	}
	for _, tt := range tests {
		if got := sourceLabel(tt.src); got != tt.want {
			t.Errorf("sourceLabel(%d) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

// Smoke-tests the flag plumbing: --color and --no-open must reach Options and
// produce the workspace file at <parent>/<dirname>.code-workspace. --no-open
// keeps the test from launching the `code` CLI.
func TestRootCmd_WritesWorkspaceWithColorFlag(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--color", "#5a3b8c", "--no-open", target})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want nil", err)
	}

	wsPath := filepath.Join(parent, "myproj.code-workspace")
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatalf("workspace file not written: %v", err)
	}
	var ws struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(data, &ws); err != nil {
		t.Fatalf("workspace is not valid JSON: %v", err)
	}
	if ws.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v, want #5a3b8c", ws.Settings["peacock.color"])
	}
}

// An unparseable --color is an input error, which errToExit maps to 1 — not a
// guard (2) and not a filesystem error (3).
func TestRootCmd_InvalidColorIsInputError(t *testing.T) {
	target := t.TempDir()
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--color", "not-a-color", "--no-open", target})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() = nil, want an error for an invalid color")
	}
	if got := errToExit(err); got != 1 {
		t.Errorf("errToExit(%v) = %d, want 1", err, got)
	}
}
