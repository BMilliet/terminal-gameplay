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

	// Load or create default features
	featuresContent, err := r.fileManager.GetFeaturesContent()
	if err != nil {
		r.utils.HandleError(err, "Failed to read features")
	}

	var features *FeaturesDTO
	if featuresContent == "" {
		features = GetDefaultFeatures()
		r.migrateLegacyFeatures(features)
		jsonStr, err := ToJSON(features)
		if err != nil {
			r.utils.HandleError(err, "Failed to create default features")
		}
		if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
			r.utils.HandleError(err, "Failed to write default features")
		}
	} else {
		features, err = ParseJSONContent[FeaturesDTO](featuresContent)
		if err != nil {
			r.utils.HandleError(err, "Failed to parse features.json")
		}
		features.Normalize()
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

	if features.Notes {
		if err := r.fileManager.SyncNotesContent(&config.Notes); err != nil {
			r.utils.HandleError(err, "Failed to initialize notes")
		}
	}

	for {
		// Show multi-page view
		result := r.viewBuilder.NewMultiPageView(config, features)
		r.utils.ValidateInput(result)

		// Parse result: "page|label|value"
		parts := strings.SplitN(result, "|", 3)
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
			case "clear_frequency":
				// Clear the frequency history
				features.Frequencies = make(map[string]int)
				jsonStr, err := ToJSON(features)
				if err != nil {
					r.utils.HandleError(err, "Failed to serialize features")
				}
				if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
					r.utils.HandleError(err, "Failed to write features")
				}

				println(styles.Text("✓ Frequency history cleared", styles.AquamarineColor))
			}
			return

		case "features":
			switch label {
			case "frequent_goTo":
				features.FrequentGoTo = !features.FrequentGoTo
			case "notes":
				features.Notes = !features.Notes
				if features.Notes {
					if err := r.fileManager.SyncNotesContent(&config.Notes); err != nil {
						r.utils.HandleError(err, "Failed to initialize notes")
					}
				}
			}

			jsonStr, err := ToJSON(features)
			if err != nil {
				r.utils.HandleError(err, "Failed to serialize features")
			}
			if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
				r.utils.HandleError(err, "Failed to write features")
			}

			continue

		case "goTo", "frequent":
			// Increment goTo frequency counter if it's a goTo navigation
			if features.FrequentGoTo {
				features.IncrementGoTo(label)
				jsonStr, err := ToJSON(features)
				if err != nil {
					r.utils.HandleError(err, "Failed to serialize features")
				}
				if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
					r.utils.HandleError(err, "Failed to write features")
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
			return

		case "commands":
			println(styles.Text("\n⚠️  Commands execution not implemented yet", styles.ErrorColor))
			return

		case "notes":
			if label == AddNoteAction {
				if err := r.createNote(config); err != nil {
					r.utils.HandleError(err, "Failed to create note")
				}
				continue
			}

			noteContent, ok := config.Notes.Get(label)
			if !ok {
				noteContent = value
			}

			notePath, err := r.fileManager.EnsureNoteFile(label, noteContent)
			if err != nil {
				r.utils.HandleError(err, "Failed to prepare note")
			}

			if err := r.utils.OpenInNvim(notePath); err != nil {
				r.utils.HandleError(err, "Failed to open note in nvim")
			}

			if err := r.fileManager.SyncNotesContent(&config.Notes); err != nil {
				r.utils.HandleError(err, "Failed to reload notes")
			}
		}
	}
}

func (r *Runner) createNote(config *ConfigDTO) error {
	noteName := strings.TrimSpace(r.viewBuilder.NewTextFieldView("New note filename", "daily-note.md"))
	if noteName == ExitSignal || noteName == "" {
		return nil
	}

	notePath, err := r.fileManager.EnsureNoteFile(noteName, "")
	if err != nil {
		return err
	}

	if err := r.utils.OpenInNvim(notePath); err != nil {
		return err
	}

	content, err := r.fileManager.ReadFileContent(notePath)
	if err != nil {
		return err
	}

	config.Notes.Set(noteName, content)
	jsonStr, err := ToJSON(config)
	if err != nil {
		return err
	}

	if err := r.fileManager.WriteConfigContent(jsonStr); err != nil {
		return err
	}

	return r.fileManager.SyncNotesContent(&config.Notes)
}

func (r *Runner) migrateLegacyFeatures(features *FeaturesDTO) {
	optionsContent, err := r.fileManager.GetOptionsContent()
	if err == nil && strings.TrimSpace(optionsContent) != "" {
		options, err := ParseJSONContent[OptionsDTO](optionsContent)
		if err == nil {
			features.FrequentGoTo = options.FrequentGoTo
		}
	}

	goToFreqContent, err := r.fileManager.GetGoToFrequencyContent()
	if err == nil && strings.TrimSpace(goToFreqContent) != "" {
		goToFrequency, err := ParseJSONContent[GoToFrequencyDTO](goToFreqContent)
		if err == nil && goToFrequency.Frequencies != nil {
			features.Frequencies = goToFrequency.Frequencies
		}
	}

	features.Normalize()
}
