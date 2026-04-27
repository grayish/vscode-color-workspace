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
