package frequent

import (
	"reflect"
	"testing"

	"terminal-gameplay/internal/utils"
)

func TestIncrementNormalizesNilMap(t *testing.T) {
	frequencies := Increment(nil, "work")

	if frequencies == nil {
		t.Fatal("expected Increment to allocate a map")
	}
	if got := frequencies["work"]; got != 1 {
		t.Fatalf("expected work frequency 1, got %d", got)
	}
}

func TestTopKeysOrdersByFrequencyDescending(t *testing.T) {
	got := TopKeys(map[string]int{
		"rare":  1,
		"daily": 7,
		"often": 3,
	})

	want := []string{"daily", "often", "rare"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TopKeys() = %#v, want %#v", got, want)
	}
}

func TestBuildListOnlyIncludesKnownGoToKeysWhenEnabled(t *testing.T) {
	goTo := utils.OrderedMap{
		Keys: []string{"work", "home"},
		Values: map[string]string{
			"work": "~/work",
			"home": "~",
		},
	}

	got := BuildList(goTo, true, map[string]int{
		"missing": 10,
		"home":    5,
		"work":    2,
	})

	want := []utils.ListItem{
		{T: "home", D: "~"},
		{T: "work", D: "~/work"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildList() = %#v, want %#v", got, want)
	}

	if disabled := BuildList(goTo, false, map[string]int{"home": 5}); len(disabled) != 0 {
		t.Fatalf("BuildList disabled returned %#v, want empty list", disabled)
	}
}
