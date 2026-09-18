package interactive

import (
	"testing"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
)

// A zero Choices means "no affects selected, don't open, keep the source
// settings" — every field is driven by the form, so none may silently inherit
// runner defaults.
func TestApplyToOptions_Empty(t *testing.T) {
	opts := ApplyToOptions(Choices{}, "/tmp/foo")
	if opts.TargetDir != "/tmp/foo" {
		t.Errorf("TargetDir = %q, want /tmp/foo", opts.TargetDir)
	}
	if opts.Palette.Affect.ActivityBar || opts.Palette.Affect.StatusBar || opts.Palette.Affect.TitleBar {
		t.Errorf("no affects should be set, got %+v", opts.Palette.Affect)
	}
	if !opts.NoOpen {
		t.Error("OpenAfter=false should set NoOpen")
	}
	if !opts.KeepSource {
		t.Error("DeleteSource=false should set KeepSource")
	}
}

func TestApplyToOptions_ColorSource(t *testing.T) {
	tests := []struct {
		source string
		custom string
		want   string
	}{
		{"random", "", "random"},
		{"custom", "#5a3b8c", "#5a3b8c"},
		// "inherit" clears ColorInput so the runner falls back to
		// .vscode/settings.json rather than generating a new color.
		{"inherit", "#5a3b8c", ""},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			opts := ApplyToOptions(Choices{ColorSource: tt.source, CustomColor: tt.custom}, "/tmp/foo")
			if opts.ColorInput != tt.want {
				t.Errorf("ColorInput = %q, want %q", opts.ColorInput, tt.want)
			}
		})
	}
}

// The advanced group is hidden unless Advanced is set, so its answers must not
// reach Options otherwise — including a percentage the user typed and then
// backed out of.
func TestApplyToOptions_AdvancedGating(t *testing.T) {
	def := ApplyToOptions(Choices{}, "/tmp/foo").Palette.Standard.DarkenLightenPct

	notAdvanced := ApplyToOptions(Choices{
		Advanced:            false,
		SquigglyBeGone:      true,
		KeepForegroundColor: true,
		DarkenLightenPct:    "25",
	}, "/tmp/foo")
	if notAdvanced.Palette.Standard.SquigglyBeGone || notAdvanced.Palette.Standard.KeepForegroundColor {
		t.Error("advanced answers must be ignored when Advanced=false")
	}
	if notAdvanced.Palette.Standard.DarkenLightenPct != def {
		t.Errorf("DarkenLightenPct = %v, want default %v", notAdvanced.Palette.Standard.DarkenLightenPct, def)
	}

	advanced := ApplyToOptions(Choices{
		Advanced:            true,
		SquigglyBeGone:      true,
		KeepForegroundColor: true,
		KeepBadgeColor:      true,
		DarkenLightenPct:    "25",
	}, "/tmp/foo")
	if !advanced.Palette.Standard.SquigglyBeGone || !advanced.Palette.Standard.KeepForegroundColor || !advanced.Palette.Standard.KeepBadgeColor {
		t.Errorf("advanced answers should apply, got %+v", advanced.Palette.Standard)
	}
	if advanced.Palette.Standard.DarkenLightenPct != 25 {
		t.Errorf("DarkenLightenPct = %v, want 25", advanced.Palette.Standard.DarkenLightenPct)
	}
}

// An unparseable or non-positive percentage leaves the default in place rather
// than driving the palette with 0.
func TestApplyToOptions_RejectsBadPct(t *testing.T) {
	def := ApplyToOptions(Choices{}, "/tmp/foo").Palette.Standard.DarkenLightenPct
	for _, pct := range []string{"", "abc", "0", "-5"} {
		t.Run("pct="+pct, func(t *testing.T) {
			opts := ApplyToOptions(Choices{Advanced: true, DarkenLightenPct: pct}, "/tmp/foo")
			if opts.Palette.Standard.DarkenLightenPct != def {
				t.Errorf("DarkenLightenPct = %v, want default %v", opts.Palette.Standard.DarkenLightenPct, def)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"10", 10, false},
		{"12.5", 12.5, false},
		{"-5", -5, false},
		{"", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseFloat(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFloat(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFloat(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseAdjChoice(t *testing.T) {
	tests := []struct {
		in   string
		want color.Adjustment
	}{
		{"lighten", color.AdjustLighten},
		{"darken", color.AdjustDarken},
		{"none", color.AdjustNone},
		{"", color.AdjustNone},
		{"bogus", color.AdjustNone},
	}
	for _, tt := range tests {
		if got := parseAdjChoice(tt.in); got != tt.want {
			t.Errorf("parseAdjChoice(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
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
