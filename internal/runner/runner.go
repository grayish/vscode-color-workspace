package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// GuardError indicates a safety guard triggered. Exit code 2.
// Carries data only; presentation is the CLI layer's responsibility.
type GuardError struct {
	Guard int
	Path  string   // path to the file containing the offending keys
	Keys  []string // residual keys
}

// Error returns a single-line summary used by %v, log lines, and errors.As
// fallbacks. The full multi-line presentation is rendered by cmd/ccws.
func (e *GuardError) Error() string {
	return fmt.Sprintf("guard %d: %d residual keys in %s", e.Guard, len(e.Keys), e.Path)
}

// Result is the output of a successful Run.
type Result struct {
	WorkspaceFile   string
	ColorHex        string
	ColorSource     ColorSource
	SettingsCleaned bool
	Preconfigured   bool     // true when ws already had peacock keys and Force=false; nothing was written
	PeacockKeys     []string // existing peacock keys detected on Preconfigured short-circuit (sorted, dotted paths)
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

	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")

	ws, err := workspace.Read(wsPath)
	if err != nil {
		return nil, err
	}

	// Short-circuit: existing peacock workspace, no force → skip everything,
	// just open. Guard 2 is intentionally not checked on this path.
	if ws != nil && !opts.Force {
		if keys := workspace.ExistingPeacockKeys(ws); len(keys) > 0 {
			res := &Result{
				WorkspaceFile: wsPath,
				Preconfigured: true,
				PeacockKeys:   keys,
			}
			if !opts.NoOpen {
				if err := r.Opener.Open(wsPath); err != nil {
					if errors.Is(err, ErrCodeNotFound) {
						res.Warnings = append(res.Warnings, "code CLI not on PATH; open manually: "+wsPath)
					} else {
						res.Warnings = append(res.Warnings, "failed to open with code: "+err.Error())
					}
				}
			}
			return res, nil
		}
	}

	c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput)
	if err != nil {
		return nil, err
	}
	_ = anchorIntent // wired in Task 10

	settingsPath := filepath.Join(abs, ".vscode", "settings.json")
	srcSettings, err := vscodesettings.Read(settingsPath)
	if err != nil {
		return nil, err
	}
	willClean := !opts.KeepSource && srcSettings != nil
	if willClean && !opts.Force {
		if keys := vscodesettings.ResidualColorKeys(srcSettings); len(keys) > 0 {
			return nil, &GuardError{Guard: 2, Path: settingsPath, Keys: keys}
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

	warnings := append([]string(nil), resolveWarns...)
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

// CheckPreconfigured reports whether target/<...>.code-workspace already
// contains peacock keys. Returns the absolute workspace file path and the
// detected keys when present; otherwise returns ("", nil, nil). Filesystem
// or parse errors propagate to the caller.
//
// Used by interactive Phase A to detect whether to show the "already
// configured" prompt before invoking the form.
func CheckPreconfigured(target string) (string, []string, error) {
	wsPath, err := workspaceFilePath(target)
	if err != nil {
		return "", nil, err
	}
	ws, err := workspace.Read(wsPath)
	if err != nil {
		return "", nil, err
	}
	if ws == nil {
		return "", nil, nil
	}
	keys := workspace.ExistingPeacockKeys(ws)
	if len(keys) == 0 {
		return "", nil, nil
	}
	return wsPath, keys, nil
}

// workspaceFilePath returns the absolute path of the <parent>/<folder>.code-workspace
// file for target without touching the filesystem.
func workspaceFilePath(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(abs), filepath.Base(abs)+".code-workspace"), nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
