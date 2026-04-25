package tui

import (
	"bytes"
	"fmt"
	"strings"
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

func TestBullets_NoTruncation(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets([]string{"a", "b", "c"}, 8)
	got := buf.String()
	want := "           • a\n           • b\n           • c\n"
	if got != want {
		t.Errorf("Bullets mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBullets_Truncates(t *testing.T) {
	items := make([]string, 17)
	for i := range items {
		items[i] = fmt.Sprintf("k%d", i)
	}
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(items, 8)
	got := buf.String()
	var want strings.Builder
	for i := 0; i < 8; i++ {
		want.WriteString(fmt.Sprintf("           • k%d\n", i))
	}
	want.WriteString("           …(9 more)\n")
	if got != want.String() {
		t.Errorf("Bullets truncation mismatch:\ngot:  %q\nwant: %q", got, want.String())
	}
}

func TestBullets_Empty(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(nil, 8)
	if buf.Len() != 0 {
		t.Errorf("Bullets(nil) should produce no output, got %q", buf.String())
	}
}

func TestBullets_ExactlyAtLimit(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h"} // exactly 8
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	w.Bullets(items, 8)
	got := buf.String()
	if strings.Contains(got, "more") {
		t.Errorf("expected no truncation marker for items==max, got %q", got)
	}
	lines := strings.Count(got, "\n")
	if lines != 8 {
		t.Errorf("expected 8 lines, got %d in %q", lines, got)
	}
}
