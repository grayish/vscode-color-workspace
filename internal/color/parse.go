package color

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Parse resolves an input string to a Color.
// Accepted: "#rrggbb"/"rrggbb", "#rgb"/"rgb", CSS named, "random".
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
	default:
		return Color{}, fmt.Errorf("color: invalid hex %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("color: invalid hex %q: %w", s, err)
	}
	return Color{R: uint8((v >> 16) & 0xff), G: uint8((v >> 8) & 0xff), B: uint8(v & 0xff)}, nil
}

// Random returns a color with R/G/B each uniformly random in [0, 256).
func Random() Color {
	return Color{
		R: uint8(rand.Float64() * 256),
		G: uint8(rand.Float64() * 256),
		B: uint8(rand.Float64() * 256),
	}
}
