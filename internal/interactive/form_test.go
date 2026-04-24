package interactive

import (
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
)

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

func TestApplyToOptions_Adjustments(t *testing.T) {
	c := Choices{
		TargetDir:         "/tmp/foo",
		AdjustActivityBar: "lighten",
		AdjustTitleBar:    "darken",
		AdjustStatusBar:   "none",
	}
	opts := ApplyToOptions(c, "/tmp/foo")
	if opts.Palette.Adjust.ActivityBar != color.AdjustLighten {
		t.Errorf("ActivityBar = %v, want AdjustLighten", opts.Palette.Adjust.ActivityBar)
	}
	if opts.Palette.Adjust.TitleBar != color.AdjustDarken {
		t.Errorf("TitleBar = %v, want AdjustDarken", opts.Palette.Adjust.TitleBar)
	}
	if opts.Palette.Adjust.StatusBar != color.AdjustNone {
		t.Errorf("StatusBar = %v, want AdjustNone", opts.Palette.Adjust.StatusBar)
	}
}
