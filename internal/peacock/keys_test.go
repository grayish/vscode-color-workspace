package peacock

import "testing"

func TestColorKeys_Count(t *testing.T) {
	if got := len(ColorKeys()); got != 29 {
		t.Errorf("ColorKeys() len = %d, want 29", got)
	}
}

func TestColorKeys_Contains(t *testing.T) {
	set := ColorKeysSet()
	must := []string{
		"activityBar.background", "activityBar.activeBackground",
		"activityBarBadge.background", "titleBar.activeBackground",
		"statusBar.background", "editorGroup.border", "panel.border",
		"sideBar.border", "sash.hoverBorder", "tab.activeBorder",
		"commandCenter.border", "editorError.foreground",
	}
	for _, k := range must {
		if !set[k] { t.Errorf("ColorKeysSet missing %q", k) }
	}
}

func TestHasPeacockPrefix(t *testing.T) {
	if !HasPeacockPrefix("peacock.color") { t.Error("peacock.color should match") }
	if HasPeacockPrefix("workbench.colorCustomizations") { t.Error("workbench.* should not match") }
}

func TestSettingColor(t *testing.T) {
	if SettingColor != "peacock.color" { t.Errorf("SettingColor = %q", SettingColor) }
}
