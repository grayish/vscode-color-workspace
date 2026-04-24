package runner

import (
	"errors"
	"testing"
)

func TestFakeOpener(t *testing.T) {
	f := &FakeOpener{}
	if err := f.Open("/path/to/ws.code-workspace"); err != nil {
		t.Fatal(err)
	}
	if len(f.Calls) != 1 || f.Calls[0] != "/path/to/ws.code-workspace" {
		t.Errorf("calls = %v", f.Calls)
	}
}

func TestFakeOpener_ReturnsError(t *testing.T) {
	f := &FakeOpener{Err: errors.New("boom")}
	if err := f.Open("x"); err == nil {
		t.Error("want error")
	}
}
