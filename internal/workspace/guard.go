package workspace

import (
	"sort"

	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// ExistingPeacockKeys returns dotted paths of Peacock-managed keys found in
// ws. Empty if none. Empty for nil ws. Sorted for deterministic output.
func ExistingPeacockKeys(ws *Workspace) []string {
	if ws == nil {
		return nil
	}
	var out []string
	colorKeys := peacock.ColorKeysSet()
	for k, v := range ws.Settings {
		if peacock.HasPeacockPrefix(k) {
			out = append(out, "settings."+k)
			continue
		}
		if k != peacock.SectionColorCustomizations {
			continue
		}
		cc, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for ck := range cc {
			if colorKeys[ck] {
				out = append(out, "settings."+peacock.SectionColorCustomizations+"."+ck)
			}
		}
	}
	sort.Strings(out)
	return out
}
