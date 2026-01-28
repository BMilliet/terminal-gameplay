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

	// Load or create default options
	optionsContent, err := r.fileManager.GetOptionsContent()
	if err != nil {
		r.utils.HandleError(err, "Failed to read options")
	}

	var options *OptionsDTO
	if optionsContent == "" {
		// Create default options
		options = GetDefaultOptions()
		jsonStr, err := ToJSON(options)
		if err != nil {
			r.utils.HandleError(err, "Failed to create default options")
		}
		if err := r.fileManager.WriteOptionsContent(jsonStr); err != nil {
			r.utils.HandleError(err, "Failed to write default options")
		}
	} else {
		options, err = ParseJSONContent[OptionsDTO](optionsContent)
		if err != nil {
			r.utils.HandleError(err, "Failed to parse options.json")
		}
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

	// Load or create default goTo frequency
	goToFreqContent, err := r.fileManager.GetGoToFrequencyContent()
	if err != nil {
		r.utils.HandleError(err, "Failed to read goTo frequency")
	}

	var goToFrequency *GoToFrequencyDTO
	if goToFreqContent == "" {
		// Create default goTo frequency
		goToFrequency = GetDefaultGoToFrequency()
		jsonStr, err := ToJSON(goToFrequency)
		if err != nil {
			r.utils.HandleError(err, "Failed to create default goTo frequency")
		}
		if err := r.fileManager.WriteGoToFrequencyContent(jsonStr); err != nil {
			r.utils.HandleError(err, "Failed to write default goTo frequency")
		}
	} else {
		goToFrequency, err = ParseJSONContent[GoToFrequencyDTO](goToFreqContent)
		if err != nil {
			r.utils.HandleError(err, "Failed to parse goto_frequency.json")
		}
	}

	// Check if all pages are empty
	if len(config.GoTo.Keys) == 0 && len(config.Commands.Keys) == 0 && len(config.Notes.Keys) == 0 {
		println(styles.Text("\n⚠️  All pages are empty!", styles.ErrorColor))
		println(styles.Text("\nPlease edit your config file:", styles.TitleColor))
		println(styles.Text("  "+r.fileManager.(*FileManager).ConfigPath, styles.FooterColor))
		println()
		return
	}

	// Show multi-page view
	result := r.viewBuilder.NewMultiPageView(config, options, goToFrequency)
	r.utils.ValidateInput(result)

	// Parse result: "page|label|value"
	parts := strings.Split(result, "|")
	if len(parts) != 3 {
		return
	}

	page := parts[0]
	label := parts[1]
	value := parts[2]

	// Handle based on page type
	switch page {
	case "settings":
		// Handle settings toggle
		switch label {
		case "frequent_goTo":
			// Toggle the frequent_goTo option
			options.FrequentGoTo = !options.FrequentGoTo

			// Save the updated options
			jsonStr, err := ToJSON(options)
			if err != nil {
				r.utils.HandleError(err, "Failed to serialize options")
			}
			if err := r.fileManager.WriteOptionsContent(jsonStr); err != nil {
				r.utils.HandleError(err, "Failed to write options")
			}

			var statusMsg string
			if options.FrequentGoTo {
				statusMsg = "✓ Frequent GoTo enabled"
			} else {
				statusMsg = "✓ Frequent GoTo disabled"
			}
			println(styles.Text(statusMsg, styles.AquamarineColor))

		case "clear_frequency":
			// Clear the frequency history
			emptyFrequency := GetDefaultGoToFrequency()
			jsonStr, err := ToJSON(emptyFrequency)
			if err != nil {
				r.utils.HandleError(err, "Failed to serialize goTo frequency")
			}
			if err := r.fileManager.WriteGoToFrequencyContent(jsonStr); err != nil {
				r.utils.HandleError(err, "Failed to write goTo frequency")
			}

			println(styles.Text("✓ Frequency history cleared", styles.AquamarineColor))
		}

	case "goTo", "frequent":
		// Increment goTo frequency counter if it's a goTo navigation
		if options.FrequentGoTo {
			goToFrequency.IncrementGoTo(label)
			jsonStr, err := ToJSON(goToFrequency)
			if err != nil {
				r.utils.HandleError(err, "Failed to serialize goTo frequency")
			}
			if err := r.fileManager.WriteGoToFrequencyContent(jsonStr); err != nil {
				r.utils.HandleError(err, "Failed to write goTo frequency")
			}
		}

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
