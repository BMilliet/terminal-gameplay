package src

import (
	"strings"
)

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

	// Load or create default config
	configContent, err := r.fileManager.GetConfigContent()
	if err != nil {
		r.utils.HandleError(err, "Failed to read config")
	}

	var config *ConfigDTO
	if configContent == "" {
		// Create default config
		config = GetDefaultConfig()
		jsonStr, err := ToJSON(config)
		if err != nil {
			r.utils.HandleError(err, "Failed to create default config")
		}
		if err := r.fileManager.WriteConfigContent(jsonStr); err != nil {
			r.utils.HandleError(err, "Failed to write default config")
		}
	} else {
		config, err = ParseJSONContent[ConfigDTO](configContent)
		if err != nil {
			r.utils.HandleError(err, "Failed to parse config.json")
		}
	}

	// Check if all pages are empty
	if len(config.Warp) == 0 && len(config.Commands) == 0 && len(config.Notes) == 0 {
		println(styles.Text("\n⚠️  All pages are empty!", styles.ErrorColor))
		println(styles.Text("\nPlease edit your config file:", styles.TitleColor))
		println(styles.Text("  "+r.fileManager.(*FileManager).ConfigPath, styles.FooterColor))
		println()
		return
	}

	// Show multi-page view
	result := r.viewBuilder.NewMultiPageView(config)
	r.utils.ValidateInput(result)

	// Parse result: "page|label|value"
	parts := strings.Split(result, "|")
	if len(parts) != 3 {
		return
	}

	page := parts[0]
	label := parts[1]
	value := parts[2]

	// Just print the selected value
	println()
	println(styles.Text("Selected from ["+page+"]:", styles.TitleColor))
	println(styles.Text("  Label: "+label, styles.PrimaryColor))
	println(styles.Text("  Value: "+value, styles.SuccessColor))
	println()
}
