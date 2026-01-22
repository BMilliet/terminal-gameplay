package src

type Runner struct {
	fileManager FileManagerInterface
	utils       UtilsInterface
	viewBuilder ViewBuilderInterface
}

func NewRunner(fm FileManagerInterface, u UtilsInterface, b ViewBuilderInterface) *Runner {
	return &Runner{
		fileManager: fm,
		utils:       u,
		viewBuilder: b,
	}
}

func (r *Runner) Start() {
	styles := DefaultStyles()

	// Initialize application directory and config file
	if err := r.fileManager.BasicSetup(); err != nil {
		r.utils.HandleError(err, "Failed to initialize application")
	}

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
