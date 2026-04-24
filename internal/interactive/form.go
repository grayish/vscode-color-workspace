// Package interactive builds the huh form for `ccws interactive`.
package interactive

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
)

// Choices collects all form answers.
type Choices struct {
	TargetDir   string
	ColorSource string // "inherit" | "random" | "custom"
	CustomColor string

	AffectActivityBar           bool
	AffectStatusBar             bool
	AffectTitleBar              bool
	AffectEditorGroupBorder     bool
	AffectPanelBorder           bool
	AffectSideBarBorder         bool
	AffectSashHover             bool
	AffectStatusAndTitleBorders bool
	AffectDebuggingStatusBar    bool
	AffectTabActiveBorder       bool

	DeleteSource bool
	OpenAfter    bool
	Advanced     bool

	KeepForegroundColor bool
	KeepBadgeColor      bool
	SquigglyBeGone      bool
	DarkenLightenPct    string
}

// Run displays the form and returns populated Choices or error.
func Run(initialTarget string) (*Choices, error) {
	c := &Choices{
		TargetDir:         initialTarget,
		ColorSource:       "random",
		AffectActivityBar: true,
		AffectStatusBar:   true,
		AffectTitleBar:    true,
		DeleteSource:      true,
		OpenAfter:         true,
		DarkenLightenPct:  "10",
	}

	hasInherit := false
	if c.TargetDir != "" {
		settingsPath := filepath.Join(c.TargetDir, ".vscode", "settings.json")
		if s, err := vscodesettings.Read(settingsPath); err == nil && s != nil {
			if _, ok := s.PeacockColor(); ok {
				hasInherit = true
				c.ColorSource = "inherit"
			}
		}
	}

	colorOpts := []huh.Option[string]{}
	if hasInherit {
		colorOpts = append(colorOpts, huh.NewOption("Use existing peacock.color from .vscode/settings.json", "inherit"))
	}
	colorOpts = append(colorOpts,
		huh.NewOption("Random", "random"),
		huh.NewOption("Custom (enter hex or CSS name)", "custom"),
	)

	affectsMulti := []huh.Option[string]{
		huh.NewOption("activityBar (default)", "activityBar").Selected(c.AffectActivityBar),
		huh.NewOption("statusBar (default)", "statusBar").Selected(c.AffectStatusBar),
		huh.NewOption("titleBar (default)", "titleBar").Selected(c.AffectTitleBar),
		huh.NewOption("editorGroupBorder", "editorGroupBorder"),
		huh.NewOption("panelBorder", "panelBorder"),
		huh.NewOption("sideBarBorder", "sideBarBorder"),
		huh.NewOption("sashHover", "sashHover"),
		huh.NewOption("statusAndTitleBorders", "statusAndTitleBorders"),
		huh.NewOption("debuggingStatusBar", "debuggingStatusBar"),
		huh.NewOption("tabActiveBorder", "tabActiveBorder"),
	}
	affectsSelected := []string{"activityBar", "statusBar", "titleBar"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Target directory").Value(&c.TargetDir),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Color source").Options(colorOpts...).Value(&c.ColorSource),
		),
		huh.NewGroup(
			huh.NewInput().Title("Hex or CSS name").Value(&c.CustomColor),
		).WithHideFunc(func() bool { return c.ColorSource != "custom" }),
		huh.NewGroup(
			huh.NewMultiSelect[string]().Title("Affected elements").Options(affectsMulti...).Value(&affectsSelected),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Delete peacock settings from .vscode/settings.json?").
				Affirmative("Yes").Negative("No").Value(&c.DeleteSource),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Open with `code` after creation?").
				Affirmative("Yes").Negative("No").Value(&c.OpenAfter),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Show advanced options?").
				Affirmative("Yes").Negative("Skip").Value(&c.Advanced),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("keepForegroundColor").Value(&c.KeepForegroundColor),
			huh.NewConfirm().Title("keepBadgeColor").Value(&c.KeepBadgeColor),
			huh.NewConfirm().Title("squigglyBeGone").Value(&c.SquigglyBeGone),
			huh.NewInput().Title("darkenLightenPct (default 10)").Value(&c.DarkenLightenPct),
		).WithHideFunc(func() bool { return !c.Advanced }),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	in := func(s string) bool {
		for _, v := range affectsSelected {
			if v == s {
				return true
			}
		}
		return false
	}
	c.AffectActivityBar = in("activityBar")
	c.AffectStatusBar = in("statusBar")
	c.AffectTitleBar = in("titleBar")
	c.AffectEditorGroupBorder = in("editorGroupBorder")
	c.AffectPanelBorder = in("panelBorder")
	c.AffectSideBarBorder = in("sideBarBorder")
	c.AffectSashHover = in("sashHover")
	c.AffectStatusAndTitleBorders = in("statusAndTitleBorders")
	c.AffectDebuggingStatusBar = in("debuggingStatusBar")
	c.AffectTabActiveBorder = in("tabActiveBorder")
	return c, nil
}

// ApplyToOptions converts Choices to runner.Options.
func ApplyToOptions(c Choices, targetDir string) runner.Options {
	opts := runner.Defaults()
	opts.TargetDir = targetDir
	switch c.ColorSource {
	case "inherit":
		opts.ColorInput = ""
	case "random":
		opts.ColorInput = "random"
	case "custom":
		opts.ColorInput = c.CustomColor
	}
	opts.NoOpen = !c.OpenAfter
	opts.KeepSource = !c.DeleteSource
	opts.Palette.Affect.ActivityBar = c.AffectActivityBar
	opts.Palette.Affect.StatusBar = c.AffectStatusBar
	opts.Palette.Affect.TitleBar = c.AffectTitleBar
	opts.Palette.Affect.EditorGroupBorder = c.AffectEditorGroupBorder
	opts.Palette.Affect.PanelBorder = c.AffectPanelBorder
	opts.Palette.Affect.SideBarBorder = c.AffectSideBarBorder
	opts.Palette.Affect.SashHover = c.AffectSashHover
	opts.Palette.Affect.StatusAndTitleBorders = c.AffectStatusAndTitleBorders
	opts.Palette.Affect.DebuggingStatusBar = c.AffectDebuggingStatusBar
	opts.Palette.Affect.TabActiveBorder = c.AffectTabActiveBorder
	if c.Advanced {
		opts.Palette.Standard.KeepForegroundColor = c.KeepForegroundColor
		opts.Palette.Standard.KeepBadgeColor = c.KeepBadgeColor
		opts.Palette.Standard.SquigglyBeGone = c.SquigglyBeGone
		if pct, err := parseFloat(c.DarkenLightenPct); err == nil && pct > 0 {
			opts.Palette.Standard.DarkenLightenPct = pct
		}
	}
	return opts
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
