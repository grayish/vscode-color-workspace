// Package jsonc provides JSONC (JSON with comments) parsing and standardized
// JSON writing. VSCode's settings.json and .code-workspace both use JSONC.
package jsonc

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tailscale/hujson"
)

// Read parses JSONC input into v. Comments and trailing commas are tolerated.
func Read(data []byte, v any) error {
	norm, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("jsonc: parse: %w", err)
	}
	norm.Standardize()
	if err := json.Unmarshal(norm.Pack(), v); err != nil {
		return fmt.Errorf("jsonc: unmarshal: %w", err)
	}
	return nil
}

// Write marshals v to indented JSON (2-space) with a trailing newline.
// Comments from input are NOT preserved.
func Write(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("jsonc: encode: %w", err)
	}
	return buf.Bytes(), nil
}
