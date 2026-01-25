package src

import (
	"fmt"
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
	if len(config.Warp.Keys) == 0 && len(config.Commands.Keys) == 0 && len(config.Notes.Keys) == 0 {
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
	value := parts[2]

	// Handle based on page type
	switch page {
	case "warp":
		// Expand ~ to home directory
		expandedPath := r.utils.ExpandPath(value)

		// Write cd command to file
		cmdFile := r.fileManager.(*FileManager).AppDir + "/cmd-exec"
		command := fmt.Sprintf("cd %s", expandedPath)

		if err := r.fileManager.WriteFileContent(cmdFile, command); err != nil {
			r.utils.HandleError(err, "Failed to write command file")
		}

	case "commands":
		println(styles.Text("\n⚠️  Commands execution not implemented yet", styles.ErrorColor))

	case "notes":
		// Copy value to clipboard
		if err := r.utils.CopyToClipboard(value); err != nil {
			r.utils.HandleError(err, "Failed to copy to clipboard")
		}

		println(styles.Text("✓ Copied to clipboard: "+value, styles.AquamarineColor))
	}
}
