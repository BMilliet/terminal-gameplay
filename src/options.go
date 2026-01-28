package src

type OptionsDTO struct {
	FrequentGoTo bool `json:"frequent_goTo"`
}

func GetDefaultOptions() *OptionsDTO {
	return &OptionsDTO{
		FrequentGoTo: true,
	}
}
