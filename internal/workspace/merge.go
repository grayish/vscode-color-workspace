package workspace

import "github.com/sang-bin/vscode-color-workspace/internal/peacock"

// ApplyPeacock writes peacock.color and merges palette into
// workbench.colorCustomizations, preserving unrelated keys. Does not modify
// Folders (caller handles that).
func ApplyPeacock(ws *Workspace, colorHex string, palette map[string]string) {
	if ws.Settings == nil {
		ws.Settings = map[string]any{}
	}
	ws.Settings[peacock.SettingColor] = colorHex

	cc, ok := ws.Settings[peacock.SectionColorCustomizations].(map[string]any)
	if !ok {
		cc = map[string]any{}
	}
	for k, v := range palette {
		cc[k] = v
	}
	if len(cc) > 0 {
		ws.Settings[peacock.SectionColorCustomizations] = cc
	}
}

// EnsureFolder inserts a folder entry with the given relative path if not
// already present.
func EnsureFolder(ws *Workspace, path string) {
	for _, f := range ws.Folders {
		if f.Path == path {
			return
		}
	}
	ws.Folders = append(ws.Folders, Folder{Path: path})
}
