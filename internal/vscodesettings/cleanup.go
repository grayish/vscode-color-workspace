package vscodesettings

import "github.com/sang-bin/vscode-color-workspace/internal/peacock"

// Cleanup removes all peacock-managed keys in place. Returns true if any
// change was made. Safe on nil.
//
// Removes:
//   - any key starting with "peacock."
//   - in workbench.colorCustomizations, any key in peacock.ColorKeys()
//   - the workbench.colorCustomizations key itself if it becomes empty
func Cleanup(s *Settings) bool {
	if s == nil || s.Raw == nil {
		return false
	}
	changed := false
	for k := range s.Raw {
		if peacock.HasPeacockPrefix(k) {
			delete(s.Raw, k)
			changed = true
		}
	}
	if cc, ok := s.Raw[peacock.SectionColorCustomizations].(map[string]any); ok {
		pk := peacock.ColorKeysSet()
		for k := range cc {
			if pk[k] {
				delete(cc, k)
				changed = true
			}
		}
		if len(cc) == 0 {
			delete(s.Raw, peacock.SectionColorCustomizations)
		}
	}
	return changed
}

// IsEmpty reports whether the settings map has zero keys.
func (s *Settings) IsEmpty() bool {
	return s == nil || len(s.Raw) == 0
}
