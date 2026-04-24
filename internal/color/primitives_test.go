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

func abs(x int) int { if x < 0 { return -x }; return x }

func TestColor_ToHSL_Hue(t *testing.T) {
	h, _, _ := (Color{255, 0, 0}).ToHSL()
	if h < -0.5 || h > 0.5 { t.Errorf("red hue = %f, want ~0", h) }
	h, _, _ = (Color{0, 255, 0}).ToHSL()
	if h < 119.5 || h > 120.5 { t.Errorf("green hue = %f, want ~120", h) }
}
