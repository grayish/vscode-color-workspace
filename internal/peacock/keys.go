// Package peacock exposes Peacock-specific setting names as Go constants.
// Matches the ColorSettings and StandardSettings enums from
// /Users/user/Projects/vscode-peacock/src/models/enums.ts.
package peacock

import "strings"

// colorKeys is the full set of VSCode workbench.colorCustomizations keys
// that Peacock manages.
var colorKeys = []string{
	"activityBar.activeBackground",
	"activityBar.background",
	"activityBar.foreground",
	"activityBar.inactiveForeground",
	"activityBarBadge.background",
	"activityBarBadge.foreground",
	"commandCenter.border",
	"editorGroup.border",
	"panel.border",
	"sideBar.border",
	"sash.hoverBorder",
	"editorError.foreground",
	"editorWarning.foreground",
	"editorInfo.foreground",
	"statusBar.border",
	"statusBar.background",
	"statusBar.foreground",
	"statusBar.debuggingBorder",
	"statusBar.debuggingBackground",
	"statusBar.debuggingForeground",
	"statusBarItem.hoverBackground",
	"statusBarItem.remoteBackground",
	"statusBarItem.remoteForeground",
	"tab.activeBorder",
	"titleBar.activeBackground",
	"titleBar.activeForeground",
	"titleBar.border",
	"titleBar.inactiveBackground",
	"titleBar.inactiveForeground",
}

// Peacock extension setting names (under the "peacock." namespace).
const (
	SettingColor                       = "peacock.color"
	SettingRemoteColor                 = "peacock.remoteColor"
	SettingAffectActivityBar           = "peacock.affectActivityBar"
	SettingAffectStatusBar             = "peacock.affectStatusBar"
	SettingAffectTitleBar              = "peacock.affectTitleBar"
	SettingAffectEditorGroupBorder     = "peacock.affectEditorGroupBorder"
	SettingAffectPanelBorder           = "peacock.affectPanelBorder"
	SettingAffectSideBarBorder         = "peacock.affectSideBarBorder"
	SettingAffectSashHover             = "peacock.affectSashHover"
	SettingAffectStatusAndTitleBorders = "peacock.affectStatusAndTitleBorders"
	SettingAffectDebuggingStatusBar    = "peacock.affectDebuggingStatusBar"
	SettingAffectTabActiveBorder       = "peacock.affectTabActiveBorder"
	SettingKeepForegroundColor         = "peacock.keepForegroundColor"
	SettingKeepBadgeColor              = "peacock.keepBadgeColor"
	SettingSquigglyBeGone              = "peacock.squigglyBeGone"
	SettingDarkenLightenPercentage     = "peacock.darkenLightenPercentage"
	SettingDarkForegroundColor         = "peacock.darkForegroundColor"
	SettingLightForegroundColor        = "peacock.lightForegroundColor"

	SectionColorCustomizations = "workbench.colorCustomizations"
)

// DefaultDarkForeground matches Peacock's ForegroundColors.DarkForeground.
const DefaultDarkForeground = "#15202b"

// DefaultLightForeground matches Peacock's ForegroundColors.LightForeground.
const DefaultLightForeground = "#e7e7e7"

// InactiveAlpha matches Peacock's inactiveElementAlpha (0x99).
const InactiveAlpha = 0x99

// ColorKeys returns the full list of keys Peacock manages.
func ColorKeys() []string {
	out := make([]string, len(colorKeys))
	copy(out, colorKeys)
	return out
}

// ColorKeysSet returns a set (map) for O(1) membership checks.
func ColorKeysSet() map[string]bool {
	s := make(map[string]bool, len(colorKeys))
	for _, k := range colorKeys {
		s[k] = true
	}
	return s
}

// HasPeacockPrefix reports whether a key begins with "peacock.".
func HasPeacockPrefix(key string) bool {
	return strings.HasPrefix(key, "peacock.")
}
