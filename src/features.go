package src

import "sort"

type FeaturesDTO struct {
	FrequentGoTo bool           `json:"frequent_goTo"`
	Notes        bool           `json:"notes"`
	Frequencies  map[string]int `json:"frequencies"`
}

func GetDefaultFeatures() *FeaturesDTO {
	return &FeaturesDTO{
		FrequentGoTo: true,
		Notes:        true,
		Frequencies:  make(map[string]int),
	}
}

func (f *FeaturesDTO) Normalize() {
	if f.Frequencies == nil {
		f.Frequencies = make(map[string]int)
	}
}

func (f *FeaturesDTO) IncrementGoTo(key string) {
	f.Normalize()
	f.Frequencies[key]++
}

func (f *FeaturesDTO) GetTopGoToKeys() []string {
	if len(f.Frequencies) == 0 {
		return []string{}
	}

	type keyFreq struct {
		key  string
		freq int
	}

	items := make([]keyFreq, 0, len(f.Frequencies))
	for k, v := range f.Frequencies {
		items = append(items, keyFreq{k, v})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].freq > items[j].freq
	})

	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.key
	}

	return keys
}

func (f *FeaturesDTO) FrequencyIsEmpty() bool {
	return len(f.Frequencies) == 0
}
