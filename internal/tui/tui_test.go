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

func TestDetails_LabelValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Details([]Detail{
		{Label: "color", Value: "#abc"},
		{Label: "file", Value: "~/x"},
	})
	got := buf.String()
	want := "         color  #abc\n         file  ~/x\n"
	if got != want {
		t.Errorf("Details mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDetails_HeaderRow(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Details([]Detail{{Label: "keys", Value: ""}})
	got := buf.String()
	want := "         keys\n"
	if got != want {
		t.Errorf("Details header mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}
