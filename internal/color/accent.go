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

// HoverOf returns the hover variant with the default 10% amount.
func HoverOf(base Color) Color {
	return hoverOfAmount(base, DefaultLightenDarkenAmount)
}

// hoverOfAmount returns the hover variant using a caller-specified percent.
func hoverOfAmount(base Color, pct float64) Color {
	if base.IsLight() {
		return base.Darken(pct)
	}
	return base.Lighten(pct)
}

// ReadableAccent returns an accent color with contrast >= ratio against base.
// Ports Peacock's getReadableAccentColorHex (color-library.ts).
func ReadableAccent(base Color, ratio float64) Color {
	triad := base.Triad()
	fg := triad[1]
	h, s, l := fg.ToHSL()
	if s == 0 {
		h = 60 * math.Round(l*6)
	}
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
	return Color{255, 255, 255}
}
