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

func TestColor_HSLRoundtrip(t *testing.T) {
	tests := []Color{
		{255, 0, 0}, {0, 255, 0}, {0, 0, 255},
		{90, 59, 140}, {255, 255, 255}, {0, 0, 0},
	}
	for _, c := range tests {
		h, s, l := c.ToHSL()
		back := FromHSL(h, s, l)
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
	h, _, _ := (Color{255, 0, 0}).ToHSL()
	if h < -0.5 || h > 0.5 {
		t.Errorf("red hue = %f, want ~0", h)
	}
	h, _, _ = (Color{0, 255, 0}).ToHSL()
	if h < 119.5 || h > 120.5 {
		t.Errorf("green hue = %f, want ~120", h)
	}
}

// Achromatic colors have no defined hue, and the HSL conversion is delegated
// to go-colorful — the round trip must survive regardless of what hue the
// library reports for them.
func TestColor_HSLRoundtrip_Achromatic(t *testing.T) {
	for _, c := range []Color{{0, 0, 0}, {1, 1, 1}, {128, 128, 128}, {254, 254, 254}, {255, 255, 255}} {
		h, s, l := c.ToHSL()
		back := FromHSL(h, s, l)
		if back != c {
			t.Errorf("Roundtrip(%v) -> %v (h=%f s=%f l=%f)", c, back, h, s, l)
		}
	}
}

// FromHSL documents that out-of-range input is wrapped (hue) or clamped
// (saturation, lightness) rather than producing a garbage color.
func TestFromHSL_WrapsAndClamps(t *testing.T) {
	tests := []struct {
		name                string
		h, s, l             float64
		wantH, wantS, wantL float64
	}{
		{"hue above range wraps", 420, 1, 0.5, 60, 1, 0.5},
		{"hue far above range wraps", 1140, 1, 0.5, 60, 1, 0.5},
		{"negative hue wraps", -60, 1, 0.5, 300, 1, 0.5},
		{"hue at 360 wraps to 0", 360, 1, 0.5, 0, 1, 0.5},
		{"saturation above 1 clamps", 120, 1.5, 0.5, 120, 1, 0.5},
		{"negative saturation clamps", 120, -0.5, 0.5, 120, 0, 0.5},
		{"lightness above 1 clamps", 120, 1, 1.5, 120, 1, 1},
		{"negative lightness clamps", 120, 1, -0.5, 120, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := FromHSL(tt.h, tt.s, tt.l), FromHSL(tt.wantH, tt.wantS, tt.wantL); got != want {
				t.Errorf("FromHSL(%v, %v, %v) = %v, want %v", tt.h, tt.s, tt.l, got, want)
			}
		})
	}
}

// Clamping lightness to the extremes yields pure black and white whatever the
// hue, which is what ApplyLightness relies on at the ends of the ladder.
func TestFromHSL_ExtremeLightness(t *testing.T) {
	if got := FromHSL(200, 0.8, 0); got != (Color{0, 0, 0}) {
		t.Errorf("FromHSL(l=0) = %v, want black", got)
	}
	if got := FromHSL(200, 0.8, 1); got != (Color{255, 255, 255}) {
		t.Errorf("FromHSL(l=1) = %v, want white", got)
	}
}
