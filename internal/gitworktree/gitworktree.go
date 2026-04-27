// Package gitworktree wraps `git worktree list --porcelain` so callers can
// reason about the set of worktrees attached to a given target directory
// without depending on the git binary directly.
package gitworktree

import (
	"bufio"
	"bytes"
	"errors"
	"hash/fnv"
	"path/filepath"
	"strings"
)

// ErrNotInWorktree means the target directory is not under any git repo,
// or git is unavailable, or `git worktree list` produced unusable output.
// Callers treat this as "skip worktree logic, fall back to the existing
// resolution chain."
var ErrNotInWorktree = errors.New("gitworktree: target is not in a git worktree")

// Worktree describes a single worktree as reported by `git worktree list --porcelain`.
type Worktree struct {
	Path   string // absolute working tree path
	GitDir string // populated by List; <main>/.git or <main>/.git/worktrees/<name>
	Branch string // empty for detached HEAD
	IsMain bool   // true for the primary worktree (first entry in --porcelain)
	Bare   bool   // true for bare repos (no working tree)
}

// parsePorcelain converts the raw bytes of `git worktree list --porcelain`
// into a slice of Worktree records. Records are separated by blank lines.
// The first record is treated as main by the caller (List sets IsMain).
func parsePorcelain(data []byte) ([]Worktree, error) {
	var out []Worktree
	var cur Worktree
	started := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if started {
				out = append(out, cur)
				cur = Worktree{}
				started = false
			}
			continue
		}
		started = true
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			cur.Branch = ""
		case line == "bare":
			cur.Bare = true
		}
	}
	if started {
		out = append(out, cur)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// IdentityHash returns a stable 64-bit identifier for a worktree.
// Main returns 0 by convention so it always maps to LadderOffset = 0.
// Linked worktrees use FNV-1a over basename(GitDir) — git keeps that name
// stable across `git worktree move` and branch renames.
func IdentityHash(w Worktree) uint64 {
	if w.IsMain {
		return 0
	}
	name := filepath.Base(w.GitDir)
	if name == "" || name == "." || name == "/" {
		name = w.Path
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	sum := h.Sum64()
	if sum == 0 {
		return 1 // never collide with the main-worktree convention
	}
	return sum
}

// FindSelf returns the worktree whose Path equals targetDir or is an
// ancestor of targetDir. Returns nil if no entry matches.
func FindSelf(worktrees []Worktree, targetDir string) *Worktree {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return nil
	}
	var best *Worktree
	for i := range worktrees {
		w := &worktrees[i]
		if w.Path == "" {
			continue
		}
		if abs == w.Path || strings.HasPrefix(abs, w.Path+string(filepath.Separator)) {
			if best == nil || len(w.Path) > len(best.Path) {
				best = w
			}
		}
	}
	return best
}
