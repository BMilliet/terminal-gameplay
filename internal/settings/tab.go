package settings

import "terminal-gameplay/internal/utils"

const (
	PageName             = "settings"
	FeaturesPageName     = "features"
	FeaturesAction       = "features"
	ClearFrequencyAction = "clear_frequency"
	FrequentGoToFeature  = "frequent_goTo"
	ScriptsFeature       = "scripts"
	NotesFeature         = "notes"
)

func BuildSettingsList() []utils.ListItem {
	return []utils.ListItem{
		{
			T:     FeaturesAction,
			D:     "configure feature flags",
			IsDiv: false,
		},
		{
			T:     ClearFrequencyAction,
			D:     "clear all frequency history",
			IsDiv: false,
		},
	}
}

func BuildFeaturesList(features *FeaturesDTO) []utils.ListItem {
	return []utils.ListItem{
		{
			T:     FrequentGoToFeature,
			D:     enabledStatus(features.FrequentGoTo),
			IsDiv: false,
		},
		{
			T:     ScriptsFeature,
			D:     enabledStatus(features.Scripts),
			IsDiv: false,
		},
		{
			T:     NotesFeature,
			D:     enabledStatus(features.Notes),
			IsDiv: false,
		},
	}
}

func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled ✓"
	}
	return "disabled ✗"
}
