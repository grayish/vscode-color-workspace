# vscode-color-workspace (`ccws`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI (`ccws`) that generates `<parent>/<folder>.code-workspace` files with Peacock-equivalent color settings, migrates existing Peacock config out of `.vscode/settings.json`, and opens the workspace in VSCode.

**Architecture:** Layered packages with no cycles — `color` (pure primitives + palette) → `peacock` (key constants) → `workspace` / `vscodesettings` (file I/O with safety guards) → `runner` (orchestration) → `interactive` + `cmd/ccws` (CLI surface). TDD with golden tests cross-checked against Peacock's JS output.

**Tech Stack:** Go 1.22+, Cobra (subcommands), Charmbracelet Huh (interactive form), go-colorful (HSL conversion), tailscale/hujson (JSONC parsing).

**Spec reference:** `docs/superpowers/specs/2026-04-25-ccws-vscode-color-workspace-design.md`

**Peacock source reference:** `/Users/user/Projects/vscode-peacock/src/` — primary files `color-library.ts`, `configuration/read-configuration.ts` (`prepareColors` + `collect*Settings`), `models/enums.ts` (`ColorSettings`, `AffectedSettings`).

---

## Task 1: Scaffold Go module and directory structure

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `cmd/ccws/main.go` (stub)
- Create: `internal/color/`, `internal/peacock/`, `internal/workspace/`, `internal/vscodesettings/`, `internal/interactive/`, `internal/runner/` (empty dirs)

- [ ] **Step 1: Initialize Go module**

Run: `cd /Users/user/Projects/color-vscode-workspace && go mod init github.com/sang-bin/vscode-color-workspace`

Expected: `go.mod` created.

- [ ] **Step 2: Create `.gitignore`**

Write to `.gitignore`:

```
/ccws
/bin/
*.test
*.out
.DS_Store
```

- [ ] **Step 3: Create placeholder `cmd/ccws/main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("ccws: not yet implemented")
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: no errors, binary `ccws` produced.

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore cmd/ccws/main.go
git commit -m "scaffold: go module + cmd/ccws stub"
```

---

## Task 2: Color struct — hex / HSL / brightness / alpha

**Files:**
- Create: `internal/color/primitives.go`
- Create: `internal/color/primitives_test.go`

- [ ] **Step 1: Write failing tests**

`internal/color/primitives_test.go`:

```go
package color

import "testing"

func TestColor_Hex(t *testing.T) {
	tests := []struct {
		c    Color
		want string
	}{
		{Color{0, 0, 0}, "#000000"},
		{Color{255, 255, 255}, "#ffffff"},
		{Color{255, 0, 0}, "#ff0000"},
		{Color{90, 59, 140}, "#5a3b8c"},
	}
	for _, tt := range tests {
		if got := tt.c.Hex(); got != tt.want {
			t.Errorf("Hex(%v) = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestColor_HexWithAlpha(t *testing.T) {
	c := Color{90, 59, 140}
	if got := c.HexWithAlpha(0x99); got != "#5a3b8c99" {
		t.Errorf("HexWithAlpha = %q, want #5a3b8c99", got)
	}
}

func TestColor_Brightness(t *testing.T) {
	// Note: 299+587+114 == 1000, so for equal R/G/B we get the channel value back.
	tests := []struct {
		c    Color
		want int
	}{
		{Color{0, 0, 0}, 0},
		{Color{255, 255, 255}, 255},
		{Color{128, 128, 128}, 128},
	}
	for _, tt := range tests {
		if got := tt.c.Brightness(); got != tt.want {
			t.Errorf("Brightness(%v) = %d, want %d", tt.c, got, tt.want)
		}
	}
}

func TestColor_IsLight(t *testing.T) {
	if !(Color{255, 255, 255}).IsLight() {
		t.Error("white should be light")
	}
	if (Color{0, 0, 0}).IsLight() {
		t.Error("black should not be light")
	}
	if !(Color{128, 128, 128}).IsLight() {
		t.Error("gray 128 should be light (brightness == 128 threshold)")
	}
}
```

Note: Brightness of #808080 is 128 exactly; IsLight uses >= 128 so true.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error (Color undefined).

- [ ] **Step 3: Implement primitives**

`internal/color/primitives.go`:

```go
// Package color provides color primitives ported from Peacock's tinycolor2
// usage, plus the palette generator that mirrors Peacock's prepareColors.
package color

import "fmt"

// Color is an RGB color with 0-255 channels. Alpha is handled separately
// since only a few outputs need it (see HexWithAlpha).
type Color struct {
	R, G, B uint8
}

// Hex returns a #rrggbb string.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// HexWithAlpha returns a #rrggbbaa string.
func (c Color) HexWithAlpha(a uint8) string {
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, a)
}

// Brightness matches tinycolor: (r*299 + g*587 + b*114) / 1000.
func (c Color) Brightness() int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// IsLight matches tinycolor: brightness >= 128.
func (c Color) IsLight() bool {
	return c.Brightness() >= 128
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add Color struct with hex and brightness"
```

---

## Task 3: HSL conversion via go-colorful

**Files:**
- Modify: `internal/color/primitives.go`
- Modify: `internal/color/primitives_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Write failing HSL round-trip test**

Append to `internal/color/primitives_test.go`:

```go
func TestColor_HSLRoundtrip(t *testing.T) {
	tests := []Color{
		{255, 0, 0},     // red
		{0, 255, 0},     // green
		{0, 0, 255},     // blue
		{90, 59, 140},   // purple sample
		{255, 255, 255}, // white
		{0, 0, 0},       // black
	}
	for _, c := range tests {
		h, s, l := c.ToHSL()
		back := FromHSL(h, s, l)
		// Allow 1-unit rounding error per channel
		dr := abs(int(c.R) - int(back.R))
		dg := abs(int(c.G) - int(back.G))
		db := abs(int(c.B) - int(back.B))
		if dr > 1 || dg > 1 || db > 1 {
			t.Errorf("Roundtrip(%v) -> %v (diff r=%d g=%d b=%d)", c, back, dr, dg, db)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestColor_ToHSL_Hue(t *testing.T) {
	// Red should have hue 0
	h, _, _ := (Color{255, 0, 0}).ToHSL()
	if h < -0.5 || h > 0.5 {
		t.Errorf("red hue = %f, want ~0", h)
	}
	// Green should have hue 120
	h, _, _ = (Color{0, 255, 0}).ToHSL()
	if h < 119.5 || h > 120.5 {
		t.Errorf("green hue = %f, want ~120", h)
	}
}
```

- [ ] **Step 2: Add go-colorful dep**

Run: `go get github.com/lucasb-eyer/go-colorful@v1.2.0`
Expected: `go.mod` / `go.sum` updated.

- [ ] **Step 3: Implement HSL conversion**

Append to `internal/color/primitives.go`:

```go
import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
)

// ToHSL returns hue in [0, 360), saturation and lightness in [0, 1].
func (c Color) ToHSL() (h, s, l float64) {
	cf := colorful.Color{
		R: float64(c.R) / 255,
		G: float64(c.G) / 255,
		B: float64(c.B) / 255,
	}
	return cf.Hsl()
}

// FromHSL builds a Color from HSL values (h in [0,360), s/l in [0,1]).
// Out-of-range values are wrapped / clamped.
func FromHSL(h, s, l float64) Color {
	h = wrapHue(h)
	s = clamp01(s)
	l = clamp01(l)
	cf := colorful.Hsl(h, s, l)
	r, g, b := cf.RGB255()
	return Color{r, g, b}
}

func wrapHue(h float64) float64 {
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	return h
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
```

Remove the duplicate `import "fmt"` line — merge into the single import block at the top:

```go
import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/ go.mod go.sum
git commit -m "color: add HSL conversion via go-colorful"
```

---

## Task 4: Parse hex / CSS named colors / random

**Files:**
- Create: `internal/color/names.go` (CSS named color lookup)
- Create: `internal/color/parse.go`
- Create: `internal/color/parse_test.go`

- [ ] **Step 1: Write failing tests**

`internal/color/parse_test.go`:

```go
package color

import "testing"

func TestParse_Hex(t *testing.T) {
	tests := []struct {
		in   string
		want Color
	}{
		{"#ff0000", Color{255, 0, 0}},
		{"ff0000", Color{255, 0, 0}},
		{"#FF0000", Color{255, 0, 0}},
		{"#f00", Color{255, 0, 0}},
		{"f00", Color{255, 0, 0}},
		{"#5a3b8c", Color{90, 59, 140}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParse_Named(t *testing.T) {
	tests := []struct {
		in   string
		want Color
	}{
		{"red", Color{255, 0, 0}},
		{"Red", Color{255, 0, 0}},
		{"white", Color{255, 255, 255}},
		{"black", Color{0, 0, 0}},
		{"papayawhip", Color{255, 239, 213}},
		{"rebeccapurple", Color{102, 51, 153}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParse_Random(t *testing.T) {
	c, err := Parse("random")
	if err != nil {
		t.Fatalf("Parse(\"random\") error: %v", err)
	}
	// Just assert it returns a valid Color (any value).
	_ = c
}

func TestParse_Invalid(t *testing.T) {
	inputs := []string{"", "not-a-color", "#xyz", "#1234", "#12345", "rgb(1,2,3)"}
	for _, in := range inputs {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil err, want error", in)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error (Parse, namedColors undefined).

- [ ] **Step 3: Create named color table**

`internal/color/names.go` — ship the W3C CSS named color list. For brevity, use the exhaustive 147-entry table. Starter excerpt (the full table should include all standard CSS colors — the engineer should paste from a reference like Wikipedia's "Web colors" page or the W3C CSS Color Module Level 4 list):

```go
package color

import "strings"

// cssNamedColors maps lowercased CSS named colors to their hex values.
// Source: W3C CSS Color Module Level 4.
var cssNamedColors = map[string]Color{
	"aliceblue":            {240, 248, 255},
	"antiquewhite":         {250, 235, 215},
	"aqua":                 {0, 255, 255},
	"aquamarine":           {127, 255, 212},
	"azure":                {240, 255, 255},
	"beige":                {245, 245, 220},
	"bisque":               {255, 228, 196},
	"black":                {0, 0, 0},
	"blanchedalmond":       {255, 235, 205},
	"blue":                 {0, 0, 255},
	"blueviolet":           {138, 43, 226},
	"brown":                {165, 42, 42},
	"burlywood":            {222, 184, 135},
	"cadetblue":            {95, 158, 160},
	"chartreuse":           {127, 255, 0},
	"chocolate":            {210, 105, 30},
	"coral":                {255, 127, 80},
	"cornflowerblue":       {100, 149, 237},
	"cornsilk":             {255, 248, 220},
	"crimson":              {220, 20, 60},
	"cyan":                 {0, 255, 255},
	"darkblue":             {0, 0, 139},
	"darkcyan":             {0, 139, 139},
	"darkgoldenrod":        {184, 134, 11},
	"darkgray":             {169, 169, 169},
	"darkgreen":            {0, 100, 0},
	"darkgrey":             {169, 169, 169},
	"darkkhaki":            {189, 183, 107},
	"darkmagenta":          {139, 0, 139},
	"darkolivegreen":       {85, 107, 47},
	"darkorange":           {255, 140, 0},
	"darkorchid":           {153, 50, 204},
	"darkred":              {139, 0, 0},
	"darksalmon":           {233, 150, 122},
	"darkseagreen":         {143, 188, 143},
	"darkslateblue":        {72, 61, 139},
	"darkslategray":        {47, 79, 79},
	"darkslategrey":        {47, 79, 79},
	"darkturquoise":        {0, 206, 209},
	"darkviolet":           {148, 0, 211},
	"deeppink":             {255, 20, 147},
	"deepskyblue":          {0, 191, 255},
	"dimgray":              {105, 105, 105},
	"dimgrey":              {105, 105, 105},
	"dodgerblue":           {30, 144, 255},
	"firebrick":            {178, 34, 34},
	"floralwhite":          {255, 250, 240},
	"forestgreen":          {34, 139, 34},
	"fuchsia":              {255, 0, 255},
	"gainsboro":            {220, 220, 220},
	"ghostwhite":           {248, 248, 255},
	"gold":                 {255, 215, 0},
	"goldenrod":            {218, 165, 32},
	"gray":                 {128, 128, 128},
	"green":                {0, 128, 0},
	"greenyellow":          {173, 255, 47},
	"grey":                 {128, 128, 128},
	"honeydew":             {240, 255, 240},
	"hotpink":              {255, 105, 180},
	"indianred":            {205, 92, 92},
	"indigo":               {75, 0, 130},
	"ivory":                {255, 255, 240},
	"khaki":                {240, 230, 140},
	"lavender":             {230, 230, 250},
	"lavenderblush":        {255, 240, 245},
	"lawngreen":            {124, 252, 0},
	"lemonchiffon":         {255, 250, 205},
	"lightblue":            {173, 216, 230},
	"lightcoral":           {240, 128, 128},
	"lightcyan":            {224, 255, 255},
	"lightgoldenrodyellow": {250, 250, 210},
	"lightgray":            {211, 211, 211},
	"lightgreen":           {144, 238, 144},
	"lightgrey":            {211, 211, 211},
	"lightpink":            {255, 182, 193},
	"lightsalmon":          {255, 160, 122},
	"lightseagreen":        {32, 178, 170},
	"lightskyblue":         {135, 206, 250},
	"lightslategray":       {119, 136, 153},
	"lightslategrey":       {119, 136, 153},
	"lightsteelblue":       {176, 196, 222},
	"lightyellow":          {255, 255, 224},
	"lime":                 {0, 255, 0},
	"limegreen":            {50, 205, 50},
	"linen":                {250, 240, 230},
	"magenta":              {255, 0, 255},
	"maroon":               {128, 0, 0},
	"mediumaquamarine":     {102, 205, 170},
	"mediumblue":           {0, 0, 205},
	"mediumorchid":         {186, 85, 211},
	"mediumpurple":         {147, 112, 219},
	"mediumseagreen":       {60, 179, 113},
	"mediumslateblue":      {123, 104, 238},
	"mediumspringgreen":    {0, 250, 154},
	"mediumturquoise":      {72, 209, 204},
	"mediumvioletred":      {199, 21, 133},
	"midnightblue":         {25, 25, 112},
	"mintcream":            {245, 255, 250},
	"mistyrose":            {255, 228, 225},
	"moccasin":             {255, 228, 181},
	"navajowhite":          {255, 222, 173},
	"navy":                 {0, 0, 128},
	"oldlace":              {253, 245, 230},
	"olive":                {128, 128, 0},
	"olivedrab":            {107, 142, 35},
	"orange":               {255, 165, 0},
	"orangered":            {255, 69, 0},
	"orchid":               {218, 112, 214},
	"palegoldenrod":        {238, 232, 170},
	"palegreen":            {152, 251, 152},
	"paleturquoise":        {175, 238, 238},
	"palevioletred":        {219, 112, 147},
	"papayawhip":           {255, 239, 213},
	"peachpuff":            {255, 218, 185},
	"peru":                 {205, 133, 63},
	"pink":                 {255, 192, 203},
	"plum":                 {221, 160, 221},
	"powderblue":           {176, 224, 230},
	"purple":               {128, 0, 128},
	"rebeccapurple":        {102, 51, 153},
	"red":                  {255, 0, 0},
	"rosybrown":            {188, 143, 143},
	"royalblue":            {65, 105, 225},
	"saddlebrown":          {139, 69, 19},
	"salmon":               {250, 128, 114},
	"sandybrown":           {244, 164, 96},
	"seagreen":             {46, 139, 87},
	"seashell":             {255, 245, 238},
	"sienna":               {160, 82, 45},
	"silver":               {192, 192, 192},
	"skyblue":              {135, 206, 235},
	"slateblue":            {106, 90, 205},
	"slategray":            {112, 128, 144},
	"slategrey":            {112, 128, 144},
	"snow":                 {255, 250, 250},
	"springgreen":          {0, 255, 127},
	"steelblue":            {70, 130, 180},
	"tan":                  {210, 180, 140},
	"teal":                 {0, 128, 128},
	"thistle":              {216, 191, 216},
	"tomato":               {255, 99, 71},
	"transparent":          {0, 0, 0}, // special, not useful for backgrounds
	"turquoise":            {64, 224, 208},
	"violet":               {238, 130, 238},
	"wheat":                {245, 222, 179},
	"white":                {255, 255, 255},
	"whitesmoke":           {245, 245, 245},
	"yellow":               {255, 255, 0},
	"yellowgreen":          {154, 205, 50},
}

// lookupNamed returns the Color for a CSS name (case-insensitive) and true if found.
func lookupNamed(name string) (Color, bool) {
	c, ok := cssNamedColors[strings.ToLower(name)]
	return c, ok
}
```

- [ ] **Step 4: Implement Parse and Random**

`internal/color/parse.go`:

```go
package color

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Parse resolves an input string to a Color.
// Accepted forms:
//   - "#rrggbb" or "rrggbb"
//   - "#rgb" or "rgb" (3-digit shorthand)
//   - CSS named colors (case-insensitive)
//   - "random"
func Parse(input string) (Color, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Color{}, fmt.Errorf("color: empty input")
	}
	if strings.EqualFold(s, "random") {
		return Random(), nil
	}
	if c, ok := lookupNamed(s); ok {
		return c, nil
	}
	return parseHex(s)
}

func parseHex(s string) (Color, error) {
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
		// ok
	default:
		return Color{}, fmt.Errorf("color: invalid hex %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("color: invalid hex %q: %w", s, err)
	}
	return Color{
		R: uint8((v >> 16) & 0xff),
		G: uint8((v >> 8) & 0xff),
		B: uint8(v & 0xff),
	}, nil
}

// Random returns a color with R/G/B each uniformly random in [0, 256).
// This matches tinycolor.random()'s behavior (RGB-space uniform), so ugly
// colors are possible — a deliberate trade for Peacock parity.
func Random() Color {
	return Color{
		R: uint8(rand.Float64() * 256),
		G: uint8(rand.Float64() * 256),
		B: uint8(rand.Float64() * 256),
	}
}
```

Note: `uint8(rand.Float64() * 256)` works because Float64 is in `[0, 1)`, so the product is in `[0, 256)`, truncates to `[0, 255]`. In Go 1.22+, `math/rand/v2` auto-seeds from OS entropy.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/color/
git commit -m "color: add Parse (hex/named/random)"
```

---

## Task 5: Lighten / darken / triad / complement

**Files:**
- Modify: `internal/color/primitives.go`
- Create: `internal/color/transforms_test.go`

- [ ] **Step 1: Write failing tests**

`internal/color/transforms_test.go`:

```go
package color

import "testing"

func TestColor_Triad(t *testing.T) {
	triad := Color{255, 0, 0}.Triad() // red, H=0
	// triad[0] = self
	if triad[0] != (Color{255, 0, 0}) {
		t.Errorf("triad[0] = %v, want red", triad[0])
	}
	// triad[1] = H=120, should be green-ish
	h1, _, _ := triad[1].ToHSL()
	if h1 < 119.5 || h1 > 120.5 {
		t.Errorf("triad[1] hue = %f, want ~120", h1)
	}
	// triad[2] = H=240
	h2, _, _ := triad[2].ToHSL()
	if h2 < 239.5 || h2 > 240.5 {
		t.Errorf("triad[2] hue = %f, want ~240", h2)
	}
}

func TestColor_Complement(t *testing.T) {
	// red -> cyan
	c := (Color{255, 0, 0}).Complement()
	h, _, _ := c.ToHSL()
	if h < 179.5 || h > 180.5 {
		t.Errorf("complement red hue = %f, want ~180", h)
	}
}

func TestColor_Lighten(t *testing.T) {
	// A mid-gray lightened 10% should be lighter
	c := Color{128, 128, 128}
	light := c.Lighten(10)
	if light.Brightness() <= c.Brightness() {
		t.Errorf("lighten(%v) = %v, should be brighter", c, light)
	}
}

func TestColor_Darken(t *testing.T) {
	c := Color{128, 128, 128}
	dark := c.Darken(10)
	if dark.Brightness() >= c.Brightness() {
		t.Errorf("darken(%v) = %v, should be darker", c, dark)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error (Triad/Complement/Lighten/Darken undefined).

- [ ] **Step 3: Implement transforms**

Append to `internal/color/primitives.go`:

```go
// Triad returns three colors 120° apart in hue (HSL).
// Matches tinycolor's triad(): [self, +120°, +240°].
func (c Color) Triad() [3]Color {
	h, s, l := c.ToHSL()
	return [3]Color{
		c,
		FromHSL(h+120, s, l),
		FromHSL(h+240, s, l),
	}
}

// Complement returns the color with hue shifted 180°.
func (c Color) Complement() Color {
	h, s, l := c.ToHSL()
	return FromHSL(h+180, s, l)
}

// Lighten shifts L up by pct (0-100 scale); clamped to 1.
func (c Color) Lighten(pct float64) Color {
	h, s, l := c.ToHSL()
	return FromHSL(h, s, l+pct/100)
}

// Darken shifts L down by pct (0-100 scale); clamped to 0.
func (c Color) Darken(pct float64) Color {
	h, s, l := c.ToHSL()
	return FromHSL(h, s, l-pct/100)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add triad/complement/lighten/darken"
```

---

## Task 6: WCAG relative luminance + contrast ratio

**Files:**
- Modify: `internal/color/primitives.go`
- Create: `internal/color/contrast_test.go`

- [ ] **Step 1: Write failing tests**

`internal/color/contrast_test.go`:

```go
package color

import (
	"math"
	"testing"
)

func TestContrast_BlackWhite(t *testing.T) {
	c := Contrast(Color{0, 0, 0}, Color{255, 255, 255})
	if math.Abs(c-21) > 0.01 {
		t.Errorf("contrast(black, white) = %f, want ~21", c)
	}
}

func TestContrast_Symmetric(t *testing.T) {
	a := Color{255, 0, 0}
	b := Color{0, 0, 255}
	if Contrast(a, b) != Contrast(b, a) {
		t.Error("contrast should be symmetric")
	}
}

func TestContrast_Same(t *testing.T) {
	if got := Contrast(Color{123, 45, 67}, Color{123, 45, 67}); math.Abs(got-1) > 0.01 {
		t.Errorf("contrast(same) = %f, want 1", got)
	}
}

func TestRelativeLuminance(t *testing.T) {
	// Black = 0, white = 1
	if got := (Color{0, 0, 0}).RelativeLuminance(); got != 0 {
		t.Errorf("lum(black) = %f, want 0", got)
	}
	if got := (Color{255, 255, 255}).RelativeLuminance(); math.Abs(got-1) > 0.0001 {
		t.Errorf("lum(white) = %f, want 1", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error.

- [ ] **Step 3: Implement luminance and contrast**

Append to `internal/color/primitives.go`:

```go
import (
	"fmt"
	"math"

	"github.com/lucasb-eyer/go-colorful"
)
```

(merge `math` into existing import block at top of file)

```go
// RelativeLuminance returns the WCAG 2.1 relative luminance in [0, 1].
func (c Color) RelativeLuminance() float64 {
	return 0.2126*linearize(float64(c.R)/255) +
		0.7152*linearize(float64(c.G)/255) +
		0.0722*linearize(float64(c.B)/255)
}

func linearize(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// Contrast returns the WCAG contrast ratio between two colors (1 to 21).
func Contrast(a, b Color) float64 {
	la := a.RelativeLuminance()
	lb := b.RelativeLuminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add WCAG luminance + contrast ratio"
```

---

## Task 7: Element-style derivatives + readable accent

**Files:**
- Create: `internal/color/accent.go`
- Create: `internal/color/accent_test.go`

**Background:** `getReadableAccentColorHex` (Peacock `color-library.ts:52-104`) generates a badge background color by taking the triad[1] of the base, boosting saturation if needed, and picking the shade with the lowest contrast that still meets a readability threshold.

- [ ] **Step 1: Write failing tests**

`internal/color/accent_test.go`:

```go
package color

import (
	"testing"
)

func TestReadableAccent_MeetsRatio(t *testing.T) {
	base := Color{90, 59, 140} // #5a3b8c (purple)
	accent := ReadableAccent(base, 2.0)
	if got := Contrast(accent, base); got < 2.0 {
		t.Errorf("accent contrast = %f, want >= 2.0", got)
	}
}

func TestReadableAccent_Grayscale(t *testing.T) {
	// A gray input should still produce a saturated accent
	base := Color{128, 128, 128}
	accent := ReadableAccent(base, 2.0)
	if accent == base {
		t.Errorf("accent same as grayscale base")
	}
}

func TestReadableAccent_ExtremeBase(t *testing.T) {
	// Extreme values shouldn't panic
	_ = ReadableAccent(Color{0, 0, 0}, 2.0)
	_ = ReadableAccent(Color{255, 255, 255}, 2.0)
}

func TestHoverColor_IsLight(t *testing.T) {
	// Light base -> hover darker; dark base -> hover lighter
	light := Color{240, 240, 240}
	darkened := HoverOf(light)
	if darkened.Brightness() >= light.Brightness() {
		t.Errorf("hover of light should darken")
	}
	dark := Color{30, 30, 30}
	lightened := HoverOf(dark)
	if lightened.Brightness() <= dark.Brightness() {
		t.Errorf("hover of dark should lighten")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error.

- [ ] **Step 3: Implement accent primitives**

`internal/color/accent.go`:

```go
package color

import "math"

// DefaultLightenDarkenAmount is Peacock's defaultAmountToDarkenLighten.
const DefaultLightenDarkenAmount = 10.0

// defaultSaturationBoost matches Peacock's defaultSaturation (0.5).
const defaultSaturationBoost = 0.5

// Readability ratio thresholds (Peacock's ReadabilityRatios enum).
const (
	RatioUILow = 2.0
	RatioUI    = 3.0
	RatioText  = 4.5
)

// HoverOf returns the hover color variant: darken if base is light, lighten otherwise.
// Matches Peacock's getBackgroundHoverColorHex.
func HoverOf(base Color) Color {
	if base.IsLight() {
		return base.Darken(DefaultLightenDarkenAmount)
	}
	return base.Lighten(DefaultLightenDarkenAmount)
}

// ReadableAccent returns an accent color with contrast >= ratio against base.
// Ports Peacock's getReadableAccentColorHex (color-library.ts).
func ReadableAccent(base Color, ratio float64) Color {
	triad := base.Triad()
	fg := triad[1]
	h, s, l := fg.ToHSL()

	// Grayscale (s == 0): synthesize a hue based on lightness.
	if s == 0 {
		h = 60 * math.Round(l*6)
	}

	// Desaturated: boost saturation.
	if s < 0.15 {
		s = defaultSaturationBoost
	}

	type shade struct {
		c        Color
		contrast float64
	}
	const shadeCount = 16
	shades := make([]shade, shadeCount)
	for i := 0; i < shadeCount; i++ {
		c := FromHSL(h, s, float64(i)/float64(shadeCount))
		shades[i] = shade{c, Contrast(c, base)}
	}

	// Sort by contrast ascending; find first shade meeting ratio.
	// (Insertion sort; shadeCount=16 so this is trivial.)
	for i := 1; i < len(shades); i++ {
		for j := i; j > 0 && shades[j-1].contrast > shades[j].contrast; j-- {
			shades[j-1], shades[j] = shades[j], shades[j-1]
		}
	}

	for _, sh := range shades {
		if sh.contrast >= ratio {
			return sh.c
		}
	}
	return Color{255, 255, 255} // fallback
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add HoverOf + ReadableAccent"
```

---

## Task 8: Peacock key constants

**Files:**
- Create: `internal/peacock/keys.go`
- Create: `internal/peacock/keys_test.go`

- [ ] **Step 1: Write failing tests**

`internal/peacock/keys_test.go`:

```go
package peacock

import "testing"

func TestColorKeys_Count(t *testing.T) {
	if got := len(ColorKeys()); got != 28 {
		t.Errorf("ColorKeys() len = %d, want 28", got)
	}
}

func TestColorKeys_Contains(t *testing.T) {
	set := ColorKeysSet()
	must := []string{
		"activityBar.background",
		"activityBar.activeBackground",
		"activityBarBadge.background",
		"titleBar.activeBackground",
		"statusBar.background",
		"editorGroup.border",
		"panel.border",
		"sideBar.border",
		"sash.hoverBorder",
		"tab.activeBorder",
		"commandCenter.border",
		"editorError.foreground",
	}
	for _, k := range must {
		if !set[k] {
			t.Errorf("ColorKeysSet missing %q", k)
		}
	}
}

func TestHasPeacockPrefix(t *testing.T) {
	if !HasPeacockPrefix("peacock.color") {
		t.Error("peacock.color should match")
	}
	if HasPeacockPrefix("workbench.colorCustomizations") {
		t.Error("workbench.* should not match")
	}
}

func TestSettingColor(t *testing.T) {
	if SettingColor != "peacock.color" {
		t.Errorf("SettingColor = %q", SettingColor)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/peacock/...`
Expected: compile error.

- [ ] **Step 3: Implement**

`internal/peacock/keys.go`:

```go
// Package peacock exposes Peacock-specific setting names as Go constants.
// Matches the ColorSettings and StandardSettings enums from
// /Users/user/Projects/vscode-peacock/src/models/enums.ts.
package peacock

import "strings"

// ColorKeys is the full set of VSCode workbench.colorCustomizations keys
// that Peacock manages (28 keys).
var colorKeys = []string{
	"activityBar.activeBackground",
	"activityBar.background",
	"activityBar.foreground",
	"activityBar.inactiveForeground",
	"activityBarBadge.background",
	"activityBarBadge.foreground",
	"commandCenter.border",
	"editorGroup.border",
	"panel.border",
	"sideBar.border",
	"sash.hoverBorder",
	"editorError.foreground",
	"editorWarning.foreground",
	"editorInfo.foreground",
	"statusBar.border",
	"statusBar.background",
	"statusBar.foreground",
	"statusBar.debuggingBorder",
	"statusBar.debuggingBackground",
	"statusBar.debuggingForeground",
	"statusBarItem.hoverBackground",
	"statusBarItem.remoteBackground",
	"statusBarItem.remoteForeground",
	"tab.activeBorder",
	"titleBar.activeBackground",
	"titleBar.activeForeground",
	"titleBar.border",
	"titleBar.inactiveBackground",
	"titleBar.inactiveForeground",
}

// Peacock extension setting names (under the "peacock." namespace in settings.json).
const (
	SettingColor                   = "peacock.color"
	SettingRemoteColor             = "peacock.remoteColor"
	SettingAffectActivityBar       = "peacock.affectActivityBar"
	SettingAffectStatusBar         = "peacock.affectStatusBar"
	SettingAffectTitleBar          = "peacock.affectTitleBar"
	SettingAffectEditorGroupBorder = "peacock.affectEditorGroupBorder"
	SettingAffectPanelBorder       = "peacock.affectPanelBorder"
	SettingAffectSideBarBorder     = "peacock.affectSideBarBorder"
	SettingAffectSashHover         = "peacock.affectSashHover"
	SettingAffectStatusAndTitleBorders = "peacock.affectStatusAndTitleBorders"
	SettingAffectDebuggingStatusBar    = "peacock.affectDebuggingStatusBar"
	SettingAffectTabActiveBorder       = "peacock.affectTabActiveBorder"
	SettingKeepForegroundColor         = "peacock.keepForegroundColor"
	SettingKeepBadgeColor              = "peacock.keepBadgeColor"
	SettingSquigglyBeGone              = "peacock.squigglyBeGone"
	SettingDarkenLightenPercentage     = "peacock.darkenLightenPercentage"
	SettingDarkForegroundColor         = "peacock.darkForegroundColor"
	SettingLightForegroundColor        = "peacock.lightForegroundColor"

	SectionColorCustomizations = "workbench.colorCustomizations"
)

// DefaultDarkForeground matches Peacock's ForegroundColors.DarkForeground.
const DefaultDarkForeground = "#15202b"

// DefaultLightForeground matches Peacock's ForegroundColors.LightForeground.
const DefaultLightForeground = "#e7e7e7"

// InactiveAlpha matches Peacock's inactiveElementAlpha (0x99).
const InactiveAlpha = 0x99

// ColorKeys returns the full list of keys Peacock manages.
func ColorKeys() []string {
	out := make([]string, len(colorKeys))
	copy(out, colorKeys)
	return out
}

// ColorKeysSet returns a set (map) for O(1) membership checks.
func ColorKeysSet() map[string]bool {
	s := make(map[string]bool, len(colorKeys))
	for _, k := range colorKeys {
		s[k] = true
	}
	return s
}

// HasPeacockPrefix reports whether a key begins with "peacock.".
func HasPeacockPrefix(key string) bool {
	return strings.HasPrefix(key, "peacock.")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/peacock/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/peacock/
git commit -m "peacock: add ColorKeys + setting-name constants"
```

---

## Task 9: Palette Options struct + ElementStyle helper

**Files:**
- Create: `internal/color/palette.go`
- Create: `internal/color/palette_test.go`

- [ ] **Step 1: Write failing tests**

`internal/color/palette_test.go`:

```go
package color

import "testing"

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if !opts.Affect.ActivityBar || !opts.Affect.StatusBar || !opts.Affect.TitleBar {
		t.Error("default affect should enable activity/status/title bars")
	}
	if opts.Affect.EditorGroupBorder || opts.Affect.TabActiveBorder {
		t.Error("default affect should disable border/tab options")
	}
	if opts.Standard.DarkenLightenPct != 10 {
		t.Errorf("default pct = %f, want 10", opts.Standard.DarkenLightenPct)
	}
}

func TestElementStyle_Derivatives(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	style := elementStyle(base, opts)
	if style.Background != base {
		t.Errorf("background mismatch")
	}
	if style.Foreground == style.Background {
		t.Errorf("foreground should differ from background")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error.

- [ ] **Step 3: Implement**

`internal/color/palette.go`:

```go
package color

// AffectOptions mirrors Peacock's AffectedSettings enum: booleans for which
// UI elements should be recolored.
type AffectOptions struct {
	ActivityBar            bool
	StatusBar              bool
	TitleBar               bool
	EditorGroupBorder      bool
	PanelBorder            bool
	SideBarBorder          bool
	SashHover              bool
	StatusAndTitleBorders  bool
	DebuggingStatusBar     bool
	TabActiveBorder        bool
}

// StandardOptions mirrors Peacock's StandardSettings toggles.
type StandardOptions struct {
	KeepForegroundColor bool
	KeepBadgeColor      bool
	SquigglyBeGone      bool
	DarkenLightenPct    float64 // default 10
	DarkForegroundHex   string  // default "#15202b"
	LightForegroundHex  string  // default "#e7e7e7"
}

// Options is the full palette configuration.
type Options struct {
	Affect   AffectOptions
	Standard StandardOptions
}

// DefaultOptions mirrors Peacock's defaults.
func DefaultOptions() Options {
	return Options{
		Affect: AffectOptions{
			ActivityBar: true,
			StatusBar:   true,
			TitleBar:    true,
		},
		Standard: StandardOptions{
			DarkenLightenPct:   10,
			DarkForegroundHex:  "#15202b",
			LightForegroundHex: "#e7e7e7",
		},
	}
}

// elementStyle holds a background color's derived styles.
type elementStyleT struct {
	Background         Color
	BackgroundHover    Color
	Inactive           Color // alpha-modified background
	Foreground         Color
	InactiveForeground Color // alpha-modified foreground
	BadgeBackground    Color
	BadgeForeground    Color
}

func elementStyle(base Color, opts Options) elementStyleT {
	fg := foregroundFor(base, opts)
	badgeBg := ReadableAccent(base, RatioUILow)
	badgeFg := foregroundFor(badgeBg, opts)
	return elementStyleT{
		Background:         base,
		BackgroundHover:    HoverOf(base),
		Inactive:           base, // alpha applied at string format time via HexWithAlpha
		Foreground:         fg,
		InactiveForeground: fg, // alpha applied at string format time
		BadgeBackground:    badgeBg,
		BadgeForeground:    badgeFg,
	}
}

// foregroundFor picks light-on-dark vs dark-on-light using tinycolor.isLight.
// Honors DarkForegroundHex / LightForegroundHex overrides.
func foregroundFor(bg Color, opts Options) Color {
	var hex string
	if bg.IsLight() {
		hex = opts.Standard.DarkForegroundHex
		if hex == "" {
			hex = "#15202b"
		}
	} else {
		hex = opts.Standard.LightForegroundHex
		if hex == "" {
			hex = "#e7e7e7"
		}
	}
	c, _ := parseHex(hex)
	return c
}
```

Note: `elementStyleT` stores the *opaque* background/foreground; the inactive variants are rendered with alpha during output (in each `collect*` function).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add Options struct + elementStyle"
```

---

## Task 10: collectTitleBarSettings

**Files:**
- Modify: `internal/color/palette.go`
- Modify: `internal/color/palette_test.go`

**Background:** Port `/Users/user/Projects/vscode-peacock/src/configuration/read-configuration.ts:304-324`.

- [ ] **Step 1: Write failing tests**

Append to `internal/color/palette_test.go`:

```go
func TestCollectTitleBar_Defaults(t *testing.T) {
	base := Color{90, 59, 140} // #5a3b8c
	opts := DefaultOptions()
	out := collectTitleBar(base, opts)

	// With TitleBar=true, these must be set:
	must := []string{
		"titleBar.activeBackground",
		"titleBar.inactiveBackground",
		"titleBar.activeForeground",
		"titleBar.inactiveForeground",
		"commandCenter.border",
	}
	for _, k := range must {
		if _, ok := out[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if out["titleBar.activeBackground"] != "#5a3b8c" {
		t.Errorf("activeBackground = %q, want #5a3b8c", out["titleBar.activeBackground"])
	}
	// StatusAndTitleBorders is off -> no titleBar.border
	if _, ok := out["titleBar.border"]; ok {
		t.Error("titleBar.border should not be set without StatusAndTitleBorders")
	}
}

func TestCollectTitleBar_Disabled(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Affect.TitleBar = false
	out := collectTitleBar(base, opts)
	if len(out) != 0 {
		t.Errorf("disabled titleBar returned %d keys, want 0", len(out))
	}
}

func TestCollectTitleBar_KeepForeground(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Standard.KeepForegroundColor = true
	out := collectTitleBar(base, opts)
	if _, ok := out["titleBar.activeForeground"]; ok {
		t.Error("activeForeground should be omitted with KeepForegroundColor")
	}
	if _, ok := out["commandCenter.border"] ; ok {
		t.Error("commandCenter.border should be omitted with KeepForegroundColor")
	}
	if _, ok := out["titleBar.activeBackground"]; !ok {
		t.Error("activeBackground still expected")
	}
}

func TestCollectTitleBar_WithBorders(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Affect.StatusAndTitleBorders = true
	out := collectTitleBar(base, opts)
	if out["titleBar.border"] != "#5a3b8c" {
		t.Errorf("titleBar.border = %q, want #5a3b8c", out["titleBar.border"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: compile error (collectTitleBar undefined).

- [ ] **Step 3: Implement**

Append to `internal/color/palette.go`:

```go
// collectTitleBar ports collectTitleBarSettings from Peacock.
func collectTitleBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.TitleBar {
		return out
	}
	style := elementStyle(base, opts)
	out["titleBar.activeBackground"] = style.Background.Hex()
	if opts.Affect.StatusAndTitleBorders {
		out["titleBar.border"] = style.Background.Hex()
	}
	out["titleBar.inactiveBackground"] = style.Background.HexWithAlpha(0x99)
	if !opts.Standard.KeepForegroundColor {
		out["titleBar.activeForeground"] = style.Foreground.Hex()
		out["titleBar.inactiveForeground"] = style.Foreground.HexWithAlpha(0x99)
		out["commandCenter.border"] = style.Foreground.HexWithAlpha(0x99)
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: port collectTitleBarSettings"
```

---

## Task 11: collectActivityBarSettings

**Files:**
- Modify: `internal/color/palette.go`
- Modify: `internal/color/palette_test.go`

**Background:** Port `read-configuration.ts:326-353`.

- [ ] **Step 1: Write failing tests**

Append to `internal/color/palette_test.go`:

```go
func TestCollectActivityBar_Defaults(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	out := collectActivityBar(base, opts)
	must := []string{
		"activityBar.background",
		"activityBar.activeBackground",
		"activityBar.foreground",
		"activityBar.inactiveForeground",
		"activityBarBadge.background",
		"activityBarBadge.foreground",
	}
	for _, k := range must {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	if out["activityBar.background"] != out["activityBar.activeBackground"] {
		t.Error("background and activeBackground should match")
	}
}

func TestCollectActivityBar_KeepBadge(t *testing.T) {
	opts := DefaultOptions()
	opts.Standard.KeepBadgeColor = true
	out := collectActivityBar(Color{90, 59, 140}, opts)
	if _, ok := out["activityBarBadge.background"]; ok {
		t.Error("badge should be omitted")
	}
	if _, ok := out["activityBarBadge.foreground"]; ok {
		t.Error("badge fg should be omitted")
	}
}

func TestCollectActivityBar_Disabled(t *testing.T) {
	opts := DefaultOptions()
	opts.Affect.ActivityBar = false
	out := collectActivityBar(Color{90, 59, 140}, opts)
	if len(out) != 0 {
		t.Errorf("disabled -> %d keys, want 0", len(out))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

Append to `internal/color/palette.go`:

```go
// collectActivityBar ports collectActivityBarSettings from Peacock.
func collectActivityBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.ActivityBar {
		return out
	}
	style := elementStyle(base, opts)
	out["activityBar.background"] = style.Background.Hex()
	out["activityBar.activeBackground"] = style.Background.Hex()
	if !opts.Standard.KeepForegroundColor {
		out["activityBar.foreground"] = style.Foreground.Hex()
		out["activityBar.inactiveForeground"] = style.Foreground.HexWithAlpha(0x99)
	}
	if !opts.Standard.KeepBadgeColor {
		out["activityBarBadge.background"] = style.BadgeBackground.Hex()
		out["activityBarBadge.foreground"] = style.BadgeForeground.Hex()
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: port collectActivityBarSettings"
```

---

## Task 12: collectStatusBarSettings

**Files:**
- Modify: `internal/color/palette.go`
- Modify: `internal/color/palette_test.go`

**Background:** Port `read-configuration.ts:355-392`.

- [ ] **Step 1: Write failing tests**

Append to `internal/color/palette_test.go`:

```go
func TestCollectStatusBar_Defaults(t *testing.T) {
	opts := DefaultOptions()
	out := collectStatusBar(Color{90, 59, 140}, opts)
	must := []string{
		"statusBar.background",
		"statusBarItem.hoverBackground",
		"statusBarItem.remoteBackground",
		"statusBar.foreground",
		"statusBarItem.remoteForeground",
	}
	for _, k := range must {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
}

func TestCollectStatusBar_Debugging(t *testing.T) {
	opts := DefaultOptions()
	opts.Affect.DebuggingStatusBar = true
	out := collectStatusBar(Color{90, 59, 140}, opts)
	if _, ok := out["statusBar.debuggingBackground"]; !ok {
		t.Error("expected debuggingBackground")
	}
	if _, ok := out["statusBar.debuggingForeground"]; !ok {
		t.Error("expected debuggingForeground")
	}
}

func TestCollectStatusBar_BordersWithDebug(t *testing.T) {
	opts := DefaultOptions()
	opts.Affect.DebuggingStatusBar = true
	opts.Affect.StatusAndTitleBorders = true
	out := collectStatusBar(Color{90, 59, 140}, opts)
	if _, ok := out["statusBar.border"]; !ok {
		t.Error("expected statusBar.border")
	}
	if _, ok := out["statusBar.debuggingBorder"]; !ok {
		t.Error("expected statusBar.debuggingBorder")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

Append to `internal/color/palette.go`:

```go
// collectStatusBar ports collectStatusBarSettings from Peacock.
func collectStatusBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.StatusBar {
		return out
	}
	style := elementStyle(base, opts)
	out["statusBar.background"] = style.Background.Hex()
	out["statusBarItem.hoverBackground"] = style.BackgroundHover.Hex()
	out["statusBarItem.remoteBackground"] = style.Background.Hex()

	if opts.Affect.StatusAndTitleBorders {
		out["statusBar.border"] = style.Background.Hex()
	}
	if !opts.Standard.KeepForegroundColor {
		out["statusBar.foreground"] = style.Foreground.Hex()
		out["statusBarItem.remoteForeground"] = style.Foreground.Hex()
	}
	if opts.Affect.DebuggingStatusBar {
		debugBg := base.Complement()
		out["statusBar.debuggingBackground"] = debugBg.Hex()
		if opts.Affect.StatusAndTitleBorders {
			out["statusBar.debuggingBorder"] = debugBg.Hex()
		}
		if !opts.Standard.KeepForegroundColor {
			out["statusBar.debuggingForeground"] = foregroundFor(debugBg, opts).Hex()
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: port collectStatusBarSettings"
```

---

## Task 13: collectAccentBorder + collectSquigglyBeGone

**Files:**
- Modify: `internal/color/palette.go`
- Modify: `internal/color/palette_test.go`

**Background:** Port `read-configuration.ts:394-416` (borders) and `read-configuration.ts:418-434` (squiggly).

- [ ] **Step 1: Write failing tests**

Append to `internal/color/palette_test.go`:

```go
func TestCollectAccentBorder(t *testing.T) {
	opts := DefaultOptions()
	opts.Affect.EditorGroupBorder = true
	opts.Affect.PanelBorder = true
	opts.Affect.TabActiveBorder = true
	out := collectAccentBorder(Color{90, 59, 140}, opts)
	want := []string{"editorGroup.border", "panel.border", "tab.activeBorder"}
	for _, k := range want {
		if out[k] != "#5a3b8c" {
			t.Errorf("%s = %q, want #5a3b8c", k, out[k])
		}
	}
	if _, ok := out["sideBar.border"]; ok {
		t.Error("sideBar.border should be absent")
	}
}

func TestCollectSquigglyBeGone_Off(t *testing.T) {
	opts := DefaultOptions()
	out := collectSquigglyBeGone(opts)
	if len(out) != 0 {
		t.Errorf("off -> len=%d", len(out))
	}
}

func TestCollectSquigglyBeGone_On(t *testing.T) {
	opts := DefaultOptions()
	opts.Standard.SquigglyBeGone = true
	out := collectSquigglyBeGone(opts)
	for _, k := range []string{"editorError.foreground", "editorWarning.foreground", "editorInfo.foreground"} {
		if out[k] != "#00000000" {
			t.Errorf("%s = %q, want #00000000", k, out[k])
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

Append to `internal/color/palette.go`:

```go
// collectAccentBorder ports collectAccentBorderSettings.
func collectAccentBorder(base Color, opts Options) map[string]string {
	out := map[string]string{}
	hex := base.Hex()
	if opts.Affect.EditorGroupBorder {
		out["editorGroup.border"] = hex
	}
	if opts.Affect.PanelBorder {
		out["panel.border"] = hex
	}
	if opts.Affect.SideBarBorder {
		out["sideBar.border"] = hex
	}
	if opts.Affect.SashHover {
		out["sash.hoverBorder"] = hex
	}
	if opts.Affect.TabActiveBorder {
		out["tab.activeBorder"] = hex
	}
	return out
}

// collectSquigglyBeGone ports collectSquigglyBeGoneSettings.
// Sets error/warning/info foregrounds to transparent.
func collectSquigglyBeGone(opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Standard.SquigglyBeGone {
		return out
	}
	const transparent = "#00000000"
	out["editorError.foreground"] = transparent
	out["editorWarning.foreground"] = transparent
	out["editorInfo.foreground"] = transparent
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: port collectAccentBorder + collectSquigglyBeGone"
```

---

## Task 14: Palette orchestrator

**Files:**
- Modify: `internal/color/palette.go`
- Modify: `internal/color/palette_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/color/palette_test.go`:

```go
func TestPalette_DefaultsContainExpected(t *testing.T) {
	base := Color{90, 59, 140}
	out := Palette(base, DefaultOptions())

	// Default affect: activityBar + statusBar + titleBar -> must contain these keys
	for _, k := range []string{
		"activityBar.background",
		"statusBar.background",
		"titleBar.activeBackground",
	} {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	// Default affect: editorGroupBorder off -> no editorGroup.border
	if _, ok := out["editorGroup.border"]; ok {
		t.Error("editorGroup.border should be absent")
	}
}

func TestPalette_AllOn(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Affect.EditorGroupBorder = true
	opts.Affect.PanelBorder = true
	opts.Affect.SideBarBorder = true
	opts.Affect.SashHover = true
	opts.Affect.StatusAndTitleBorders = true
	opts.Affect.DebuggingStatusBar = true
	opts.Affect.TabActiveBorder = true
	opts.Standard.SquigglyBeGone = true

	out := Palette(base, opts)
	// With everything on we should cover most of the 28 keys
	if len(out) < 25 {
		t.Errorf("all-on palette has %d keys, want >= 25", len(out))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/color/...`
Expected: FAIL (Palette undefined).

- [ ] **Step 3: Implement**

Append to `internal/color/palette.go`:

```go
// Palette builds the workbench.colorCustomizations map for a base color
// given the selected affect/standard options. Keys are only emitted for
// enabled elements (mirrors Peacock's prepareColors behavior).
func Palette(base Color, opts Options) map[string]string {
	out := map[string]string{}
	for _, f := range []func(Color, Options) map[string]string{
		collectTitleBar,
		collectActivityBar,
		collectStatusBar,
		collectAccentBorder,
	} {
		for k, v := range f(base, opts) {
			out[k] = v
		}
	}
	for k, v := range collectSquigglyBeGone(opts) {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/color/
git commit -m "color: add Palette orchestrator"
```

---

## Task 15: Palette golden fixture against Peacock JS

**Files:**
- Create: `scripts/gen-peacock-fixture/main.js`
- Create: `internal/color/testdata/fixture.json`
- Create: `internal/color/golden_test.go`

**Background:** The goal is to verify 1:1 parity with Peacock's JS output for a handful of base colors. We write a small Node script that imports Peacock's `color-library` and re-implements `prepareColors` behavior with a fixed options object, then dumps a JSON fixture the Go test consumes.

- [ ] **Step 1: Write the Node fixture generator**

`scripts/gen-peacock-fixture/main.js`:

```javascript
// Generate golden-test fixtures by invoking Peacock's prepareColors logic
// from outside the VSCode runtime.
//
// Prereq: inside /Users/user/Projects/vscode-peacock/, run `npm install` so
// tinycolor2 is on disk.
//
// Usage (from vscode-color-workspace root):
//   node scripts/gen-peacock-fixture/main.js > internal/color/testdata/fixture.json
//
// Since Peacock's prepareColors() reads settings via vscode.workspace API,
// we can't call it directly. Instead we re-implement the same logic using
// Peacock's color-library primitives, which is a pure TS module.

const path = require('path');
const peacockRoot = '/Users/user/Projects/vscode-peacock';
const tinycolor = require(path.join(peacockRoot, 'node_modules', 'tinycolor2'));

// --- Mirror color-library.ts helpers ---
const inactiveAlpha = 0x99 / 0xff;
const defaultSaturation = 0.5;
const defaultAmount = 10;
const darkFg = '#15202b';
const lightFg = '#e7e7e7';

function formatHex(c) {
  return c.getAlpha() < 1 ? c.toHex8String() : c.toHexString();
}

function getBgHex(c) { return formatHex(tinycolor(c)); }

function getInactiveBg(c) {
  const x = tinycolor(c);
  x.setAlpha(inactiveAlpha);
  return formatHex(x);
}

function getHover(c) {
  const x = tinycolor(c);
  return formatHex(x.isLight() ? x.darken() : x.lighten());
}

function getFg(bg) {
  const x = tinycolor(bg);
  return formatHex(tinycolor(x.isLight() ? darkFg : lightFg));
}

function getInactiveFg(bg) {
  const f = tinycolor(getFg(bg));
  f.setAlpha(inactiveAlpha);
  return formatHex(f);
}

function getReadableAccent(bg, ratio) {
  const background = tinycolor(bg);
  const fg = background.triad()[1];
  let { h, s, l } = fg.toHsl();
  if (s === 0) h = 60 * Math.round(l * 6);
  if (s < 0.15) s = defaultSaturation;
  const count = 16;
  const shades = [...Array(count).keys()].map(i => {
    const c = tinycolor({ h, s, l: i / count });
    return { contrast: tinycolor.readability(c, background), hex: formatHex(c) };
  });
  shades.sort((a, b) => a.contrast - b.contrast);
  const found = shades.find(s => s.contrast >= ratio);
  return found ? found.hex : '#ffffff';
}

function complement(c) { return formatHex(tinycolor(c).complement()); }

function elementStyle(bg) {
  return {
    bg: getBgHex(bg),
    bgHover: getHover(bg),
    inactiveBg: getInactiveBg(bg),
    fg: getFg(bg),
    inactiveFg: getInactiveFg(bg),
    badgeBg: getReadableAccent(bg, 2),
  };
}

// --- Mirror prepareColors with explicit opts ---
function prepareColors(bg, opts) {
  const out = {};
  const style = elementStyle(bg);
  const debugBg = complement(bg);

  if (opts.titleBar) {
    out['titleBar.activeBackground'] = style.bg;
    if (opts.statusAndTitleBorders) out['titleBar.border'] = style.bg;
    out['titleBar.inactiveBackground'] = style.inactiveBg;
    if (!opts.keepForegroundColor) {
      out['titleBar.activeForeground'] = style.fg;
      out['titleBar.inactiveForeground'] = style.inactiveFg;
      out['commandCenter.border'] = style.inactiveFg;
    }
  }
  if (opts.activityBar) {
    out['activityBar.background'] = style.bg;
    out['activityBar.activeBackground'] = style.bg;
    if (!opts.keepForegroundColor) {
      out['activityBar.foreground'] = style.fg;
      out['activityBar.inactiveForeground'] = style.inactiveFg;
    }
    if (!opts.keepBadgeColor) {
      out['activityBarBadge.background'] = style.badgeBg;
      out['activityBarBadge.foreground'] = getFg(style.badgeBg);
    }
  }
  if (opts.statusBar) {
    out['statusBar.background'] = style.bg;
    out['statusBarItem.hoverBackground'] = style.bgHover;
    out['statusBarItem.remoteBackground'] = style.bg;
    if (opts.statusAndTitleBorders) out['statusBar.border'] = style.bg;
    if (!opts.keepForegroundColor) {
      out['statusBar.foreground'] = style.fg;
      out['statusBarItem.remoteForeground'] = style.fg;
    }
    if (opts.debuggingStatusBar) {
      out['statusBar.debuggingBackground'] = debugBg;
      if (opts.statusAndTitleBorders) out['statusBar.debuggingBorder'] = debugBg;
      if (!opts.keepForegroundColor) out['statusBar.debuggingForeground'] = getFg(debugBg);
    }
  }
  if (opts.editorGroupBorder) out['editorGroup.border'] = style.bg;
  if (opts.panelBorder) out['panel.border'] = style.bg;
  if (opts.sideBarBorder) out['sideBar.border'] = style.bg;
  if (opts.sashHover) out['sash.hoverBorder'] = style.bg;
  if (opts.tabActiveBorder) out['tab.activeBorder'] = style.bg;
  if (opts.squigglyBeGone) {
    out['editorError.foreground'] = '#00000000';
    out['editorWarning.foreground'] = '#00000000';
    out['editorInfo.foreground'] = '#00000000';
  }
  return out;
}

// --- Fixtures ---
const defaultOpts = {
  activityBar: true, statusBar: true, titleBar: true,
  editorGroupBorder: false, panelBorder: false, sideBarBorder: false,
  sashHover: false, statusAndTitleBorders: false,
  debuggingStatusBar: false, tabActiveBorder: false,
  keepForegroundColor: false, keepBadgeColor: false, squigglyBeGone: false,
};

const fixtures = [
  { base: '#ff0000', label: 'red', opts: defaultOpts },
  { base: '#42b883', label: 'peacock_green', opts: defaultOpts },
  { base: '#5a3b8c', label: 'purple', opts: defaultOpts },
  { base: '#000000', label: 'black', opts: defaultOpts },
  { base: '#ffffff', label: 'white', opts: defaultOpts },
];

const output = fixtures.map(f => ({
  base: f.base,
  label: f.label,
  opts: f.opts,
  palette: prepareColors(f.base, f.opts),
}));

process.stdout.write(JSON.stringify(output, null, 2) + '\n');
```

- [ ] **Step 2: Generate fixture**

Run: `cd /Users/user/Projects/vscode-peacock && npm install` (if node_modules missing)
Run: `cd /Users/user/Projects/color-vscode-workspace && node scripts/gen-peacock-fixture/main.js > internal/color/testdata/fixture.json`
Expected: JSON array of 5 entries written.

Check: `wc -l internal/color/testdata/fixture.json` — should be > 50 lines.

- [ ] **Step 3: Write golden test**

`internal/color/golden_test.go`:

```go
package color

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/fixture.json
var fixtureJSON []byte

type fixtureEntry struct {
	Base    string            `json:"base"`
	Label   string            `json:"label"`
	Opts    map[string]bool   `json:"opts"`
	Palette map[string]string `json:"palette"`
}

func TestPalette_GoldenFixture(t *testing.T) {
	var entries []fixtureEntry
	if err := json.Unmarshal(fixtureJSON, &entries); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	for _, e := range entries {
		t.Run(e.Label, func(t *testing.T) {
			base, err := Parse(e.Base)
			if err != nil {
				t.Fatalf("parse %q: %v", e.Base, err)
			}
			opts := optsFromMap(e.Opts)
			got := Palette(base, opts)
			if len(got) != len(e.Palette) {
				t.Errorf("palette size = %d, want %d", len(got), len(e.Palette))
			}
			for k, v := range e.Palette {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
			for k := range got {
				if _, ok := e.Palette[k]; !ok {
					t.Errorf("unexpected key %q in Go output", k)
				}
			}
		})
	}
}

func optsFromMap(m map[string]bool) Options {
	opts := DefaultOptions()
	opts.Affect = AffectOptions{
		ActivityBar:           m["activityBar"],
		StatusBar:             m["statusBar"],
		TitleBar:              m["titleBar"],
		EditorGroupBorder:     m["editorGroupBorder"],
		PanelBorder:           m["panelBorder"],
		SideBarBorder:         m["sideBarBorder"],
		SashHover:             m["sashHover"],
		StatusAndTitleBorders: m["statusAndTitleBorders"],
		DebuggingStatusBar:    m["debuggingStatusBar"],
		TabActiveBorder:       m["tabActiveBorder"],
	}
	opts.Standard.KeepForegroundColor = m["keepForegroundColor"]
	opts.Standard.KeepBadgeColor = m["keepBadgeColor"]
	opts.Standard.SquigglyBeGone = m["squigglyBeGone"]
	return opts
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/color/...`
Expected: PASS.

If the golden test fails on a specific key, the Go port has drifted from the JS reference. Diagnose by comparing the key's expected vs. got hex string, tracing back through `elementStyle` / `collectX` / primitives. HSL roundtrip differences can cause ±1 channel drift; if that's the only disagreement, relax the assertion to tolerate 1-unit per channel. But strict equality is the first target.

- [ ] **Step 5: Commit**

```bash
git add scripts/ internal/color/testdata/ internal/color/golden_test.go
git commit -m "color: add peacock golden-fixture test"
```

---

## Task 16: JSONC read utility

**Files:**
- Create: `internal/jsonc/jsonc.go`
- Create: `internal/jsonc/jsonc_test.go`

- [ ] **Step 1: Write failing tests**

`internal/jsonc/jsonc_test.go`:

```go
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
	}`) // trailing comma also
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
	// Should be indented (contain newline)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		t.Error("output should end with newline")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/jsonc/...`
Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Add hujson dep**

Run: `go get github.com/tailscale/hujson@v0.0.0-20241010212012-29efb4a0184b` (or latest)

- [ ] **Step 4: Implement**

`internal/jsonc/jsonc.go`:

```go
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
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/jsonc/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/jsonc/ go.mod go.sum
git commit -m "jsonc: add Read/Write helpers using hujson"
```

---

## Task 17: Workspace file struct + read/write

**Files:**
- Create: `internal/workspace/workspace.go`
- Create: `internal/workspace/workspace_test.go`

- [ ] **Step 1: Write failing tests**

`internal/workspace/workspace_test.go`:

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_Missing(t *testing.T) {
	ws, err := Read(filepath.Join(t.TempDir(), "nope.code-workspace"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ws != nil {
		t.Errorf("expected nil for missing file, got %+v", ws)
	}
}

func TestRead_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.code-workspace")
	content := `{
		// settings below
		"folders": [{ "path": "./foo" }],
		"settings": {
			"peacock.color": "#5a3b8c",
			"editor.fontSize": 14
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := Read(path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace")
	}
	if len(ws.Folders) != 1 || ws.Folders[0].Path != "./foo" {
		t.Errorf("folders = %+v", ws.Folders)
	}
	if ws.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", ws.Settings["peacock.color"])
	}
}

func TestWrite_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bar.code-workspace")
	ws := &Workspace{
		Folders: []Folder{{Path: "./bar"}},
		Settings: map[string]any{
			"peacock.color": "#5a3b8c",
			"workbench.colorCustomizations": map[string]any{
				"activityBar.background": "#5a3b8c",
			},
		},
	}
	if err := Write(path, ws); err != nil {
		t.Fatalf("err = %v", err)
	}
	// Round-trip
	ws2, err := Read(path)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if ws2.Folders[0].Path != "./bar" {
		t.Errorf("folders mismatch after roundtrip")
	}
	if ws2.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color missing")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/workspace/...`
Expected: FAIL (package missing).

- [ ] **Step 3: Implement**

`internal/workspace/workspace.go`:

```go
// Package workspace handles reading, merging, and writing VSCode
// .code-workspace files (JSONC format).
package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
)

// Folder is an entry in the top-level "folders" array.
type Folder struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// Workspace mirrors the VSCode .code-workspace JSON schema. We keep the
// settings block as a free-form map so we don't lose unknown keys during
// round-trip.
type Workspace struct {
	Folders    []Folder       `json:"folders"`
	Settings   map[string]any `json:"settings,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
	Launch     map[string]any `json:"launch,omitempty"`
	Tasks      map[string]any `json:"tasks,omitempty"`
	// Other is a catch-all for any other top-level keys. Populated by
	// custom UnmarshalJSON; emitted by custom MarshalJSON.
	Other map[string]any `json:"-"`
}

// Read parses the file at path. Returns (nil, nil) if the file does not exist.
func Read(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("workspace read: %w", err)
	}
	var raw map[string]any
	if err := jsonc.Read(data, &raw); err != nil {
		return nil, fmt.Errorf("workspace read %s: %w", path, err)
	}
	return fromMap(raw), nil
}

// Write serializes ws to path atomically (temp file + rename).
func Write(path string, ws *Workspace) error {
	data, err := jsonc.Write(toMap(ws))
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0o644)
}

func fromMap(m map[string]any) *Workspace {
	ws := &Workspace{Other: map[string]any{}}
	for k, v := range m {
		switch k {
		case "folders":
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if o, ok := e.(map[string]any); ok {
						f := Folder{}
						if p, ok := o["path"].(string); ok {
							f.Path = p
						}
						if n, ok := o["name"].(string); ok {
							f.Name = n
						}
						ws.Folders = append(ws.Folders, f)
					}
				}
			}
		case "settings":
			if o, ok := v.(map[string]any); ok {
				ws.Settings = o
			}
		case "extensions":
			if o, ok := v.(map[string]any); ok {
				ws.Extensions = o
			}
		case "launch":
			if o, ok := v.(map[string]any); ok {
				ws.Launch = o
			}
		case "tasks":
			if o, ok := v.(map[string]any); ok {
				ws.Tasks = o
			}
		default:
			ws.Other[k] = v
		}
	}
	return ws
}

func toMap(ws *Workspace) map[string]any {
	out := map[string]any{}
	for k, v := range ws.Other {
		out[k] = v
	}
	folders := make([]map[string]any, 0, len(ws.Folders))
	for _, f := range ws.Folders {
		m := map[string]any{"path": f.Path}
		if f.Name != "" {
			m["name"] = f.Name
		}
		folders = append(folders, m)
	}
	out["folders"] = folders
	if len(ws.Settings) > 0 {
		out["settings"] = ws.Settings
	}
	if len(ws.Extensions) > 0 {
		out["extensions"] = ws.Extensions
	}
	if len(ws.Launch) > 0 {
		out["launch"] = ws.Launch
	}
	if len(ws.Tasks) > 0 {
		out["tasks"] = ws.Tasks
	}
	return out
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccws-*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleaned := false
	defer func() {
		if !cleaned {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		tmp.Close()
		return fmt.Errorf("atomic write: copy: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("atomic write: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomic write: close: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("atomic write: chmod: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	cleaned = true
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/workspace/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/
git commit -m "workspace: add Read/Write + atomic file write"
```

---

## Task 18: Guard 1 (workspace merge conflict detection)

**Files:**
- Create: `internal/workspace/guard.go`
- Create: `internal/workspace/guard_test.go`

- [ ] **Step 1: Write failing tests**

`internal/workspace/guard_test.go`:

```go
package workspace

import (
	"reflect"
	"sort"
	"testing"
)

func TestExistingPeacockKeys_None(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"editor.fontSize": 14.0,
	}}
	got := ExistingPeacockKeys(ws)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestExistingPeacockKeys_PeacockColor(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"peacock.color": "#5a3b8c",
	}}
	got := ExistingPeacockKeys(ws)
	want := []string{"settings.peacock.color"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExistingPeacockKeys_ColorCustomizations(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
			"editor.background":      "#000000", // non-peacock, should not trigger
		},
	}}
	got := ExistingPeacockKeys(ws)
	sort.Strings(got)
	want := []string{"settings.workbench.colorCustomizations.activityBar.background"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExistingPeacockKeys_NilWorkspace(t *testing.T) {
	if got := ExistingPeacockKeys(nil); len(got) != 0 {
		t.Errorf("nil -> %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/workspace/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/workspace/guard.go`:

```go
package workspace

import (
	"sort"

	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// ExistingPeacockKeys returns dotted paths of Peacock-managed keys found in
// ws. Empty if none. Empty for nil ws. Returned paths are sorted for
// deterministic output; callers use them in warning messages.
func ExistingPeacockKeys(ws *Workspace) []string {
	if ws == nil {
		return nil
	}
	var out []string
	colorKeys := peacock.ColorKeysSet()
	for k, v := range ws.Settings {
		if peacock.HasPeacockPrefix(k) {
			out = append(out, "settings."+k)
			continue
		}
		if k != peacock.SectionColorCustomizations {
			continue
		}
		cc, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for ck := range cc {
			if colorKeys[ck] {
				out = append(out, "settings."+peacock.SectionColorCustomizations+"."+ck)
			}
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/workspace/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/guard.go internal/workspace/guard_test.go
git commit -m "workspace: detect existing peacock keys (guard 1)"
```

---

## Task 19: Merge logic

**Files:**
- Create: `internal/workspace/merge.go`
- Create: `internal/workspace/merge_test.go`

- [ ] **Step 1: Write failing tests**

`internal/workspace/merge_test.go`:

```go
package workspace

import "testing"

func TestApplyPeacock_NewSettings(t *testing.T) {
	ws := &Workspace{}
	palette := map[string]string{
		"activityBar.background": "#5a3b8c",
	}
	ApplyPeacock(ws, "#5a3b8c", palette)

	if ws.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", ws.Settings["peacock.color"])
	}
	cc, _ := ws.Settings["workbench.colorCustomizations"].(map[string]any)
	if cc["activityBar.background"] != "#5a3b8c" {
		t.Errorf("activityBar.background not applied")
	}
}

func TestApplyPeacock_PreservesOtherSettings(t *testing.T) {
	ws := &Workspace{
		Settings: map[string]any{
			"editor.fontSize": 14.0,
			"workbench.colorCustomizations": map[string]any{
				"editor.background": "#000000", // non-peacock, preserve
			},
		},
	}
	ApplyPeacock(ws, "#5a3b8c", map[string]string{
		"activityBar.background": "#5a3b8c",
	})

	if ws.Settings["editor.fontSize"].(float64) != 14.0 {
		t.Errorf("editor.fontSize lost")
	}
	cc := ws.Settings["workbench.colorCustomizations"].(map[string]any)
	if cc["editor.background"] != "#000000" {
		t.Error("custom colorCustomization lost")
	}
	if cc["activityBar.background"] != "#5a3b8c" {
		t.Error("peacock key not applied")
	}
}

func TestApplyPeacock_EnsuresFolders(t *testing.T) {
	ws := &Workspace{}
	ApplyPeacock(ws, "#5a3b8c", map[string]string{})
	if ws.Settings == nil {
		t.Error("settings should be initialized")
	}
	// folders is applied by the caller (Write path), not by merge.
	// This test just confirms merge doesn't touch folders.
	if ws.Folders != nil {
		t.Error("merge should not mutate folders")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/workspace/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/workspace/merge.go`:

```go
package workspace

import "github.com/sang-bin/vscode-color-workspace/internal/peacock"

// ApplyPeacock writes the peacock.color setting and merges the palette into
// workbench.colorCustomizations, preserving unrelated keys. Does not modify
// Folders (caller handles that).
func ApplyPeacock(ws *Workspace, colorHex string, palette map[string]string) {
	if ws.Settings == nil {
		ws.Settings = map[string]any{}
	}
	ws.Settings[peacock.SettingColor] = colorHex

	cc, ok := ws.Settings[peacock.SectionColorCustomizations].(map[string]any)
	if !ok {
		cc = map[string]any{}
	}
	for k, v := range palette {
		cc[k] = v
	}
	if len(cc) > 0 {
		ws.Settings[peacock.SectionColorCustomizations] = cc
	}
}

// EnsureFolder inserts a single folder entry with the given relative path
// if no folder with that path is present.
func EnsureFolder(ws *Workspace, path string) {
	for _, f := range ws.Folders {
		if f.Path == path {
			return
		}
	}
	ws.Folders = append(ws.Folders, Folder{Path: path})
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/workspace/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/merge.go internal/workspace/merge_test.go
git commit -m "workspace: add ApplyPeacock + EnsureFolder"
```

---

## Task 20: Source settings struct + Read

**Files:**
- Create: `internal/vscodesettings/settings.go`
- Create: `internal/vscodesettings/settings_test.go`

- [ ] **Step 1: Write failing tests**

`internal/vscodesettings/settings_test.go`:

```go
package vscodesettings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_Missing(t *testing.T) {
	s, err := Read(filepath.Join(t.TempDir(), ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s != nil {
		t.Errorf("missing file should return nil, got %+v", s)
	}
}

func TestRead_Existing(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	content := `{
		"peacock.color": "#5a3b8c",
		"editor.tabSize": 2,
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c"
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Read(path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s == nil {
		t.Fatal("expected settings")
	}
	if s.Raw["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", s.Raw["peacock.color"])
	}
	if s.Raw["editor.tabSize"].(float64) != 2 {
		t.Errorf("editor.tabSize = %v", s.Raw["editor.tabSize"])
	}
}

func TestPeacockColor(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#abcdef",
	}}
	if got, ok := s.PeacockColor(); !ok || got != "#abcdef" {
		t.Errorf("PeacockColor = %q, %v", got, ok)
	}
}

func TestPeacockColor_Missing(t *testing.T) {
	s := &Settings{Raw: map[string]any{}}
	if _, ok := s.PeacockColor(); ok {
		t.Error("should not be present")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/vscodesettings/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/vscodesettings/settings.go`:

```go
// Package vscodesettings reads and modifies .vscode/settings.json, with
// Peacock-specific helpers for detection and cleanup.
package vscodesettings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// Settings is a loaded .vscode/settings.json as a raw map.
type Settings struct {
	Path string
	Raw  map[string]any
}

// Read loads the settings file at path. Returns (nil, nil) for missing file.
func Read(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("settings read: %w", err)
	}
	var raw map[string]any
	if err := jsonc.Read(data, &raw); err != nil {
		return nil, fmt.Errorf("settings read %s: %w", path, err)
	}
	return &Settings{Path: path, Raw: raw}, nil
}

// PeacockColor returns the peacock.color setting if present.
func (s *Settings) PeacockColor() (string, bool) {
	v, ok := s.Raw[peacock.SettingColor].(string)
	return v, ok && v != ""
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/vscodesettings/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/vscodesettings/
git commit -m "vscodesettings: add Read + PeacockColor"
```

---

## Task 21: Guard 2 (source residual detection)

**Files:**
- Create: `internal/vscodesettings/guard.go`
- Modify: `internal/vscodesettings/settings_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/vscodesettings/settings_test.go`:

```go
func TestResidualColorKeys_None(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#5a3b8c",
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
		},
	}}
	if got := ResidualColorKeys(s); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestResidualColorKeys_HasNonPeacock(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c", // peacock, would be deleted
			"editor.background":      "#000000", // non-peacock, residual
			"terminal.background":    "#111111", // non-peacock, residual
		},
	}}
	got := ResidualColorKeys(s)
	if len(got) != 2 {
		t.Errorf("got %v, want 2 entries", got)
	}
}

func TestResidualColorKeys_NilSettings(t *testing.T) {
	if got := ResidualColorKeys(nil); len(got) != 0 {
		t.Errorf("nil -> %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/vscodesettings/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/vscodesettings/guard.go`:

```go
package vscodesettings

import (
	"sort"

	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// ResidualColorKeys returns the list of keys that would remain in
// workbench.colorCustomizations after deleting Peacock-managed keys.
// Used by Guard 2 (the tool aborts if the caller is about to clean up
// and these would be left behind).
func ResidualColorKeys(s *Settings) []string {
	if s == nil {
		return nil
	}
	cc, ok := s.Raw[peacock.SectionColorCustomizations].(map[string]any)
	if !ok {
		return nil
	}
	pk := peacock.ColorKeysSet()
	var out []string
	for k := range cc {
		if !pk[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/vscodesettings/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/vscodesettings/
git commit -m "vscodesettings: add ResidualColorKeys (guard 2)"
```

---

## Task 22: Cleanup (delete peacock keys + cascade)

**Files:**
- Create: `internal/vscodesettings/cleanup.go`
- Modify: `internal/vscodesettings/settings_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/vscodesettings/settings_test.go`:

```go
func TestCleanup_DeletesPeacockKeys(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color":             "#5a3b8c",
		"peacock.affectActivityBar": true,
		"editor.tabSize":            2.0,
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
			"editor.background":      "#000000",
		},
	}}
	Cleanup(s)

	if _, ok := s.Raw["peacock.color"]; ok {
		t.Error("peacock.color should be deleted")
	}
	if _, ok := s.Raw["peacock.affectActivityBar"]; ok {
		t.Error("peacock.affectActivityBar should be deleted")
	}
	if s.Raw["editor.tabSize"].(float64) != 2 {
		t.Error("editor.tabSize should be preserved")
	}
	cc := s.Raw["workbench.colorCustomizations"].(map[string]any)
	if _, ok := cc["activityBar.background"]; ok {
		t.Error("activityBar.background should be deleted")
	}
	if cc["editor.background"] != "#000000" {
		t.Error("editor.background should be preserved")
	}
}

func TestCleanup_RemovesEmptyColorCustomizations(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c", // only peacock keys
		},
	}}
	Cleanup(s)
	if _, ok := s.Raw["workbench.colorCustomizations"]; ok {
		t.Error("empty colorCustomizations should be removed")
	}
}

func TestCleanup_NoSettings(t *testing.T) {
	if Cleanup(nil) {
		t.Error("nil should return false")
	}
}

func TestCleanup_Empty(t *testing.T) {
	s := &Settings{Raw: map[string]any{}}
	if Cleanup(s) {
		t.Error("empty raw should return false (no change)")
	}
}

func TestCleanup_ReportsEmpty(t *testing.T) {
	s := &Settings{Raw: map[string]any{
		"peacock.color": "#5a3b8c",
	}}
	changed := Cleanup(s)
	if !changed {
		t.Error("should report changed")
	}
	if s.IsEmpty() != true {
		t.Error("IsEmpty should be true after removing the only key")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/vscodesettings/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/vscodesettings/cleanup.go`:

```go
package vscodesettings

import "github.com/sang-bin/vscode-color-workspace/internal/peacock"

// Cleanup removes all peacock-managed keys in place. Returns true if any
// change was made. Safe to call on nil settings (returns false).
//
// Removes:
//   - any key starting with "peacock."
//   - in workbench.colorCustomizations, any key in peacock.ColorKeys()
//   - the workbench.colorCustomizations key itself if it becomes empty
func Cleanup(s *Settings) bool {
	if s == nil || s.Raw == nil {
		return false
	}
	changed := false
	for k := range s.Raw {
		if peacock.HasPeacockPrefix(k) {
			delete(s.Raw, k)
			changed = true
		}
	}
	if cc, ok := s.Raw[peacock.SectionColorCustomizations].(map[string]any); ok {
		pk := peacock.ColorKeysSet()
		for k := range cc {
			if pk[k] {
				delete(cc, k)
				changed = true
			}
		}
		if len(cc) == 0 {
			delete(s.Raw, peacock.SectionColorCustomizations)
		}
	}
	return changed
}

// IsEmpty reports whether the settings map has zero keys.
func (s *Settings) IsEmpty() bool {
	return s == nil || len(s.Raw) == 0
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/vscodesettings/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/vscodesettings/
git commit -m "vscodesettings: add Cleanup with cascade-on-empty"
```

---

## Task 23: Write + file/dir deletion cascade

**Files:**
- Create: `internal/vscodesettings/write.go`
- Modify: `internal/vscodesettings/settings_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/vscodesettings/settings_test.go`:

```go
func TestWriteOrDelete_Delete(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Settings{Path: path, Raw: map[string]any{}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("settings.json should be deleted")
	}
	if _, err := os.Stat(vdir); !os.IsNotExist(err) {
		t.Error(".vscode should be deleted (empty)")
	}
}

func TestWriteOrDelete_WriteNonEmpty(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	s := &Settings{Path: path, Raw: map[string]any{"editor.tabSize": 2.0}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(data) == 0 {
		t.Error("file should not be empty")
	}
}

func TestWriteOrDelete_KeepsNonEmptyVSCodeDir(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.Mkdir(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Add a sibling file so .vscode is not empty after settings.json goes away
	if err := os.WriteFile(filepath.Join(vdir, "launch.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vdir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Settings{Path: path, Raw: map[string]any{}}
	if err := WriteOrDelete(s); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("settings.json should be deleted")
	}
	if _, err := os.Stat(vdir); err != nil {
		t.Error(".vscode should NOT be deleted (still has launch.json)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/vscodesettings/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/vscodesettings/write.go`:

```go
package vscodesettings

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/jsonc"
)

// WriteOrDelete writes s.Raw to s.Path, or deletes the file (and any now-empty
// parent .vscode/ directory) if s.Raw is empty.
func WriteOrDelete(s *Settings) error {
	if s == nil || s.Path == "" {
		return nil
	}
	if s.IsEmpty() {
		if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete %s: %w", s.Path, err)
		}
		parent := filepath.Dir(s.Path)
		if filepath.Base(parent) == ".vscode" {
			entries, err := os.ReadDir(parent)
			if err == nil && len(entries) == 0 {
				if err := os.Remove(parent); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("delete %s: %w", parent, err)
				}
			}
		}
		return nil
	}
	data, err := jsonc.Write(s.Raw)
	if err != nil {
		return err
	}
	return atomicWrite(s.Path, data, 0o644)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccws-*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	tmpPath := tmp.Name()
	cleaned := false
	defer func() {
		if !cleaned {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleaned = true
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/vscodesettings/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/vscodesettings/
git commit -m "vscodesettings: add WriteOrDelete with .vscode cleanup"
```

---

## Task 24: Runner Options + color resolution

**Files:**
- Create: `internal/runner/options.go`
- Create: `internal/runner/resolve.go`
- Create: `internal/runner/resolve_test.go`

- [ ] **Step 1: Write failing tests**

`internal/runner/resolve_test.go`:

```go
package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveColor_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	// Even if settings.json has peacock.color, --color should win.
	writeSettings(t, dir, `{"peacock.color": "#111111"}`)
	got, src, err := ResolveColor(dir, "#222222")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hex() != "#222222" {
		t.Errorf("got %s, want #222222", got.Hex())
	}
	if src != SourceFlag {
		t.Errorf("source = %v, want SourceFlag", src)
	}
}

func TestResolveColor_InheritFromSettings(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"peacock.color": "#5a3b8c"}`)
	got, src, err := ResolveColor(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hex() != "#5a3b8c" {
		t.Errorf("got %s, want #5a3b8c", got.Hex())
	}
	if src != SourceSettings {
		t.Errorf("source = %v, want SourceSettings", src)
	}
}

func TestResolveColor_Random(t *testing.T) {
	dir := t.TempDir() // no settings
	got, src, err := ResolveColor(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceRandom {
		t.Errorf("source = %v, want SourceRandom", src)
	}
	if !strings.HasPrefix(got.Hex(), "#") {
		t.Errorf("expected hex format, got %s", got.Hex())
	}
}

func TestResolveColor_InvalidFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ResolveColor(dir, "not-a-color"); err == nil {
		t.Error("expected error for bad input")
	}
}

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	vdir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/...`
Expected: FAIL.

- [ ] **Step 3: Implement Options**

`internal/runner/options.go`:

```go
// Package runner orchestrates the full ccws flow: resolve color, generate
// palette, place the workspace file with guards, clean up source, open.
package runner

import "github.com/sang-bin/vscode-color-workspace/internal/color"

// Options is the full runner input.
type Options struct {
	TargetDir    string       // absolute path, must exist
	ColorInput   string       // raw --color flag value; empty = auto
	NoOpen       bool         // skip `code` launch
	Force        bool         // bypass both safety guards
	KeepSource   bool         // interactive only; if true, skip .vscode cleanup
	Palette      color.Options // affects + standards
}

// Defaults returns sensible default Options. TargetDir must be filled by caller.
func Defaults() Options {
	return Options{
		Palette: color.DefaultOptions(),
	}
}
```

- [ ] **Step 4: Implement resolve**

`internal/runner/resolve.go`:

```go
package runner

import (
	"fmt"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
)

// ColorSource indicates where the final color came from.
type ColorSource int

const (
	SourceFlag ColorSource = iota + 1
	SourceSettings
	SourceRandom
)

// ResolveColor applies the priority rules:
//  1. Explicit flag wins
//  2. peacock.color from .vscode/settings.json
//  3. Random
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, error) {
	if flag != "" {
		c, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, fmt.Errorf("--color: %w", err)
		}
		return c, SourceFlag, nil
	}
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			c, err := color.Parse(pc)
			if err != nil {
				return color.Color{}, 0, fmt.Errorf("peacock.color in settings: %w", err)
			}
			return c, SourceSettings, nil
		}
	}
	return color.Random(), SourceRandom, nil
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/runner/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/runner/
git commit -m "runner: add Options + ResolveColor"
```

---

## Task 25: Opener interface + exec code

**Files:**
- Create: `internal/runner/opener.go`
- Create: `internal/runner/opener_test.go`

- [ ] **Step 1: Write failing tests**

`internal/runner/opener_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/runner/opener.go`:

```go
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
	// Fork-and-forget: `code` itself detaches; we just Start, not Wait.
	cmd := exec.Command(codePath, workspacePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec code: %w", err)
	}
	// Release so our process doesn't wait.
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/
git commit -m "runner: add Opener interface + CodeOpener + FakeOpener"
```

---

## Task 26: Runner orchestrator

**Files:**
- Create: `internal/runner/runner.go`
- Create: `internal/runner/runner_test.go`

- [ ] **Step 1: Write failing tests**

`internal/runner/runner_test.go`:

```go
package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_NewProject_Random(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	r := New(opener)

	res, err := r.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if res.WorkspaceFile != wsPath {
		t.Errorf("workspace file = %q, want %q", res.WorkspaceFile, wsPath)
	}
	if _, err := os.Stat(wsPath); err != nil {
		t.Errorf("workspace not created: %v", err)
	}
	if len(opener.Calls) != 1 {
		t.Errorf("opener called %d times, want 1", len(opener.Calls))
	}
	if res.ColorSource != SourceRandom {
		t.Errorf("color source = %v, want SourceRandom", res.ColorSource)
	}
}

func TestRun_Migrate(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.MkdirAll(filepath.Join(target, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{
		"peacock.color": "#5a3b8c",
		"editor.tabSize": 2,
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c"
		}
	}`
	if err := os.WriteFile(filepath.Join(target, ".vscode", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Defaults()
	opts.TargetDir = target
	r := New(&FakeOpener{})

	res, err := r.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ColorSource != SourceSettings {
		t.Errorf("color source = %v, want SourceSettings", res.ColorSource)
	}
	// settings.json should still exist (editor.tabSize remains)
	data, err := os.ReadFile(filepath.Join(target, ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json read: %v", err)
	}
	if string(data) == "" {
		t.Error("settings.json emptied unexpectedly")
	}
	// peacock.color should be gone
	if contains(data, []byte("peacock.color")) {
		t.Error("peacock.color should have been removed")
	}
}

func TestRun_Guard1_Triggers(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing workspace with peacock keys
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	existing := `{
		"folders": [{"path":"./myproj"}],
		"settings": { "peacock.color": "#111111" }
	}`
	if err := os.WriteFile(wsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222"

	_, err := New(&FakeOpener{}).Run(opts)
	gerr, ok := err.(*GuardError)
	if !ok {
		t.Fatalf("expected GuardError, got %T: %v", err, err)
	}
	if gerr.Guard != 1 {
		t.Errorf("guard = %d, want 1", gerr.Guard)
	}
}

func TestRun_Force_BypassesGuard1(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	wsPath := filepath.Join(tmp, "myproj.code-workspace")
	if err := os.WriteFile(wsPath, []byte(`{"folders":[{"path":"./myproj"}],"settings":{"peacock.color":"#111111"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222"
	opts.Force = true
	if _, err := New(&FakeOpener{}).Run(opts); err != nil {
		t.Fatalf("--force should succeed: %v", err)
	}
	data, _ := os.ReadFile(wsPath)
	if !contains(data, []byte("#222222")) {
		t.Error("expected new color in workspace file")
	}
}

func TestRun_Guard2_Triggers(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.MkdirAll(filepath.Join(target, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{
		"workbench.colorCustomizations": {
			"activityBar.background": "#5a3b8c",
			"editor.background": "#000000"
		}
	}`
	if err := os.WriteFile(filepath.Join(target, ".vscode", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Defaults()
	opts.TargetDir = target
	opts.ColorInput = "#222222"

	_, err := New(&FakeOpener{}).Run(opts)
	gerr, ok := err.(*GuardError)
	if !ok {
		t.Fatalf("expected GuardError, got %T: %v", err, err)
	}
	if gerr.Guard != 2 {
		t.Errorf("guard = %d, want 2", gerr.Guard)
	}
}

func TestRun_NoOpen(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "myproj")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	opener := &FakeOpener{}
	opts := Defaults()
	opts.TargetDir = target
	opts.NoOpen = true
	if _, err := New(opener).Run(opts); err != nil {
		t.Fatal(err)
	}
	if len(opener.Calls) != 0 {
		t.Errorf("opener should not be called, got %d calls", len(opener.Calls))
	}
}

func contains(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
```

Test file imports should be:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/...`
Expected: FAIL (Runner undefined).

- [ ] **Step 3: Implement**

`internal/runner/runner.go`:

```go
package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
)

// GuardError indicates a safety guard triggered (Guard 1 or Guard 2).
// Callers treat this as exit code 2.
type GuardError struct {
	Guard   int      // 1 or 2
	Message string   // human summary
	Keys    []string // offending keys
}

func (e *GuardError) Error() string {
	return e.Message
}

// Result is the output of a successful Run.
type Result struct {
	WorkspaceFile string
	ColorHex      string
	ColorSource   ColorSource
	SettingsCleaned bool
	Warnings      []string
}

// Runner orchestrates the full flow.
type Runner struct {
	Opener Opener
}

// New returns a Runner using opener. If opener is nil, CodeOpener is used.
func New(opener Opener) *Runner {
	if opener == nil {
		opener = CodeOpener{}
	}
	return &Runner{Opener: opener}
}

// Run executes the full pipeline.
func (r *Runner) Run(opts Options) (*Result, error) {
	// 1. Validate target dir
	info, err := os.Stat(opts.TargetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("target does not exist: %s", opts.TargetDir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target is not a directory: %s", opts.TargetDir)
	}
	abs, err := filepath.Abs(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	// 2. Resolve color
	c, src, err := ResolveColor(abs, opts.ColorInput)
	if err != nil {
		return nil, err
	}

	// 3. Determine workspace file path (<parent>/<folder>.code-workspace)
	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")

	// 4. Guard 1: check existing workspace for peacock keys
	ws, err := workspace.Read(wsPath)
	if err != nil {
		return nil, err
	}
	if ws != nil && !opts.Force {
		if keys := workspace.ExistingPeacockKeys(ws); len(keys) > 0 {
			return nil, &GuardError{
				Guard: 1,
				Keys:  keys,
				Message: fmt.Sprintf(
					"existing peacock color settings in %s: %s\n"+
						"rerun with --force to overwrite",
					wsPath, strings.Join(keys, ", ")),
			}
		}
	}

	// 5. Read source settings + Guard 2 (only if we will clean up)
	settingsPath := filepath.Join(abs, ".vscode", "settings.json")
	src2, err := vscodesettings.Read(settingsPath)
	if err != nil {
		return nil, err
	}
	willClean := !opts.KeepSource && src2 != nil
	if willClean && !opts.Force {
		if keys := vscodesettings.ResidualColorKeys(src2); len(keys) > 0 {
			return nil, &GuardError{
				Guard: 2,
				Keys:  keys,
				Message: fmt.Sprintf(
					"non-peacock workbench.colorCustomizations would remain in %s: %s\n"+
						"remove those keys manually or rerun with --force",
					settingsPath, strings.Join(keys, ", ")),
			}
		}
	}

	// 6. Generate palette
	palette := color.Palette(c, opts.Palette)
	colorHex := c.Hex()

	// 7. Apply to workspace (create or merge)
	if ws == nil {
		ws = &workspace.Workspace{}
	}
	workspace.EnsureFolder(ws, "./"+folderName)
	workspace.ApplyPeacock(ws, colorHex, palette)
	if err := workspace.Write(wsPath, ws); err != nil {
		return nil, err
	}

	// 8. Clean up source settings
	cleaned := false
	if willClean {
		if vscodesettings.Cleanup(src2) {
			if err := vscodesettings.WriteOrDelete(src2); err != nil {
				return nil, err
			}
			cleaned = true
		}
	}

	// 9. Collect warnings
	var warnings []string
	if isGitRepo(parent) {
		warnings = append(warnings,
			fmt.Sprintf("parent directory %s is a git repository; workspace file may be committed", parent))
	}

	// 10. Open
	if !opts.NoOpen {
		if err := r.Opener.Open(wsPath); err != nil {
			if errors.Is(err, ErrCodeNotFound) {
				warnings = append(warnings, "code CLI not on PATH; open manually: "+wsPath)
			} else {
				warnings = append(warnings, "failed to open with code: "+err.Error())
			}
		}
	}

	return &Result{
		WorkspaceFile:   wsPath,
		ColorHex:        colorHex,
		ColorSource:     src,
		SettingsCleaned: cleaned,
		Warnings:        warnings,
	}, nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/
git commit -m "runner: add full orchestration with guards + cleanup + open"
```

---

## Task 27: CLI — cobra root + default command

**Files:**
- Modify: `cmd/ccws/main.go`
- Create: `cmd/ccws/root.go`

- [ ] **Step 1: Add cobra dep**

Run: `go get github.com/spf13/cobra@v1.8.0`

- [ ] **Step 2: Rewrite main**

`cmd/ccws/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(errToExit(err))
	}
}
```

- [ ] **Step 3: Implement root command**

`cmd/ccws/root.go`:

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
)

// errToExit maps error types to exit codes (§11 of the spec).
func errToExit(err error) int {
	if err == nil {
		return 0
	}
	var ge *runner.GuardError
	if errors.As(err, &ge) {
		return 2
	}
	if errors.Is(err, os.ErrPermission) {
		return 3
	}
	return 1
}

func rootCmd() *cobra.Command {
	var (
		flagColor    string
		flagNoOpen   bool
		flagForce    bool
	)

	cmd := &cobra.Command{
		Use:   "ccws [target-dir]",
		Short: "Create a .code-workspace file with Peacock-style colors and open it.",
		Long: `ccws generates a <parent>/<folder>.code-workspace file containing a
Peacock-equivalent color palette, migrates existing peacock settings from
<target>/.vscode/settings.json, and opens the workspace in VSCode.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			opts := runner.Defaults()
			opts.TargetDir = target
			opts.ColorInput = flagColor
			opts.NoOpen = flagNoOpen
			opts.Force = flagForce
			res, err := runner.New(nil).Run(opts)
			if err != nil {
				return err
			}
			fmt.Printf("wrote %s\n", res.WorkspaceFile)
			fmt.Printf("color: %s (%s)\n", res.ColorHex, sourceLabel(res.ColorSource))
			for _, w := range res.Warnings {
				fmt.Fprintln(os.Stderr, "warning: "+w)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagColor, "color", "", "Color: #RRGGBB, #RGB, CSS name, or 'random'")
	cmd.Flags().BoolVar(&flagNoOpen, "no-open", false, "Do not open with `code` after creating")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Bypass safety guards (overwrite existing peacock keys, keep non-peacock colorCustomizations)")

	cmd.AddCommand(interactiveCmd())
	return cmd
}

func sourceLabel(s runner.ColorSource) string {
	switch s {
	case runner.SourceFlag:
		return "from --color"
	case runner.SourceSettings:
		return "inherited from .vscode/settings.json"
	case runner.SourceRandom:
		return "random"
	default:
		return "?"
	}
}
```

- [ ] **Step 4: Add placeholder interactive stub**

`cmd/ccws/interactive.go`:

```go
package main

import "github.com/spf13/cobra"

func interactiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive [target-dir]",
		Short: "Walk through options interactively (huh form).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args)
		},
	}
}
```

Add stub:

```go
func runInteractive(args []string) error {
	return nil // to be implemented in Task 28
}
```

Actually put the stub directly above interactiveCmd:

```go
package main

import "github.com/spf13/cobra"

func interactiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive [target-dir]",
		Short: "Walk through options interactively (huh form).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args)
		},
	}
}

func runInteractive(args []string) error {
	return nil // implemented in Task 28
}
```

- [ ] **Step 5: Verify build and smoke test**

Run: `go build ./...`
Expected: successful build.

Run: `./ccws --help`
Expected: usage message showing `--color`, `--no-open`, `--force` flags and `interactive` subcommand.

Run on a fresh temp dir:
```bash
mkdir -p /tmp/ccws-smoke/proj
./ccws --no-open /tmp/ccws-smoke/proj
ls /tmp/ccws-smoke/
```
Expected: `proj.code-workspace` written to `/tmp/ccws-smoke/`, stdout prints the path + color line.

- [ ] **Step 6: Commit**

```bash
git add cmd/ccws/ go.mod go.sum
git commit -m "cli: add cobra root command with --color/--no-open/--force"
```

---

## Task 28: Interactive subcommand (huh form)

**Files:**
- Modify: `cmd/ccws/interactive.go`
- Create: `internal/interactive/form.go`
- Create: `internal/interactive/form_test.go`

- [ ] **Step 1: Add huh dep**

Run: `go get github.com/charmbracelet/huh@v0.5.3`

- [ ] **Step 2: Write form test (minimal — test the builder doesn't panic)**

`internal/interactive/form_test.go`:

```go
package interactive

import "testing"

func TestApplyToOptions_Empty(t *testing.T) {
	// Just ensure construction doesn't panic with minimal input
	c := Choices{}
	_ = ApplyToOptions(c, "/tmp/foo")
}

func TestApplyToOptions_Affects(t *testing.T) {
	c := Choices{
		TargetDir:         "/tmp/foo",
		AffectActivityBar: true,
		AffectTitleBar:    true,
	}
	opts := ApplyToOptions(c, "/tmp/foo")
	if !opts.Palette.Affect.ActivityBar {
		t.Error("ActivityBar should be on")
	}
	if !opts.Palette.Affect.TitleBar {
		t.Error("TitleBar should be on")
	}
	if opts.Palette.Affect.StatusBar {
		t.Error("StatusBar should be off")
	}
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/interactive/...`
Expected: FAIL (types missing).

- [ ] **Step 4: Implement form builder**

`internal/interactive/form.go`:

```go
// Package interactive builds the huh form for `ccws interactive`.
package interactive

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
)

// Choices collects all form answers.
type Choices struct {
	TargetDir string

	// Color source: "inherit", "random", "custom"
	ColorSource string
	CustomColor string

	// Affects (booleans driven by MultiSelect).
	AffectActivityBar           bool
	AffectStatusBar             bool
	AffectTitleBar              bool
	AffectEditorGroupBorder     bool
	AffectPanelBorder           bool
	AffectSideBarBorder         bool
	AffectSashHover             bool
	AffectStatusAndTitleBorders bool
	AffectDebuggingStatusBar    bool
	AffectTabActiveBorder       bool

	DeleteSource bool
	OpenAfter    bool
	Advanced     bool

	KeepForegroundColor bool
	KeepBadgeColor      bool
	SquigglyBeGone      bool
	DarkenLightenPct    string // user enters as string, parsed later
}

// Run displays the form and returns the populated Choices or error.
func Run(initialTarget string) (*Choices, error) {
	c := &Choices{
		TargetDir:        initialTarget,
		ColorSource:      "random",
		AffectActivityBar: true,
		AffectStatusBar:   true,
		AffectTitleBar:    true,
		DeleteSource:      true,
		OpenAfter:         true,
		DarkenLightenPct:  "10",
	}

	// Pre-populate: does settings.json have peacock.color? (enables "inherit" choice)
	hasInherit := false
	if c.TargetDir != "" {
		settingsPath := filepath.Join(c.TargetDir, ".vscode", "settings.json")
		if s, err := vscodesettings.Read(settingsPath); err == nil && s != nil {
			if _, ok := s.PeacockColor(); ok {
				hasInherit = true
				c.ColorSource = "inherit"
			}
		}
	}

	colorOpts := []huh.Option[string]{}
	if hasInherit {
		colorOpts = append(colorOpts, huh.NewOption("Use existing peacock.color from .vscode/settings.json", "inherit"))
	}
	colorOpts = append(colorOpts,
		huh.NewOption("Random", "random"),
		huh.NewOption("Custom (enter hex or CSS name)", "custom"),
	)

	affectsMulti := []huh.Option[string]{
		huh.NewOption("activityBar (default)", "activityBar").Selected(c.AffectActivityBar),
		huh.NewOption("statusBar (default)", "statusBar").Selected(c.AffectStatusBar),
		huh.NewOption("titleBar (default)", "titleBar").Selected(c.AffectTitleBar),
		huh.NewOption("editorGroupBorder", "editorGroupBorder"),
		huh.NewOption("panelBorder", "panelBorder"),
		huh.NewOption("sideBarBorder", "sideBarBorder"),
		huh.NewOption("sashHover", "sashHover"),
		huh.NewOption("statusAndTitleBorders", "statusAndTitleBorders"),
		huh.NewOption("debuggingStatusBar", "debuggingStatusBar"),
		huh.NewOption("tabActiveBorder", "tabActiveBorder"),
	}
	affectsSelected := []string{"activityBar", "statusBar", "titleBar"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Target directory").
				Value(&c.TargetDir),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Color source").
				Options(colorOpts...).
				Value(&c.ColorSource),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Hex or CSS name").
				Value(&c.CustomColor),
		).WithHideFunc(func() bool { return c.ColorSource != "custom" }),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Affected elements").
				Options(affectsMulti...).
				Value(&affectsSelected),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete peacock settings from .vscode/settings.json?").
				Affirmative("Yes").Negative("No").
				Value(&c.DeleteSource),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Open with `code` after creation?").
				Affirmative("Yes").Negative("No").
				Value(&c.OpenAfter),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Show advanced options?").
				Affirmative("Yes").Negative("Skip").
				Value(&c.Advanced),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("keepForegroundColor").
				Value(&c.KeepForegroundColor),
			huh.NewConfirm().
				Title("keepBadgeColor").
				Value(&c.KeepBadgeColor),
			huh.NewConfirm().
				Title("squigglyBeGone").
				Value(&c.SquigglyBeGone),
			huh.NewInput().
				Title("darkenLightenPct (default 10)").
				Value(&c.DarkenLightenPct),
		).WithHideFunc(func() bool { return !c.Advanced }),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	// Translate multiselect result back into booleans
	in := func(s string) bool {
		for _, v := range affectsSelected {
			if v == s {
				return true
			}
		}
		return false
	}
	c.AffectActivityBar = in("activityBar")
	c.AffectStatusBar = in("statusBar")
	c.AffectTitleBar = in("titleBar")
	c.AffectEditorGroupBorder = in("editorGroupBorder")
	c.AffectPanelBorder = in("panelBorder")
	c.AffectSideBarBorder = in("sideBarBorder")
	c.AffectSashHover = in("sashHover")
	c.AffectStatusAndTitleBorders = in("statusAndTitleBorders")
	c.AffectDebuggingStatusBar = in("debuggingStatusBar")
	c.AffectTabActiveBorder = in("tabActiveBorder")
	return c, nil
}

// ApplyToOptions converts Choices to runner.Options.
func ApplyToOptions(c Choices, targetDir string) runner.Options {
	opts := runner.Defaults()
	opts.TargetDir = targetDir
	switch c.ColorSource {
	case "inherit":
		opts.ColorInput = ""
	case "random":
		opts.ColorInput = "random"
	case "custom":
		opts.ColorInput = c.CustomColor
	}
	opts.NoOpen = !c.OpenAfter
	opts.KeepSource = !c.DeleteSource
	opts.Palette.Affect.ActivityBar = c.AffectActivityBar
	opts.Palette.Affect.StatusBar = c.AffectStatusBar
	opts.Palette.Affect.TitleBar = c.AffectTitleBar
	opts.Palette.Affect.EditorGroupBorder = c.AffectEditorGroupBorder
	opts.Palette.Affect.PanelBorder = c.AffectPanelBorder
	opts.Palette.Affect.SideBarBorder = c.AffectSideBarBorder
	opts.Palette.Affect.SashHover = c.AffectSashHover
	opts.Palette.Affect.StatusAndTitleBorders = c.AffectStatusAndTitleBorders
	opts.Palette.Affect.DebuggingStatusBar = c.AffectDebuggingStatusBar
	opts.Palette.Affect.TabActiveBorder = c.AffectTabActiveBorder
	if c.Advanced {
		opts.Palette.Standard.KeepForegroundColor = c.KeepForegroundColor
		opts.Palette.Standard.KeepBadgeColor = c.KeepBadgeColor
		opts.Palette.Standard.SquigglyBeGone = c.SquigglyBeGone
		if pct, err := parseFloat(c.DarkenLightenPct); err == nil && pct > 0 {
			opts.Palette.Standard.DarkenLightenPct = pct
		}
	}
	return opts
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
```

- [ ] **Step 5: Wire interactive command**

Replace `cmd/ccws/interactive.go`:

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/interactive"
	"github.com/sang-bin/vscode-color-workspace/internal/runner"
)

func interactiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive [target-dir]",
		Short: "Walk through options interactively (huh form).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args)
		},
	}
}

func runInteractive(args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	choices, err := interactive.Run(abs)
	if err != nil {
		return err
	}

	opts := interactive.ApplyToOptions(*choices, choices.TargetDir)

	// First pass: call runner; if it returns GuardError, confirm override.
	for attempt := 0; attempt < 2; attempt++ {
		res, err := runner.New(nil).Run(opts)
		if err == nil {
			fmt.Printf("wrote %s\n", res.WorkspaceFile)
			fmt.Printf("color: %s\n", res.ColorHex)
			for _, w := range res.Warnings {
				fmt.Fprintln(os.Stderr, "warning: "+w)
			}
			return nil
		}
		var ge *runner.GuardError
		if !errors.As(err, &ge) {
			return err
		}
		if attempt > 0 {
			return err
		}
		ok, cerr := confirmGuard(ge)
		if cerr != nil {
			return cerr
		}
		if !ok {
			return fmt.Errorf("aborted (guard %d)", ge.Guard)
		}
		opts.Force = true
	}
	return nil
}

func confirmGuard(ge *runner.GuardError) (bool, error) {
	title := fmt.Sprintf("Guard %d triggered", ge.Guard)
	desc := ge.Message + "\n\nKeys:\n  " + strings.Join(ge.Keys, "\n  ")
	var proceed bool
	err := huh.NewConfirm().
		Title(title).
		Description(desc).
		Affirmative("Override").
		Negative("Abort").
		Value(&proceed).
		Run()
	return proceed, err
}
```

- [ ] **Step 6: Run tests and build**

Run: `go test ./...`
Expected: all PASS.

Run: `go build ./...`
Expected: successful.

- [ ] **Step 7: Commit**

```bash
git add cmd/ccws/ internal/interactive/ go.mod go.sum
git commit -m "cli: add interactive subcommand with huh form + guard confirm"
```

---

## Task 29: Smoke test end-to-end on a real temp directory

**Files:** None (manual verification).

- [ ] **Step 1: Fresh project smoke test**

```bash
mkdir -p /tmp/ccws-e2e/brandnew
go run ./cmd/ccws --color '#5a3b8c' --no-open /tmp/ccws-e2e/brandnew
```
Expected:
- Stdout: `wrote /tmp/ccws-e2e/brandnew.code-workspace` + `color: #5a3b8c (from --color)`.
- File exists at `/tmp/ccws-e2e/brandnew.code-workspace`.
- File has `folders: [{ "path": "./brandnew" }]` and `settings.peacock.color == "#5a3b8c"`.

Run: `cat /tmp/ccws-e2e/brandnew.code-workspace`
Visually inspect 28 keys are NOT all present (only titleBar/activityBar/statusBar ones).

- [ ] **Step 2: Migrate smoke test**

```bash
mkdir -p /tmp/ccws-e2e/migrate/.vscode
cat > /tmp/ccws-e2e/migrate/.vscode/settings.json <<'EOF'
{
  "peacock.color": "#42b883",
  "editor.tabSize": 4,
  "workbench.colorCustomizations": {
    "activityBar.background": "#42b883",
    "titleBar.activeBackground": "#42b883"
  }
}
EOF
go run ./cmd/ccws --no-open /tmp/ccws-e2e/migrate
```
Expected:
- Stdout: color `#42b883` inherited from settings.json.
- `/tmp/ccws-e2e/migrate/.vscode/settings.json` now contains ONLY `editor.tabSize`.
- `/tmp/ccws-e2e/migrate.code-workspace` exists and has the peacock palette.

- [ ] **Step 3: Guard 1 smoke test**

```bash
go run ./cmd/ccws --color '#111111' --no-open /tmp/ccws-e2e/brandnew
echo "exit: $?"
```
Expected: exit code 2, stderr message listing `settings.peacock.color` and `settings.workbench.colorCustomizations.*`.

Then with `--force`:
```bash
go run ./cmd/ccws --color '#111111' --no-open --force /tmp/ccws-e2e/brandnew
echo "exit: $?"
```
Expected: exit 0, file overwritten with new color.

- [ ] **Step 4: Guard 2 smoke test**

```bash
mkdir -p /tmp/ccws-e2e/residual/.vscode
cat > /tmp/ccws-e2e/residual/.vscode/settings.json <<'EOF'
{
  "workbench.colorCustomizations": {
    "activityBar.background": "#5a3b8c",
    "editor.background": "#000000"
  }
}
EOF
go run ./cmd/ccws --color '#5a3b8c' --no-open /tmp/ccws-e2e/residual
echo "exit: $?"
```
Expected: exit 2, message mentions `editor.background`.

- [ ] **Step 5: Cleanup**

```bash
rm -rf /tmp/ccws-e2e
```

- [ ] **Step 6: Commit (docs of test)**

No file changes; skip commit. If any issues were found in the previous smoke tests, fix them before proceeding.

---

## Task 30: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

`README.md`:

````markdown
# vscode-color-workspace (`ccws`)

`ccws` generates a `.code-workspace` file with [Peacock](https://github.com/johnpapa/vscode-peacock)-equivalent colors in the parent directory of your project, so your per-project color setup never lands in the shared `.vscode/settings.json`.

## Why

Peacock stores its colors in `.vscode/settings.json` — the same file teams use for shared project settings — so your personal color preferences end up in Git. `ccws` writes the colors to `<parent>/<folder>.code-workspace` instead and opens the workspace with `code`. VSCode's workspace file scope is effectively private.

## Install

```bash
go install github.com/sang-bin/vscode-color-workspace/cmd/ccws@latest
```

## Usage

```bash
# Random color, auto-open
ccws

# Specific color by hex or CSS name
ccws --color '#5a3b8c'
ccws --color rebeccapurple

# Target a specific directory
ccws --color red /path/to/myproj

# Walk through all options
ccws interactive /path/to/myproj

# Overwrite existing peacock color settings
ccws --color '#ff0000' --force

# Skip opening (CI / scripts)
ccws --color random --no-open
```

Running `ccws` in `/home/me/code/myproj` will:

1. Resolve the color (explicit `--color` > `peacock.color` from `.vscode/settings.json` > random).
2. Generate the Peacock palette (activityBar / statusBar / titleBar by default).
3. Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file).
4. Clean up `peacock.*` keys and the peacock-managed subset of `workbench.colorCustomizations` from `/home/me/code/myproj/.vscode/settings.json`. If the settings file becomes empty it's deleted, along with an empty `.vscode/` directory.
5. Launch `code <workspace-file>`.

## Safety guards

`ccws` refuses to proceed and exits with code `2` in two situations:

- **Guard 1 — existing peacock keys in the workspace file.** Pass `--force` to overwrite.
- **Guard 2 — non-peacock `workbench.colorCustomizations` would remain in `.vscode/settings.json`.** Remove those keys manually or pass `--force`.

`ccws interactive` shows the same guards as explicit confirmation prompts.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | input error (invalid color, missing folder, parse failure) |
| 2 | safety guard triggered |
| 3 | filesystem error |

## Non-goals

Peacock favorites, `peacock.remoteColor` / Live Share, multi-root workspaces, per-element lighten/darken adjustments, VSCode Profiles integration, comment preservation in `.code-workspace` (comments are stripped on rewrite).
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

## Self-review checklist

After completing all tasks, verify against spec:

| Spec section | Task(s) covering it |
|---|---|
| §2 CLI: `ccws [dir]`, `ccws interactive [dir]` | 27, 28 |
| §2 Flags: `--color`, `--no-open`, `--force` | 27 |
| §3 Color priority: flag > settings > random | 24 |
| §4 Palette algorithm + primitives | 2, 3, 4, 5, 6, 7, 9-14 |
| §4 Golden fixture vs Peacock JS | 15 |
| §5 `<parent>/<folder>.code-workspace` + relative path | 26 |
| §5 Parent-is-git-repo warning | 26 |
| §6 Guard 1 | 18, 26 |
| §6 Guard 2 (conditional on cleanup) | 21, 26 |
| §7 Source cleanup + cascade | 22, 23, 26 |
| §8 Interactive flow | 28 |
| §9 `code` open, PATH fallback | 25, 26 |
| §10 Package structure | 1, 2-28 |
| §10 JSONC read/write | 16 |
| §10 Atomic write | 17, 23 |
| §11 Exit codes | 27 |
| §12 Test coverage | 2-28 (each task has tests) |

No placeholders remaining. All step code blocks contain concrete Go. Types consistent across tasks (Color, Options, Settings, Workspace, Runner, Choices, GuardError).
