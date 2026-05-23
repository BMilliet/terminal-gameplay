package gototab

import (
	"reflect"
	"testing"

	"terminal-gameplay/internal/utils"
)

func TestBuildSectionOptionsReturnsRootAndDividerSections(t *testing.T) {
	goTo := utils.OrderedMap{
		Keys: []string{"home", "div", "work", "div2", "personal"},
		Values: map[string]string{
			"home":     "~",
			"div":      "work projects",
			"work":     "~/work",
			"div2":     "personal projects",
			"personal": "~/personal",
		},
	}

	got := BuildSectionOptions(goTo)

	want := []utils.ListItem{
		{T: RootSection, D: "top / no section"},
		{T: "div", D: "work projects"},
		{T: "div2", D: "personal projects"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSectionOptions() = %#v, want %#v", got, want)
	}
}
