package interactive

import "testing"

func TestApplyToOptions_Empty(t *testing.T) {
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
