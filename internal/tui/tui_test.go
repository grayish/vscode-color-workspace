package tui

import (
	"bytes"
	"testing"
)

func TestNewWriter_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
}

func TestOK_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.OK("wrote foo")
	got := buf.String()
	want := "  ok     wrote foo\n"
	if got != want {
		t.Errorf("OK output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWarn_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Warn("a")
	got := buf.String()
	want := "  warn   a\n"
	if got != want {
		t.Errorf("Warn output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestError_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Error("e")
	got := buf.String()
	want := "  error  e\n"
	if got != want {
		t.Errorf("Error output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderBadge_LongLabelDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("renderBadge panicked on long label: %v", r)
		}
	}()
	got := w.renderBadge("over-five")
	if got != "over-five" {
		t.Errorf("renderBadge(over-five) = %q, want %q (no padding when label > badgeWidth)", got, "over-five")
	}
}
