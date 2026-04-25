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
