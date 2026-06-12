//go:build integration

package gitworktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realPath resolves symlinks so that macOS /var/folders (→ /private/var/…)
// comparisons work reliably.
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return r
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestList_RealGit_MainPlusLinked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	main := filepath.Join(base, "myproj")
	if err := exec.Command("git", "init", main).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	// need at least one commit before adding a worktree
	if err := exec.Command("touch", filepath.Join(main, "README")).Run(); err != nil {
		t.Fatal(err)
	}
	runGit(t, main, "add", ".")
	runGit(t, main, "commit", "-m", "init")
	linked := filepath.Join(base, "myproj-feat-x")
	runGit(t, main, "worktree", "add", "-b", "feat-x", linked)

	// On macOS t.TempDir() returns /var/… which is a symlink to /private/var/…;
	// git resolves symlinks so we must compare against the real paths.
	wantMain := realPath(t, main)
	wantLinked := realPath(t, linked)
	// git names the worktrees dir entry after the linked directory's basename.
	linkedBasename := filepath.Base(linked)
	wantGitDirSuffix := "/.git/worktrees/" + linkedBasename

	// List can be called from either directory; use the OS path here to verify
	// that List itself handles symlink resolution via git's -C flag.
	if err := os.MkdirAll(linked, 0o755); err != nil && !os.IsExist(err) {
		t.Fatalf("ensure linked dir: %v", err)
	}
	got, err := List(linked)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2; got = %+v", len(got), got)
	}
	if !got[0].IsMain {
		t.Errorf("got[0].IsMain = false, want true")
	}
	if got[0].Path != wantMain {
		t.Errorf("got[0].Path = %q, want %q", got[0].Path, wantMain)
	}
	if got[1].Path != wantLinked {
		t.Errorf("got[1].Path = %q, want %q", got[1].Path, wantLinked)
	}
	if got[1].Branch != "feat-x" {
		t.Errorf("got[1].Branch = %q, want feat-x", got[1].Branch)
	}
	if !strings.HasSuffix(got[1].GitDir, wantGitDirSuffix) {
		t.Errorf("got[1].GitDir = %q, want suffix %q", got[1].GitDir, wantGitDirSuffix)
	}
}

// A linked worktree whose directory was deleted without `git worktree remove`
// (git reports it as "prunable") must not abort the whole listing — the dead
// entry is skipped and the live worktrees are still returned. Regression test
// for the family logic silently collapsing to random when any sibling is stale.
func TestList_RealGit_SkipsPrunableEntry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	main := filepath.Join(base, "myproj")
	if err := exec.Command("git", "init", main).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := exec.Command("touch", filepath.Join(main, "README")).Run(); err != nil {
		t.Fatal(err)
	}
	runGit(t, main, "add", ".")
	runGit(t, main, "commit", "-m", "init")
	linked := filepath.Join(base, "myproj-feat-x")
	runGit(t, main, "worktree", "add", "-b", "feat-x", linked)
	dead := filepath.Join(base, "myproj-dead")
	runGit(t, main, "worktree", "add", "-b", "dead", dead)
	if err := os.RemoveAll(dead); err != nil {
		t.Fatalf("remove dead worktree dir: %v", err)
	}

	got, err := List(linked)
	if err != nil {
		t.Fatalf("List with prunable sibling: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (main + live linked); got = %+v", len(got), got)
	}
	if !got[0].IsMain {
		t.Errorf("got[0].IsMain = false, want true")
	}
	if got[0].Path != realPath(t, main) {
		t.Errorf("got[0].Path = %q, want %q", got[0].Path, realPath(t, main))
	}
	if got[1].Path != realPath(t, linked) {
		t.Errorf("got[1].Path = %q, want %q", got[1].Path, realPath(t, linked))
	}
	// The survivor is struct-copied into an earlier backing-array slot when a
	// dead entry precedes it; GitDir must survive that copy intact because it
	// feeds IdentityHash → LadderOffset.
	if want := "/.git/worktrees/" + filepath.Base(linked); !strings.HasSuffix(got[1].GitDir, want) {
		t.Errorf("got[1].GitDir = %q, want suffix %q", got[1].GitDir, want)
	}
	if got[1].Branch != "feat-x" {
		t.Errorf("got[1].Branch = %q, want feat-x", got[1].Branch)
	}
}

func TestList_NotInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	_, err := List(dir)
	if err == nil {
		t.Fatal("List(non-git dir) returned nil error")
	}
	if !errors.Is(err, ErrNotInWorktree) {
		t.Errorf("err = %v, want ErrNotInWorktree", err)
	}
}
