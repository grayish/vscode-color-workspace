package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/gitworktree"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// dbg writes a [debug] line to stderr when enabled is true. No-op otherwise.
func dbg(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
}

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

// PropagateIntent describes A2 side effects: write the anchor color to main's
// .code-workspace, then write each derived color to the corresponding linked
// worktree's .code-workspace. The runner executes the writes; resolve only
// computes the targets and skip list.
type PropagateIntent struct {
	AnchorPath  string // ws(main)
	AnchorColor color.Color
	Targets     []PropagateTarget
	Skipped     []SkippedLinked
}

// PropagateTarget is a linked worktree with its derived color.
type PropagateTarget struct {
	WorkspacePath string
	DerivedColor  color.Color
}

// SkippedLinked is a linked worktree that was not in the family (no peacock
// keys, no .code-workspace, parse error, etc.). The reason is short text
// suitable for display.
type SkippedLinked struct {
	WorkspacePath string
	Reason        string
}

// PropagateFailure is a linked worktree where the write attempt failed at
// runtime (permission denied, disk full, etc.).
type PropagateFailure struct {
	WorkspacePath string
	Err           error
}

// listWorktreesFn is the package-level injection point for the gitworktree.List
// dependency. Tests reassign it (with cleanup) to inject fixture worktree slices.
// Tests that reassign this var must not call t.Parallel().
var listWorktreesFn = gitworktree.List

// ResolveColor applies the priority rules:
//  1. Explicit --color flag (unless A2 consumes it as anchor) → SourceFlag
//  2. Worktree family logic (Cases A1/A2/A3/B/C/D in spec)   → SourceWorktree
//  3. peacock.color in target's .vscode/settings.json         → SourceSettings
//  4. Random                                                  → SourceRandom
//
// The third return is informational warnings to be surfaced via Result.Warnings.
// The fourth return (*AnchorIntent) is non-nil only for Case C (auto-establish).
// The fifth return (*PropagateIntent) is non-nil only for Case A2 (multi-worktree
// main + force). The caller (runner.Run) is responsible for executing writes.
// When debug is true, branch-by-branch diagnostics are written to stderr.
func ResolveColor(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, *PropagateIntent, error) {
	dbg(debug, "ResolveColor: targetDir=%q flag=%q force=%v", targetDir, flag, force)

	var parsedFlag color.Color
	if flag != "" {
		p, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, nil, nil, nil, fmt.Errorf("--color: %w", err)
		}
		parsedFlag = p
	}

	c, src, warns, anchorIntent, propagateIntent, ok, err := resolveFromWorktree(targetDir, flag, force, debug)
	if err != nil {
		return color.Color{}, 0, nil, nil, nil, err
	}
	if ok {
		dbg(debug, "ResolveColor: worktree logic decided source=%v color=%s", src, c.Hex())
		return c, src, warns, anchorIntent, propagateIntent, nil
	}
	dbg(debug, "ResolveColor: worktree logic skipped — falling through")

	if flag != "" {
		dbg(debug, "ResolveColor: source=Flag color=%s", parsedFlag.Hex())
		return parsedFlag, SourceFlag, warns, nil, nil, nil
	}

	// fall through to settings.json → random — preserve any Case-D warnings
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, warns, nil, nil, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			parsed, perr := color.Parse(pc)
			if perr != nil {
				return color.Color{}, 0, warns, nil, nil, fmt.Errorf("peacock.color in settings: %w", perr)
			}
			dbg(debug, "ResolveColor: source=Settings color=%s", parsed.Hex())
			return parsed, SourceSettings, warns, nil, nil, nil
		}
	}
	rc := color.Random()
	dbg(debug, "ResolveColor: source=Random color=%s", rc.Hex())
	return rc, SourceRandom, warns, nil, nil, nil
}

// readWorkspacePeacockColor parses the workspace file at path and returns
// the peacock.color setting. Returns (nil, nil) when:
//   - file does not exist
//   - file has no settings block
//   - settings has no peacock.color key
//   - peacock.color is not a parseable color (treated as missing)
func readWorkspacePeacockColor(path string) (*color.Color, error) {
	ws, err := workspace.Read(path)
	if err != nil {
		return nil, err
	}
	hex, ok := ws.PeacockColor()
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
//	ok=true  → color decided by worktree logic; caller uses (c, src, warns, anchorIntent, propagateIntent)
//	ok=false → fall through to settings/random; warns may carry a Case-D notice
//	err!=nil → hard error (e.g., file write failure for AnchorIntent)
func resolveFromWorktree(targetDir, flag string, force, debug bool) (color.Color, ColorSource, []string, *AnchorIntent, *PropagateIntent, bool, error) {
	dbg(debug, "resolveFromWorktree: targetDir=%q", targetDir)
	worktrees, err := listWorktreesFn(targetDir)
	if errors.Is(err, gitworktree.ErrNotInWorktree) {
		dbg(debug, "  gitworktree.List → ErrNotInWorktree (wrapped: %v)", err)
		return color.Color{}, 0, nil, nil, nil, false, nil
	}
	if err != nil {
		dbg(debug, "  gitworktree.List → hard error: %v", err)
		return color.Color{}, 0, nil, nil, nil, false, err
	}
	dbg(debug, "  gitworktree.List → %d worktrees", len(worktrees))
	for i, w := range worktrees {
		dbg(debug, "    [%d] path=%q gitDir=%q branch=%q isMain=%v", i, w.Path, w.GitDir, w.Branch, w.IsMain)
	}
	self := gitworktree.FindSelf(worktrees, targetDir)
	if self == nil {
		dbg(debug, "  FindSelf(%q) → nil (no worktree path matched target)", targetDir)
		return color.Color{}, 0, nil, nil, nil, false, nil
	}
	dbg(debug, "  FindSelf → path=%q isMain=%v", self.Path, self.IsMain)

	main := worktrees[0]
	mainWsPath, err := workspaceFilePath(main.Path)
	if err != nil {
		dbg(debug, "  workspaceFilePath(main=%q) error: %v", main.Path, err)
		return color.Color{}, 0, nil, nil, nil, false, err
	}
	dbg(debug, "  main worktree: path=%q wsPath=%q", main.Path, mainWsPath)

	mainColor, err := readWorkspacePeacockColor(mainWsPath)
	if err != nil {
		dbg(debug, "  readWorkspacePeacockColor(%q) error: %v", mainWsPath, err)
		return color.Color{}, 0, nil, nil, nil, false, err
	}

	// Case A1: target is the only worktree (regular git repo, no linked).
	// "Family" doesn't apply — fall through to settings/random.
	if mainColor != nil && self.IsMain && len(worktrees) == 1 {
		dbg(debug, "  Case A1: single-worktree main — skip worktree logic")
		return color.Color{}, 0, nil, nil, nil, false, nil
	}

	// Case A2: target is main of a multi-worktree repo and --force given.
	// Regenerate anchor and propagate to all colored linked worktrees.
	if mainColor != nil && self.IsMain && len(worktrees) > 1 && force {
		var anchor color.Color
		if flag != "" {
			parsed, perr := color.Parse(flag)
			if perr != nil {
				return color.Color{}, 0, nil, nil, nil, false, fmt.Errorf("--color: %w", perr)
			}
			anchor = parsed
		} else {
			anchor = color.Random()
		}
		targets, skipped := buildPropagateTargets(worktrees, anchor)
		intent := &PropagateIntent{
			AnchorPath:  mainWsPath,
			AnchorColor: anchor,
			Targets:     targets,
			Skipped:     skipped,
		}
		dbg(debug, "  Case A2: anchor=%s targets=%d skipped=%d", anchor.Hex(), len(targets), len(skipped))
		return anchor, SourceWorktree, nil, nil, intent, true, nil
	}

	// Case A: main has a color — anchor + offset (A3 in spec terminology)
	if mainColor != nil {
		if flag != "" {
			// --color flag bypasses worktree logic for non-A2 paths
			dbg(debug, "  Case A: flag set, skipping worktree derivation")
			return color.Color{}, 0, nil, nil, nil, false, nil
		}
		offset := color.LadderOffset(gitworktree.IdentityHash(*self))
		derived := mainColor.ApplyLightness(offset)
		dbg(debug, "  Case A: mainColor=%s offset=%v derived=%s", mainColor.Hex(), offset, derived.Hex())
		return derived, SourceWorktree, nil, nil, nil, true, nil
	}
	dbg(debug, "  main worktree has no peacock.color (or wsfile missing): %s", mainWsPath)

	// main has no color — check whether any other linked worktree has one
	linked, linkedColor, err := findLinkedWithColor(worktrees, self)
	if err != nil {
		dbg(debug, "  findLinkedWithColor error: %v", err)
		return color.Color{}, 0, nil, nil, nil, false, err
	}

	// Case D: linked has color but main does not — refuse to derive a family
	if linked != nil {
		dbg(debug, "  Case D: linked=%q has color=%s; main empty → family disabled", linked.Path, linkedColor.Hex())
		warn := formatFamilyDisabledWarning(linked, linkedColor, main, mainWsPath)
		return color.Color{}, 0, []string{warn}, nil, nil, false, nil
	}

	// Case B: main is target and has no color, no linked has color either —
	// fall through to existing chain (settings.json → random). No warning.
	if self.IsMain {
		dbg(debug, "  Case B: target is main, no color anywhere → fall through")
		return color.Color{}, 0, nil, nil, nil, false, nil
	}

	// Case C: target is linked, no other worktree has color — auto-establish
	// main as the family anchor with a random color. The runner executes
	// the side effect (writeAnchorWorkspace) using the returned AnchorIntent.
	if flag != "" {
		// --color flag bypasses worktree logic for non-A2 paths
		dbg(debug, "  Case C: flag set, skipping anchor auto-creation")
		return color.Color{}, 0, nil, nil, nil, false, nil
	}
	anchor := color.Random()
	anchorIntent := &AnchorIntent{
		WorkspacePath: mainWsPath,
		AnchorColor:   anchor,
	}
	selfWsPath, err := workspaceFilePath(self.Path)
	if err != nil {
		dbg(debug, "  workspaceFilePath(self=%q) error: %v", self.Path, err)
		return color.Color{}, 0, nil, nil, nil, false, err
	}
	offset := color.LadderOffset(gitworktree.IdentityHash(*self))
	derived := anchor.ApplyLightness(offset)
	dbg(debug, "  Case C: anchor=%s wsPath=%q self=%q offset=%v derived=%s", anchor.Hex(), mainWsPath, selfWsPath, offset, derived.Hex())
	warn := formatAnchorCreatedWarning(mainWsPath, selfWsPath)
	return derived, SourceWorktree, []string{warn}, anchorIntent, nil, true, nil
}

func formatAnchorCreatedWarning(mainWsPath, selfWsPath string) string {
	return fmt.Sprintf(
		"family anchor created for main worktree\n"+
			"  anchor at  %s\n"+
			"  applied    %s\n"+
			"  hint       run ccws on main worktree to claim color directly",
		mainWsPath, selfWsPath,
	)
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

// buildPropagateTargets classifies every linked worktree into either a
// PropagateTarget (will be written) or a SkippedLinked entry (skipped with
// a short reason). The main worktree is excluded from both lists. The anchor
// color is what the caller has decided to apply to main.
func buildPropagateTargets(worktrees []gitworktree.Worktree, anchor color.Color) ([]PropagateTarget, []SkippedLinked) {
	var targets []PropagateTarget
	var skipped []SkippedLinked
	for i := range worktrees {
		w := &worktrees[i]
		if w.IsMain {
			continue
		}
		wsPath, err := workspaceFilePath(w.Path)
		if err != nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: w.Path,
				Reason:        "could not derive workspace path: " + err.Error(),
			})
			continue
		}
		ws, err := workspace.Read(wsPath)
		if err != nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "parse error: " + err.Error(),
			})
			continue
		}
		if ws == nil {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "no .code-workspace",
			})
			continue
		}
		if len(workspace.ExistingPeacockKeys(ws)) == 0 {
			skipped = append(skipped, SkippedLinked{
				WorkspacePath: wsPath,
				Reason:        "no peacock keys",
			})
			continue
		}
		offset := color.LadderOffset(gitworktree.IdentityHash(*w))
		derived := anchor.ApplyLightness(offset)
		targets = append(targets, PropagateTarget{
			WorkspacePath: wsPath,
			DerivedColor:  derived,
		})
	}
	return targets, skipped
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

// formatPropagatedWarning renders the multi-line warn produced by A2.
// Sections (anchor / applied / failed / skipped) appear only when populated.
// When no linked worktrees end up in any section, a one-line hint replaces them.
//
// Each row repeats its section label because cmd/ccws/render.go strips
// leading whitespace before printing — continuation indents would render
// as orphan paths.
func formatPropagatedWarning(intent *PropagateIntent, failed []PropagateFailure) string {
	var b strings.Builder
	b.WriteString("family propagated from main worktree\n")
	fmt.Fprintf(&b, "  anchor at  %s  %s", intent.AnchorPath, intent.AnchorColor.Hex())

	if len(intent.Targets) == 0 && len(failed) == 0 && len(intent.Skipped) == 0 {
		b.WriteString("\n  (no linked worktrees in family)")
		return b.String()
	}

	for _, tgt := range intent.Targets {
		fmt.Fprintf(&b, "\n  applied    %s  %s", tgt.WorkspacePath, tgt.DerivedColor.Hex())
	}
	for _, f := range failed {
		fmt.Fprintf(&b, "\n  failed     %s  %s", f.WorkspacePath, f.Err.Error())
	}
	for _, s := range intent.Skipped {
		fmt.Fprintf(&b, "\n  skipped    %s  (%s)", s.WorkspacePath, s.Reason)
	}
	return b.String()
}
