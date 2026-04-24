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

func TestCollectTitleBar_Defaults(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	out := collectTitleBar(base, opts)
	must := []string{
		"titleBar.activeBackground", "titleBar.inactiveBackground",
		"titleBar.activeForeground", "titleBar.inactiveForeground",
		"commandCenter.border",
	}
	for _, k := range must {
		if _, ok := out[k]; !ok { t.Errorf("missing key %q", k) }
	}
	if out["titleBar.activeBackground"] != "#5a3b8c" {
		t.Errorf("activeBackground = %q, want #5a3b8c", out["titleBar.activeBackground"])
	}
	if _, ok := out["titleBar.border"]; ok {
		t.Error("titleBar.border should not be set without StatusAndTitleBorders")
	}
}

func TestCollectTitleBar_Disabled(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Affect.TitleBar = false
	out := collectTitleBar(base, opts)
	if len(out) != 0 { t.Errorf("disabled titleBar returned %d keys, want 0", len(out)) }
}

func TestCollectTitleBar_KeepForeground(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Standard.KeepForegroundColor = true
	out := collectTitleBar(base, opts)
	if _, ok := out["titleBar.activeForeground"]; ok {
		t.Error("activeForeground should be omitted with KeepForegroundColor")
	}
	if _, ok := out["commandCenter.border"]; ok {
		t.Error("commandCenter.border should be omitted with KeepForegroundColor")
	}
	if _, ok := out["titleBar.activeBackground"]; !ok {
		t.Error("activeBackground still expected")
	}
}

func TestCollectTitleBar_WithBorders(t *testing.T) {
	base := Color{90, 59, 140}
	opts := DefaultOptions()
	opts.Affect.StatusAndTitleBorders = true
	out := collectTitleBar(base, opts)
	if out["titleBar.border"] != "#5a3b8c" {
		t.Errorf("titleBar.border = %q, want #5a3b8c", out["titleBar.border"])
	}
}
