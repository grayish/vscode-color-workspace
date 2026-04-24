package color

import "testing"

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if !opts.Affect.ActivityBar || !opts.Affect.StatusBar || !opts.Affect.TitleBar {
		t.Error("default affect should enable activity/status/title bars")
	}
	if opts.Affect.EditorGroupBorder || opts.Affect.TabActiveBorder {
		t.Error("default affect should disable border/tab options")
	}
	if opts.Standard.DarkenLightenPct != 10 {
		t.Errorf("default pct = %f, want 10", opts.Standard.DarkenLightenPct)
	}
}

func TestElementStyle_Derivatives(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	style := elementStyle(base, opts)
	if style.Background != base { t.Errorf("background mismatch") }
	if style.Foreground == style.Background {
		t.Errorf("foreground should differ from background")
	}
}
