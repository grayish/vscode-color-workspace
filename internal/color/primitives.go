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
