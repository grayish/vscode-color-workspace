package gitworktree

import (
	"strings"
	"testing"
)

func TestParsePorcelain_MainPlusLinked(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /Users/user/code/myproj-feat-x",
		"HEAD def456",
		"branch refs/heads/feat-x",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Path != "/Users/user/code/myproj" || got[0].Branch != "main" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Path != "/Users/user/code/myproj-feat-x" || got[1].Branch != "feat-x" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestParsePorcelain_DetachedHEAD(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /Users/user/code/myproj-detached",
		"HEAD ddd000",
		"detached",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].Branch != "" {
		t.Errorf("detached branch = %q, want empty", got[1].Branch)
	}
}

func TestParsePorcelain_BareRepo(t *testing.T) {
	in := strings.Join([]string{
		"worktree /Users/user/code/myproj.git",
		"bare",
		"",
	}, "\n")
	got, err := parsePorcelain([]byte(in))
	if err != nil {
		t.Fatalf("parsePorcelain error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].Bare {
		t.Errorf("Bare = false, want true")
	}
}

func TestParsePorcelain_Empty(t *testing.T) {
	got, err := parsePorcelain([]byte(""))
	if err != nil {
		t.Fatalf("parsePorcelain(empty) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestIdentityHash_MainReturnsZero(t *testing.T) {
	w := Worktree{Path: "/tmp/myproj", GitDir: "/tmp/myproj/.git", IsMain: true}
	if got := IdentityHash(w); got != 0 {
		t.Errorf("IdentityHash(main) = %d, want 0", got)
	}
}

func TestIdentityHash_LinkedStable(t *testing.T) {
	w := Worktree{Path: "/tmp/myproj-feat-x", GitDir: "/tmp/myproj/.git/worktrees/feat-x"}
	a := IdentityHash(w)
	b := IdentityHash(w)
	if a != b {
		t.Errorf("IdentityHash not stable: %d vs %d", a, b)
	}
	if a == 0 {
		t.Error("IdentityHash(linked) returned 0 (collision with main convention)")
	}
}

func TestIdentityHash_DifferentLinkedDifferent(t *testing.T) {
	a := IdentityHash(Worktree{GitDir: "/tmp/.git/worktrees/feat-x"})
	b := IdentityHash(Worktree{GitDir: "/tmp/.git/worktrees/bugfix"})
	if a == b {
		t.Errorf("hashes collide: feat-x=%d bugfix=%d", a, b)
	}
}

func TestFindSelf_ExactPath(t *testing.T) {
	wts := []Worktree{
		{Path: "/tmp/main", IsMain: true},
		{Path: "/tmp/linked", IsMain: false},
	}
	got := FindSelf(wts, "/tmp/linked")
	if got == nil || got.Path != "/tmp/linked" {
		t.Errorf("FindSelf = %+v", got)
	}
}

func TestFindSelf_Subdir(t *testing.T) {
	wts := []Worktree{
		{Path: "/tmp/main", IsMain: true},
		{Path: "/tmp/linked", IsMain: false},
	}
	got := FindSelf(wts, "/tmp/linked/sub/dir")
	if got == nil || got.Path != "/tmp/linked" {
		t.Errorf("FindSelf(subdir) = %+v", got)
	}
}

func TestFindSelf_NoMatch(t *testing.T) {
	wts := []Worktree{{Path: "/tmp/main", IsMain: true}}
	if got := FindSelf(wts, "/elsewhere"); got != nil {
		t.Errorf("FindSelf(unrelated) = %+v, want nil", got)
	}
}
