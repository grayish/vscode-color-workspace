package jsonc

import (
	"reflect"
	"testing"
)

func TestRead_StandardJSON(t *testing.T) {
	in := []byte(`{"a": 1, "b": "x"}`)
	var out map[string]any
	if err := Read(in, &out); err != nil {
		t.Fatalf("err = %v", err)
	}
	if out["a"].(float64) != 1 || out["b"].(string) != "x" {
		t.Errorf("got %v", out)
	}
}

func TestRead_WithComments(t *testing.T) {
	in := []byte(`{
		// line comment
		"a": 1,
		/* block */
		"b": "x", // trailing
	}`)
	var out map[string]any
	if err := Read(in, &out); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := map[string]any{"a": float64(1), "b": "x"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got %v, want %v", out, want)
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	in := []byte(`{not json`)
	var out map[string]any
	if err := Read(in, &out); err == nil {
		t.Error("expected error")
	}
}

func TestWrite_Indented(t *testing.T) {
	in := map[string]any{"a": 1, "b": "x"}
	out, err := Write(in)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		t.Error("output should end with newline")
	}
}
