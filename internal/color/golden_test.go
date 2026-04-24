package color

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/fixture.json
var fixtureJSON []byte

type fixtureEntry struct {
	Base               string            `json:"base"`
	Label              string            `json:"label"`
	Opts               map[string]bool   `json:"opts"`
	ElementAdjustments map[string]string `json:"elementAdjustments"`
	Palette            map[string]string `json:"palette"`
}

func TestPalette_GoldenFixture(t *testing.T) {
	var entries []fixtureEntry
	if err := json.Unmarshal(fixtureJSON, &entries); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	for _, e := range entries {
		t.Run(e.Label, func(t *testing.T) {
			base, err := Parse(e.Base)
			if err != nil {
				t.Fatalf("parse %q: %v", e.Base, err)
			}
			opts := optsFromMap(e.Opts, e.ElementAdjustments)
			got := Palette(base, opts)
			if len(got) != len(e.Palette) {
				t.Errorf("palette size = %d, want %d", len(got), len(e.Palette))
			}
			for k, v := range e.Palette {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
			for k := range got {
				if _, ok := e.Palette[k]; !ok {
					t.Errorf("unexpected key %q in Go output", k)
				}
			}
		})
	}
}

func optsFromMap(m map[string]bool, adj map[string]string) Options {
	opts := DefaultOptions()
	opts.Affect = AffectOptions{
		ActivityBar:           m["activityBar"],
		StatusBar:             m["statusBar"],
		TitleBar:              m["titleBar"],
		EditorGroupBorder:     m["editorGroupBorder"],
		PanelBorder:           m["panelBorder"],
		SideBarBorder:         m["sideBarBorder"],
		SashHover:             m["sashHover"],
		StatusAndTitleBorders: m["statusAndTitleBorders"],
		DebuggingStatusBar:    m["debuggingStatusBar"],
		TabActiveBorder:       m["tabActiveBorder"],
	}
	opts.Standard.KeepForegroundColor = m["keepForegroundColor"]
	opts.Standard.KeepBadgeColor = m["keepBadgeColor"]
	opts.Standard.SquigglyBeGone = m["squigglyBeGone"]
	opts.Adjust.ActivityBar = parseAdj(adj["activityBar"])
	opts.Adjust.StatusBar = parseAdj(adj["statusBar"])
	opts.Adjust.TitleBar = parseAdj(adj["titleBar"])
	return opts
}

func parseAdj(s string) Adjustment {
	switch s {
	case "lighten":
		return AdjustLighten
	case "darken":
		return AdjustDarken
	}
	return AdjustNone
}
