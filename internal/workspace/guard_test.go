package workspace

import (
	"reflect"
	"sort"
	"testing"
)

func TestExistingPeacockKeys_None(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"editor.fontSize": 14.0,
	}}
	got := ExistingPeacockKeys(ws)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestExistingPeacockKeys_PeacockColor(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"peacock.color": "#5a3b8c",
	}}
	got := ExistingPeacockKeys(ws)
	want := []string{"settings.peacock.color"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExistingPeacockKeys_ColorCustomizations(t *testing.T) {
	ws := &Workspace{Settings: map[string]any{
		"workbench.colorCustomizations": map[string]any{
			"activityBar.background": "#5a3b8c",
			"editor.background":      "#000000",
		},
	}}
	got := ExistingPeacockKeys(ws)
	sort.Strings(got)
	want := []string{"settings.workbench.colorCustomizations.activityBar.background"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExistingPeacockKeys_NilWorkspace(t *testing.T) {
	if got := ExistingPeacockKeys(nil); len(got) != 0 {
		t.Errorf("nil -> %v", got)
	}
}
