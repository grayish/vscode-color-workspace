// Package color provides color primitives ported from Peacock's tinycolor2
// usage, plus the palette generator that mirrors Peacock's prepareColors.
package color

import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
)

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

// ToHSL returns hue in [0, 360), saturation and lightness in [0, 1].
func (c Color) ToHSL() (h, s, l float64) {
	cf := colorful.Color{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255}
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

// Triad returns three colors 120° apart in hue.
func (c Color) Triad() [3]Color {
	h, s, l := c.ToHSL()
	return [3]Color{c, FromHSL(h+120, s, l), FromHSL(h+240, s, l)}
}

// Complement shifts hue 180°.
func (c Color) Complement() Color {
	h, s, l := c.ToHSL()
	return FromHSL(h+180, s, l)
}

// Lighten shifts L up by pct (0-100 scale).
func (c Color) Lighten(pct float64) Color {
	h, s, l := c.ToHSL()
	return FromHSL(h, s, l+pct/100)
}

// Darken shifts L down by pct.
func (c Color) Darken(pct float64) Color {
	h, s, l := c.ToHSL()
	return FromHSL(h, s, l-pct/100)
}
