package color

import "testing"

func TestReadableAccent_MeetsRatio(t *testing.T) {
	base := Color{90, 59, 140}
	accent := ReadableAccent(base, 2.0)
	if got := Contrast(accent, base); got < 2.0 {
		t.Errorf("accent contrast = %f, want >= 2.0", got)
	}
}

func TestReadableAccent_Grayscale(t *testing.T) {
	base := Color{128, 128, 128}
	accent := ReadableAccent(base, 2.0)
	if accent == base {
		t.Errorf("accent same as grayscale base")
	}
}

func TestReadableAccent_ExtremeBase(t *testing.T) {
	_ = ReadableAccent(Color{0, 0, 0}, 2.0)
	_ = ReadableAccent(Color{255, 255, 255}, 2.0)
}

func TestHoverColor_IsLight(t *testing.T) {
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
