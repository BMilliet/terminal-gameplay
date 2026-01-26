package src

type OptionsDTO struct {
	FrequentWarp bool `json:"frequent_warp"`
}

func GetDefaultOptions() *OptionsDTO {
	return &OptionsDTO{
		FrequentWarp: true,
	}
}
