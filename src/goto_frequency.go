package src

import "sort"

type GoToFrequencyDTO struct {
	Frequencies map[string]int `json:"frequencies"`
}

func GetDefaultGoToFrequency() *GoToFrequencyDTO {
	return &GoToFrequencyDTO{
		Frequencies: make(map[string]int),
	}
}

// IncrementGoTo increments the frequency count for a given goTo key
func (wf *GoToFrequencyDTO) IncrementGoTo(key string) {
	if wf.Frequencies == nil {
		wf.Frequencies = make(map[string]int)
	}
	wf.Frequencies[key]++
}

// GetTopGoToKeys returns goTo keys sorted by frequency (most frequent first)
func (wf *GoToFrequencyDTO) GetTopGoToKeys() []string {
	if len(wf.Frequencies) == 0 {
		return []string{}
	}

	type keyFreq struct {
		key  string
		freq int
	}

	// Convert map to slice for sorting
	items := make([]keyFreq, 0, len(wf.Frequencies))
	for k, v := range wf.Frequencies {
		items = append(items, keyFreq{k, v})
	}

	// Sort by frequency (descending)
	sort.Slice(items, func(i, j int) bool {
		return items[i].freq > items[j].freq
	})

	// Extract keys
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.key
	}

	return keys
}

// IsEmpty returns true if there are no recorded frequencies
func (wf *GoToFrequencyDTO) IsEmpty() bool {
	return len(wf.Frequencies) == 0
}
