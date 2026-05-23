package frequent

import (
	"sort"

	"terminal-gameplay/internal/utils"
)

const PageName = "frequent"

type GoToFrequencyDTO struct {
	Frequencies map[string]int `json:"frequencies"`
}

func GetDefaultGoToFrequency() *GoToFrequencyDTO {
	return &GoToFrequencyDTO{
		Frequencies: make(map[string]int),
	}
}

func Normalize(frequencies map[string]int) map[string]int {
	if frequencies == nil {
		return make(map[string]int)
	}
	return frequencies
}

func Increment(frequencies map[string]int, key string) map[string]int {
	frequencies = Normalize(frequencies)
	frequencies[key]++
	return frequencies
}

func TopKeys(frequencies map[string]int) []string {
	if len(frequencies) == 0 {
		return []string{}
	}

	type keyFreq struct {
		key  string
		freq int
	}

	items := make([]keyFreq, 0, len(frequencies))
	for k, v := range frequencies {
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

func IsEmpty(frequencies map[string]int) bool {
	return len(frequencies) == 0
}

func BuildList(goTo utils.OrderedMap, enabled bool, frequencies map[string]int) []utils.ListItem {
	if !enabled || IsEmpty(frequencies) {
		return []utils.ListItem{}
	}

	list := []utils.ListItem{}
	for _, key := range TopKeys(frequencies) {
		if value, exists := goTo.Values[key]; exists {
			list = append(list, utils.ListItem{
				T:     key,
				D:     value,
				IsDiv: false,
			})
		}
	}

	return list
}

// IncrementGoTo increments the frequency count for a given goTo key
func (wf *GoToFrequencyDTO) IncrementGoTo(key string) {
	wf.Frequencies = Increment(wf.Frequencies, key)
}

// GetTopGoToKeys returns goTo keys sorted by frequency (most frequent first)
func (wf *GoToFrequencyDTO) GetTopGoToKeys() []string {
	return TopKeys(wf.Frequencies)
}

// IsEmpty returns true if there are no recorded frequencies
func (wf *GoToFrequencyDTO) IsEmpty() bool {
	return IsEmpty(wf.Frequencies)
}
