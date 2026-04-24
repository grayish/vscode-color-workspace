package workspace

import "testing"

func TestApplyPeacock_NewSettings(t *testing.T) {
	ws := &Workspace{}
	palette := map[string]string{
		"activityBar.background": "#5a3b8c",
	}
	ApplyPeacock(ws, "#5a3b8c", palette)

	if ws.Settings["peacock.color"] != "#5a3b8c" {
		t.Errorf("peacock.color = %v", ws.Settings["peacock.color"])
	}
	cc, _ := ws.Settings["workbench.colorCustomizations"].(map[string]any)
	if cc["activityBar.background"] != "#5a3b8c" {
		t.Errorf("activityBar.background not applied")
	}
}

func TestApplyPeacock_PreservesOtherSettings(t *testing.T) {
	ws := &Workspace{
		Settings: map[string]any{
			"editor.fontSize": 14.0,
			"workbench.colorCustomizations": map[string]any{
				"editor.background": "#000000",
			},
		},
	}
	ApplyPeacock(ws, "#5a3b8c", map[string]string{
		"activityBar.background": "#5a3b8c",
	})

	if ws.Settings["editor.fontSize"].(float64) != 14.0 {
		t.Errorf("editor.fontSize lost")
	}
	cc := ws.Settings["workbench.colorCustomizations"].(map[string]any)
	if cc["editor.background"] != "#000000" {
		t.Error("custom colorCustomization lost")
	}
	if cc["activityBar.background"] != "#5a3b8c" {
		t.Error("peacock key not applied")
	}
}

func TestApplyPeacock_EnsuresFolders(t *testing.T) {
	ws := &Workspace{}
	ApplyPeacock(ws, "#5a3b8c", map[string]string{})
	if ws.Settings == nil {
		t.Error("settings should be initialized")
	}
	if ws.Folders != nil {
		t.Error("merge should not mutate folders")
	}
}

func TestEnsureFolder_Dedupe(t *testing.T) {
	ws := &Workspace{Folders: []Folder{{Path: "./foo"}}}
	EnsureFolder(ws, "./foo")
	if len(ws.Folders) != 1 {
		t.Errorf("folders = %d, want 1", len(ws.Folders))
	}
}

func TestEnsureFolder_Add(t *testing.T) {
	ws := &Workspace{}
	EnsureFolder(ws, "./bar")
	if len(ws.Folders) != 1 || ws.Folders[0].Path != "./bar" {
		t.Errorf("folders = %+v", ws.Folders)
	}
}
