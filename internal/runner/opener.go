package runner

import (
	"fmt"
	"os/exec"
)

// Opener abstracts launching the `code` CLI so tests can stub it.
type Opener interface {
	Open(workspacePath string) error
}

// CodeOpener runs the real `code` CLI. If `code` is not on PATH it returns
// ErrCodeNotFound so the caller can emit a warning.
type CodeOpener struct{}

var ErrCodeNotFound = fmt.Errorf("code CLI not found on PATH")

func (CodeOpener) Open(workspacePath string) error {
	codePath, err := exec.LookPath("code")
	if err != nil {
		return ErrCodeNotFound
	}
	cmd := exec.Command(codePath, workspacePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec code: %w", err)
	}
	return cmd.Process.Release()
}

// FakeOpener records calls; used in tests.
type FakeOpener struct {
	Calls []string
	Err   error
}

func (f *FakeOpener) Open(p string) error {
	f.Calls = append(f.Calls, p)
	return f.Err
}
