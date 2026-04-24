package color

import ( "math"; "testing" )

func TestContrast_BlackWhite(t *testing.T) {
	c := Contrast(Color{0, 0, 0}, Color{255, 255, 255})
	if math.Abs(c-21) > 0.01 { t.Errorf("contrast(black, white) = %f, want ~21", c) }
}

func TestContrast_Symmetric(t *testing.T) {
	a, b := Color{255, 0, 0}, Color{0, 0, 255}
	if Contrast(a, b) != Contrast(b, a) { t.Error("contrast should be symmetric") }
}

func TestContrast_Same(t *testing.T) {
	if got := Contrast(Color{123, 45, 67}, Color{123, 45, 67}); math.Abs(got-1) > 0.01 {
		t.Errorf("contrast(same) = %f, want 1", got)
	}
}

func TestRelativeLuminance(t *testing.T) {
	if got := (Color{0, 0, 0}).RelativeLuminance(); got != 0 {
		t.Errorf("lum(black) = %f, want 0", got)
	}
	if got := (Color{255, 255, 255}).RelativeLuminance(); math.Abs(got-1) > 0.0001 {
		t.Errorf("lum(white) = %f, want 1", got)
	}
}
