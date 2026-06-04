package settings

import (
	"encoding/json"

	"terminal-gameplay/internal/frequent"
)

type FeaturesDTO struct {
	FrequentGoTo bool           `json:"frequent_goTo"`
	Scripts      bool           `json:"scripts"`
	Notes        bool           `json:"notes"`
	Env          bool           `json:"env"`
	Frequencies  map[string]int `json:"frequencies"`
}

func GetDefaultFeatures() *FeaturesDTO {
	return &FeaturesDTO{
		FrequentGoTo: true,
		Scripts:      true,
		Notes:        true,
		Env:          true,
		Frequencies:  make(map[string]int),
	}
}

func (f *FeaturesDTO) UnmarshalJSON(data []byte) error {
	type featuresJSON struct {
		FrequentGoTo *bool          `json:"frequent_goTo"`
		Scripts      *bool          `json:"scripts"`
		Notes        *bool          `json:"notes"`
		Env          *bool          `json:"env"`
		Frequencies  map[string]int `json:"frequencies"`
	}

	defaults := GetDefaultFeatures()
	*f = *defaults

	var parsed featuresJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	if parsed.FrequentGoTo != nil {
		f.FrequentGoTo = *parsed.FrequentGoTo
	}
	if parsed.Scripts != nil {
		f.Scripts = *parsed.Scripts
	}
	if parsed.Notes != nil {
		f.Notes = *parsed.Notes
	}
	if parsed.Env != nil {
		f.Env = *parsed.Env
	}
	if parsed.Frequencies != nil {
		f.Frequencies = parsed.Frequencies
	}

	f.Normalize()
	return nil
}

func (f *FeaturesDTO) Normalize() {
	f.Frequencies = frequent.Normalize(f.Frequencies)
}

func (f *FeaturesDTO) IncrementGoTo(key string) {
	f.Frequencies = frequent.Increment(f.Frequencies, key)
}

func (f *FeaturesDTO) GetTopGoToKeys() []string {
	return frequent.TopKeys(f.Frequencies)
}

func (f *FeaturesDTO) FrequencyIsEmpty() bool {
	return frequent.IsEmpty(f.Frequencies)
}
