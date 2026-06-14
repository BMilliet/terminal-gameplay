package clipboard

import (
	"reflect"
	"testing"

	"terminal-gameplay/internal/utils"
)

func TestBuildListUsesHiddenKeysAndClipboardValues(t *testing.T) {
	items := utils.OrderedMap{
		Keys: []string{"clip", "clip1"},
		Values: map[string]string{
			"clip":  "npm test",
			"clip1": "secret-token",
		},
	}

	got := BuildList(items)

	want := []utils.ListItem{
		{T: "clip", D: "npm test"},
		{T: "clip1", D: "secret-token"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildList() = %#v, want %#v", got, want)
	}
}

func TestNextKeyUsesFirstAvailableClipboardKey(t *testing.T) {
	items := utils.OrderedMap{
		Keys: []string{"clip", "clip2"},
		Values: map[string]string{
			"clip":  "one",
			"clip2": "three",
		},
	}

	if got := NextKey(items); got != "clip1" {
		t.Fatalf("NextKey() = %q, want clip1", got)
	}
}
