package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/gitworktree"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// ColorSource indicates where the final color came from.
type ColorSource int

const (
	SourceFlag ColorSource = iota + 1
	SourceSettings
	SourceWorktree
	SourceRandom
)

// AnchorIntent describes a side effect requested by the worktree resolver:
// write the given color as the family anchor into the main worktree's
// .code-workspace. Returned only for Case C (linked worktree is the first
// to be ccws'd in this repo). Caller (runner.Run) is responsible for
// executing the write.
type AnchorIntent struct {
	WorkspacePath string
	AnchorColor   color.Color
}

// listWorktreesFn is the package-level injection point for the gitworktree.List
// dependency. Tests reassign it (with cleanup) to inject fixture worktree slices.
var listWorktreesFn = gitworktree.List

// ResolveColor applies the priority rules:
//  1. Explicit --color flag                               → SourceFlag
//  2. Worktree family logic (Case A or C in spec)         → SourceWorktree
//  3. peacock.color in target's .vscode/settings.json     → SourceSettings
//  4. Random                                              → SourceRandom
//
// The third return is informational warnings to be surfaced via Result.Warnings.
// The fourth return is non-nil only for Case C (auto-establish), where the
// caller must write the anchor color into the main worktree's .code-workspace.
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, []string, *AnchorIntent, error) {
	if flag != "" {
		c, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, nil, nil, fmt.Errorf("--color: %w", err)
		}
		return c, SourceFlag, nil, nil, nil
	}

	c, src, warns, intent, ok, err := resolveFromWorktree(targetDir)
	if err != nil {
		return color.Color{}, 0, nil, nil, err
	}
	if ok {
		return c, src, warns, intent, nil
	}

	// fall through to existing chain — preserve any Case-D warnings
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, warns, nil, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			c, err := color.Parse(pc)
			if err != nil {
				return color.Color{}, 0, warns, nil, fmt.Errorf("peacock.color in settings: %w", err)
			}
			return c, SourceSettings, warns, nil, nil
		}
	}
	return color.Random(), SourceRandom, warns, nil, nil
}

// readWorkspacePeacockColor parses the workspace file at path and returns
// the peacock.color setting. Returns (nil, nil) when:
//   - file does not exist
//   - file has no settings block
//   - settings has no peacock.color key
//   - peacock.color is not a parseable color (treated as missing)
func readWorkspacePeacockColor(path string) (*color.Color, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	ws, err := workspace.Read(path)
	if err != nil {
		return nil, err
	}
	if ws == nil || ws.Settings == nil {
		return nil, nil
	}
	raw, ok := ws.Settings["peacock.color"]
	if !ok {
		return nil, nil
	}
	hex, ok := raw.(string)
	if !ok {
		return nil, nil
	}
	c, err := color.Parse(hex)
	if err != nil {
		return nil, nil
	}
	return &c, nil
}

// resolveFromWorktree consults the worktree context. Return tuple semantics:
//
//	ok=true  → color decided by worktree logic; caller uses (c, src, warns, intent)
//	ok=false → fall through to settings/random; warns may carry a Case-D notice
//	err!=nil → hard error (e.g., file write failure for AnchorIntent)
func resolveFromWorktree(targetDir string) (color.Color, ColorSource, []string, *AnchorIntent, bool, error) {
	worktrees, err := listWorktreesFn(targetDir)
	if errors.Is(err, gitworktree.ErrNotInWorktree) {
		return color.Color{}, 0, nil, nil, false, nil
	}
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	self := gitworktree.FindSelf(worktrees, targetDir)
	if self == nil {
		return color.Color{}, 0, nil, nil, false, nil
	}
	main := worktrees[0]
	mainWsPath, err := workspaceFilePath(main.Path)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}
	mainColor, err := readWorkspacePeacockColor(mainWsPath)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}

	// Case A: main has a color — anchor + offset
	if mainColor != nil {
		offset := color.LadderOffset(gitworktree.IdentityHash(*self))
		return mainColor.ApplyLightness(offset), SourceWorktree, nil, nil, true, nil
	}

	// main has no color — check whether any other linked worktree has one
	linked, linkedColor, err := findLinkedWithColor(worktrees, self)
	if err != nil {
		return color.Color{}, 0, nil, nil, false, err
	}

	// Case D: linked has color but main does not — refuse to derive a family
	if linked != nil {
		warn := formatFamilyDisabledWarning(linked, linkedColor, main, mainWsPath)
		return color.Color{}, 0, []string{warn}, nil, false, nil
	}

	// Cases B/C land in Task 9 — for now, fall through.
	return color.Color{}, 0, nil, nil, false, nil
}

// findLinkedWithColor returns the first non-main worktree (excluding self)
// whose .code-workspace has a peacock.color, along with that color.
func findLinkedWithColor(worktrees []gitworktree.Worktree, self *gitworktree.Worktree) (*gitworktree.Worktree, *color.Color, error) {
	for i := range worktrees {
		w := &worktrees[i]
		if w.IsMain || w.Path == self.Path {
			continue
		}
		wsPath, err := workspaceFilePath(w.Path)
		if err != nil {
			return nil, nil, err
		}
		c, err := readWorkspacePeacockColor(wsPath)
		if err != nil {
			return nil, nil, err
		}
		if c != nil {
			return w, c, nil
		}
	}
	return nil, nil, nil
}

func formatFamilyDisabledWarning(linked *gitworktree.Worktree, linkedColor *color.Color, main gitworktree.Worktree, mainWsPath string) string {
	return fmt.Sprintf(
		"worktree family disabled\n"+
			"  reason     main worktree is uncolored, but linked has color\n"+
			"  linked     %s  %s\n"+
			"  main       %s  (no color)\n"+
			"  hint       set main color first: ccws --color '%s' %s",
		filepath.Base(linked.Path), linkedColor.Hex(),
		main.Path,
		linkedColor.Hex(), main.Path,
	)
}
