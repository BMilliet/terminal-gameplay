package src

import "sort"

type WarpFrequencyDTO struct {
	Frequencies map[string]int `json:"frequencies"`
}

func GetDefaultWarpFrequency() *WarpFrequencyDTO {
	return &WarpFrequencyDTO{
		Frequencies: make(map[string]int),
	}
}

// IncrementWarp increments the frequency count for a given warp key
func (wf *WarpFrequencyDTO) IncrementWarp(key string) {
	if wf.Frequencies == nil {
		wf.Frequencies = make(map[string]int)
	}
	wf.Frequencies[key]++
}

// GetTopWarpKeys returns warp keys sorted by frequency (most frequent first)
func (wf *WarpFrequencyDTO) GetTopWarpKeys() []string {
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
func (wf *WarpFrequencyDTO) IsEmpty() bool {
	return len(wf.Frequencies) == 0
}
