package jsonc

import (
	"reflect"
	"strings"
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

// Write sets SetEscapeHTML(false): a folder path or setting value containing
// &, < or > must survive verbatim rather than turn into &-style escapes.
func TestWrite_DoesNotEscapeHTML(t *testing.T) {
	in := map[string]any{"path": "./A&B <c>"}
	out, err := Write(in)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(out), `./A&B <c>`) {
		t.Errorf("HTML characters were escaped: %s", out)
	}
}

func TestWrite_Indentation(t *testing.T) {
	out, err := Write(map[string]any{"a": map[string]any{"b": 1}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(out), "\n  \"a\": {\n    \"b\": 1\n  }") {
		t.Errorf("want 2-space nested indent, got:\n%s", out)
	}
}

func TestWrite_UnencodableValue(t *testing.T) {
	if _, err := Write(map[string]any{"ch": make(chan int)}); err == nil {
		t.Error("expected error for unencodable value")
	}
}

// hujson tolerates trailing commas in arrays as well as objects; .code-workspace
// "folders" lists are the case that matters.
func TestRead_TrailingCommaInArray(t *testing.T) {
	in := []byte(`{"folders": [{"path": "./a"}, {"path": "./b"},],}`)
	var out struct {
		Folders []struct {
			Path string `json:"path"`
		} `json:"folders"`
	}
	if err := Read(in, &out); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(out.Folders) != 2 || out.Folders[1].Path != "./b" {
		t.Errorf("folders = %+v", out.Folders)
	}
}

// Read must reject a valid-JSONC document whose shape does not fit the target,
// so callers see a decode error rather than a silently empty struct.
func TestRead_TypeMismatch(t *testing.T) {
	var out map[string]any
	if err := Read([]byte(`["a", "b"]`), &out); err == nil {
		t.Error("expected unmarshal error for array into map")
	}
}
