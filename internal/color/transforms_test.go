package color

import "testing"

func TestColor_Triad(t *testing.T) {
	triad := Color{255, 0, 0}.Triad()
	if triad[0] != (Color{255, 0, 0}) { t.Errorf("triad[0] = %v, want red", triad[0]) }
	h1, _, _ := triad[1].ToHSL()
	if h1 < 119.5 || h1 > 120.5 { t.Errorf("triad[1] hue = %f, want ~120", h1) }
	h2, _, _ := triad[2].ToHSL()
	if h2 < 239.5 || h2 > 240.5 { t.Errorf("triad[2] hue = %f, want ~240", h2) }
}

func TestColor_Complement(t *testing.T) {
	c := (Color{255, 0, 0}).Complement()
	h, _, _ := c.ToHSL()
	if h < 179.5 || h > 180.5 { t.Errorf("complement red hue = %f, want ~180", h) }
}

func TestColor_Lighten(t *testing.T) {
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
