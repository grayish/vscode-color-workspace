package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// GuardError indicates a safety guard triggered. Exit code 2.
type GuardError struct {
	Guard   int
	Message string
	Keys    []string
}

func (e *GuardError) Error() string {
	return e.Message
}

// Result is the output of a successful Run.
type Result struct {
	WorkspaceFile   string
	ColorHex        string
	ColorSource     ColorSource
	SettingsCleaned bool
	Warnings        []string
}

// Runner orchestrates the full flow.
type Runner struct {
	Opener Opener
}

// New returns a Runner using opener. If opener is nil, CodeOpener is used.
func New(opener Opener) *Runner {
	if opener == nil {
		opener = CodeOpener{}
	}
	return &Runner{Opener: opener}
}

// Run executes the full pipeline.
func (r *Runner) Run(opts Options) (*Result, error) {
	info, err := os.Stat(opts.TargetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("target does not exist: %s", opts.TargetDir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target is not a directory: %s", opts.TargetDir)
	}
	abs, err := filepath.Abs(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	c, src, err := ResolveColor(abs, opts.ColorInput)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")

	ws, err := workspace.Read(wsPath)
	if err != nil {
		return nil, err
	}
	if ws != nil && !opts.Force {
		if keys := workspace.ExistingPeacockKeys(ws); len(keys) > 0 {
			return nil, &GuardError{
				Guard: 1,
				Keys:  keys,
				Message: fmt.Sprintf(
					"existing peacock color settings in %s: %s\nrerun with --force to overwrite",
					wsPath, strings.Join(keys, ", ")),
			}
		}
	}

	settingsPath := filepath.Join(abs, ".vscode", "settings.json")
	srcSettings, err := vscodesettings.Read(settingsPath)
	if err != nil {
		return nil, err
	}
	willClean := !opts.KeepSource && srcSettings != nil
	if willClean && !opts.Force {
		if keys := vscodesettings.ResidualColorKeys(srcSettings); len(keys) > 0 {
			return nil, &GuardError{
				Guard: 2,
				Keys:  keys,
				Message: fmt.Sprintf(
					"non-peacock workbench.colorCustomizations would remain in %s: %s\nremove those keys manually or rerun with --force",
					settingsPath, strings.Join(keys, ", ")),
			}
		}
	}

	palette := color.Palette(c, opts.Palette)
	colorHex := c.Hex()

	if ws == nil {
		ws = &workspace.Workspace{}
	}
	workspace.EnsureFolder(ws, "./"+folderName)
	workspace.ApplyPeacock(ws, colorHex, palette)
	if err := workspace.Write(wsPath, ws); err != nil {
		return nil, err
	}

	cleaned := false
	if willClean {
		if vscodesettings.Cleanup(srcSettings) {
			if err := vscodesettings.WriteOrDelete(srcSettings); err != nil {
				return nil, err
			}
			cleaned = true
		}
	}

	var warnings []string
	if isGitRepo(parent) {
		warnings = append(warnings,
			fmt.Sprintf("parent directory %s is a git repository; workspace file may be committed", parent))
	}

	if !opts.NoOpen {
		if err := r.Opener.Open(wsPath); err != nil {
			if errors.Is(err, ErrCodeNotFound) {
				warnings = append(warnings, "code CLI not on PATH; open manually: "+wsPath)
			} else {
				warnings = append(warnings, "failed to open with code: "+err.Error())
			}
		}
	}

	return &Result{
		WorkspaceFile:   wsPath,
		ColorHex:        colorHex,
		ColorSource:     src,
		SettingsCleaned: cleaned,
		Warnings:        warnings,
	}, nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
