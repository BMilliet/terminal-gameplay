package settings

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFeaturesUnmarshalAppliesDefaultsForMissingFlags(t *testing.T) {
	var features FeaturesDTO
	if err := json.Unmarshal([]byte(`{"scripts":false}`), &features); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !features.FrequentGoTo {
		t.Fatalf("FrequentGoTo = false, want default true")
	}
	if features.Scripts {
		t.Fatalf("Scripts = true, want parsed false")
	}
	if !features.Notes {
		t.Fatalf("Notes = false, want default true")
	}
	if !features.Env {
		t.Fatalf("Env = false, want default true")
	}
	if !features.Alias {
		t.Fatalf("Alias = false, want default true")
	}
	if features.Frequencies == nil {
		t.Fatalf("Frequencies is nil, want initialized map")
	}
}

func TestFeaturesIncrementGoToNormalizesFrequencyMap(t *testing.T) {
	features := FeaturesDTO{}

	features.IncrementGoTo("work")

	if got := features.Frequencies["work"]; got != 1 {
		t.Fatalf("frequency = %d, want 1", got)
	}
}

func TestBuildFeaturesListReflectsEnabledStatus(t *testing.T) {
	features := &FeaturesDTO{
		FrequentGoTo: true,
		Scripts:      false,
		Notes:        true,
		Env:          false,
		Alias:        true,
	}

	got := BuildFeaturesList(features)

	want := []string{"enabled ✓", "disabled ✗", "enabled ✓", "disabled ✗", "enabled ✓"}
	statuses := []string{got[0].D, got[1].D, got[2].D, got[3].D, got[4].D}
	if !reflect.DeepEqual(statuses, want) {
		t.Fatalf("feature statuses = %#v, want %#v", statuses, want)
	}
}
