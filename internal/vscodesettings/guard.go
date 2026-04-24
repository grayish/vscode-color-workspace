package vscodesettings

import (
	"sort"

	"github.com/sang-bin/vscode-color-workspace/internal/peacock"
)

// ResidualColorKeys returns the list of keys that would remain in
// workbench.colorCustomizations after deleting Peacock-managed keys.
// Used by Guard 2.
func ResidualColorKeys(s *Settings) []string {
	if s == nil {
		return nil
	}
	cc, ok := s.Raw[peacock.SectionColorCustomizations].(map[string]any)
	if !ok {
		return nil
	}
	pk := peacock.ColorKeysSet()
	var out []string
	for k := range cc {
		if !pk[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
