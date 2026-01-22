package src

type ConfigDTO struct {
	Warp     map[string]string `json:"warp"`
	Commands map[string]string `json:"commands"`
	Notes    map[string]string `json:"notes"`
}

type ConfigItem struct {
	Label string
	Value string
}
