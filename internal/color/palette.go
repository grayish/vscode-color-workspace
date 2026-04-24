package color

// AffectOptions mirrors Peacock's AffectedSettings enum.
type AffectOptions struct {
	ActivityBar           bool
	StatusBar             bool
	TitleBar              bool
	EditorGroupBorder     bool
	PanelBorder           bool
	SideBarBorder         bool
	SashHover             bool
	StatusAndTitleBorders bool
	DebuggingStatusBar    bool
	TabActiveBorder       bool
}

// StandardOptions mirrors Peacock's StandardSettings toggles.
type StandardOptions struct {
	KeepForegroundColor bool
	KeepBadgeColor      bool
	SquigglyBeGone      bool
	DarkenLightenPct    float64
	DarkForegroundHex   string
	LightForegroundHex  string
}

// Options is the full palette configuration.
type Options struct {
	Affect   AffectOptions
	Standard StandardOptions
}

// DefaultOptions mirrors Peacock's defaults.
func DefaultOptions() Options {
	return Options{
		Affect: AffectOptions{
			ActivityBar: true,
			StatusBar:   true,
			TitleBar:    true,
		},
		Standard: StandardOptions{
			DarkenLightenPct:   10,
			DarkForegroundHex:  "#15202b",
			LightForegroundHex: "#e7e7e7",
		},
	}
}

type elementStyleT struct {
	Background         Color
	BackgroundHover    Color
	Inactive           Color
	Foreground         Color
	InactiveForeground Color
	BadgeBackground    Color
	BadgeForeground    Color
}

func elementStyle(base Color, opts Options) elementStyleT {
	fg := foregroundFor(base, opts)
	badgeBg := ReadableAccent(base, RatioUILow)
	badgeFg := foregroundFor(badgeBg, opts)
	return elementStyleT{
		Background:         base,
		BackgroundHover:    hoverOfAmount(base, hoverPct(opts)),
		Inactive:           base,
		Foreground:         fg,
		InactiveForeground: fg,
		BadgeBackground:    badgeBg,
		BadgeForeground:    badgeFg,
	}
}

func foregroundFor(bg Color, opts Options) Color {
	var hex string
	if bg.IsLight() {
		hex = opts.Standard.DarkForegroundHex
		if hex == "" { hex = "#15202b" }
	} else {
		hex = opts.Standard.LightForegroundHex
		if hex == "" { hex = "#e7e7e7" }
	}
	c, _ := parseHex(hex)
	return c
}

func hoverPct(opts Options) float64 {
	if opts.Standard.DarkenLightenPct > 0 {
		return opts.Standard.DarkenLightenPct
	}
	return DefaultLightenDarkenAmount
}

func collectStatusBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.StatusBar { return out }
	style := elementStyle(base, opts)
	out["statusBar.background"] = style.Background.Hex()
	out["statusBarItem.hoverBackground"] = style.BackgroundHover.Hex()
	out["statusBarItem.remoteBackground"] = style.Background.Hex()

	if opts.Affect.StatusAndTitleBorders {
		out["statusBar.border"] = style.Background.Hex()
	}
	if !opts.Standard.KeepForegroundColor {
		out["statusBar.foreground"] = style.Foreground.Hex()
		out["statusBarItem.remoteForeground"] = style.Foreground.Hex()
	}
	if opts.Affect.DebuggingStatusBar {
		debugBg := base.Complement()
		out["statusBar.debuggingBackground"] = debugBg.Hex()
		if opts.Affect.StatusAndTitleBorders {
			out["statusBar.debuggingBorder"] = debugBg.Hex()
		}
		if !opts.Standard.KeepForegroundColor {
			out["statusBar.debuggingForeground"] = foregroundFor(debugBg, opts).Hex()
		}
	}
	return out
}

func collectActivityBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.ActivityBar { return out }
	style := elementStyle(base, opts)
	out["activityBar.background"] = style.Background.Hex()
	out["activityBar.activeBackground"] = style.Background.Hex()
	if !opts.Standard.KeepForegroundColor {
		out["activityBar.foreground"] = style.Foreground.Hex()
		out["activityBar.inactiveForeground"] = style.Foreground.HexWithAlpha(0x99)
	}
	if !opts.Standard.KeepBadgeColor {
		out["activityBarBadge.background"] = style.BadgeBackground.Hex()
		out["activityBarBadge.foreground"] = style.BadgeForeground.Hex()
	}
	return out
}

// collectTitleBar ports collectTitleBarSettings from Peacock.
func collectTitleBar(base Color, opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Affect.TitleBar { return out }
	style := elementStyle(base, opts)
	out["titleBar.activeBackground"] = style.Background.Hex()
	if opts.Affect.StatusAndTitleBorders {
		out["titleBar.border"] = style.Background.Hex()
	}
	out["titleBar.inactiveBackground"] = style.Background.HexWithAlpha(0x99)
	if !opts.Standard.KeepForegroundColor {
		out["titleBar.activeForeground"] = style.Foreground.Hex()
		out["titleBar.inactiveForeground"] = style.Foreground.HexWithAlpha(0x99)
		out["commandCenter.border"] = style.Foreground.HexWithAlpha(0x99)
	}
	return out
}

func collectAccentBorder(base Color, opts Options) map[string]string {
	out := map[string]string{}
	hex := base.Hex()
	if opts.Affect.EditorGroupBorder { out["editorGroup.border"] = hex }
	if opts.Affect.PanelBorder { out["panel.border"] = hex }
	if opts.Affect.SideBarBorder { out["sideBar.border"] = hex }
	if opts.Affect.SashHover { out["sash.hoverBorder"] = hex }
	if opts.Affect.TabActiveBorder { out["tab.activeBorder"] = hex }
	return out
}

func collectSquigglyBeGone(opts Options) map[string]string {
	out := map[string]string{}
	if !opts.Standard.SquigglyBeGone { return out }
	const transparent = "#00000000"
	out["editorError.foreground"] = transparent
	out["editorWarning.foreground"] = transparent
	out["editorInfo.foreground"] = transparent
	return out
}

// Palette builds the workbench.colorCustomizations map for a base color.
// Only emits keys for enabled affected elements.
func Palette(base Color, opts Options) map[string]string {
	out := map[string]string{}
	for _, f := range []func(Color, Options) map[string]string{
		collectTitleBar, collectActivityBar, collectStatusBar, collectAccentBorder,
	} {
		for k, v := range f(base, opts) { out[k] = v }
	}
	for k, v := range collectSquigglyBeGone(opts) { out[k] = v }
	return out
}
