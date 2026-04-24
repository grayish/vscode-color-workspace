package runner

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Opener abstracts launching the `code` CLI so tests can stub it.
type Opener interface {
	Open(workspacePath string) error
}

// CodeOpener tries the `code` CLI first; on macOS, falls back to `open -a`
// so users whose `code` is a shell function (not a PATH binary) still work.
type CodeOpener struct{}

var ErrCodeNotFound = fmt.Errorf("code CLI not found on PATH")

func (CodeOpener) Open(workspacePath string) error {
	if codePath, err := exec.LookPath("code"); err == nil {
		return start(codePath, workspacePath)
	}
	if runtime.GOOS == "darwin" {
		if openPath, err := exec.LookPath("open"); err == nil {
			return start(openPath, "-a", "Visual Studio Code", workspacePath)
		}
	}
	return ErrCodeNotFound
}

func start(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
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
