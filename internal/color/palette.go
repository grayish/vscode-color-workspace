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
		BackgroundHover:    HoverOf(base),
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
