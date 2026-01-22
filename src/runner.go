package src

type Runner struct {
	utils       UtilsInterface
	viewBuilder ViewBuilderInterface
}

func NewRunner(u UtilsInterface, b ViewBuilderInterface) *Runner {
	return &Runner{
		utils:       u,
		viewBuilder: b,
	}
}

func (r *Runner) Start() {
	styles := DefaultStyles()

	// Example menu
	choices := []ListItem{
		{
			T: "start",
			D: "Start a new game",
		},
		{
			T: "quit",
			D: "Exit the application",
		},
	}

	answer := r.viewBuilder.NewListView("Terminal Gameplay - Main Menu", choices, 16)
	r.utils.ValidateInput(answer.T)

	if answer.T == "start" {
		playerName := r.viewBuilder.NewTextFieldView("Enter your player name", "Player")
		r.utils.ValidateInput(playerName)

		println(styles.Text("Welcome, "+playerName+"!", styles.SuccessColor))
	}
}
