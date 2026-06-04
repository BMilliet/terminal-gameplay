package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/frequent"
	gototab "terminal-gameplay/internal/goto"
	"terminal-gameplay/internal/notes"
	"terminal-gameplay/internal/scripts"
	"terminal-gameplay/internal/settings"
	toolstab "terminal-gameplay/internal/tools"
	"terminal-gameplay/internal/ui"
	"terminal-gameplay/internal/utils"
)

type Runner struct {
	fileManager      utils.FileManagerInterface
	runtime          utils.UtilsInterface
	viewBuilder      ui.ViewBuilderInterface
	pendingEnvUnsets map[string]struct{}
}

func NewRunner(fm utils.FileManagerInterface, runtime utils.UtilsInterface, b ui.ViewBuilderInterface) *Runner {
	return &Runner{
		fileManager:      fm,
		runtime:          runtime,
		viewBuilder:      b,
		pendingEnvUnsets: make(map[string]struct{}),
	}
}

func (r *Runner) Start() {
	styles := ui.DefaultStyles()

	// Initialize application directory and config file
	if err := r.fileManager.BasicSetup(); err != nil {
		r.runtime.HandleError(err, "Failed to initialize application")
	}
	if err := r.fileManager.DeleteFileIfExists(r.fileManager.CommandExecPath()); err != nil {
		r.runtime.HandleError(err, "Failed to clear pending shell command")
	}

	// Load or create default features
	featuresContent, err := r.fileManager.GetFeaturesContent()
	if err != nil {
		r.runtime.HandleError(err, "Failed to read features")
	}

	var features *settings.FeaturesDTO
	if featuresContent == "" {
		features = settings.GetDefaultFeatures()
		r.migrateLegacyFeatures(features)
		jsonStr, err := utils.ToJSON(features)
		if err != nil {
			r.runtime.HandleError(err, "Failed to create default features")
		}
		if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
			r.runtime.HandleError(err, "Failed to write default features")
		}
	} else {
		features, err = utils.ParseJSONContent[settings.FeaturesDTO](featuresContent)
		if err != nil {
			r.runtime.HandleError(err, "Failed to parse features.json")
		}
		features.Normalize()
	}

	// Load or create default config
	configContent, err := r.fileManager.GetConfigContent()
	if err != nil {
		r.runtime.HandleError(err, "Failed to read config")
	}

	var config *utils.ConfigDTO
	migratedLegacyCommands := false
	if configContent == "" {
		// Create default config
		config = utils.GetDefaultConfig()
		jsonStr, err := utils.ToJSON(config)
		if err != nil {
			r.runtime.HandleError(err, "Failed to create default config")
		}
		if err := r.fileManager.WriteConfigContent(jsonStr); err != nil {
			r.runtime.HandleError(err, "Failed to write default config")
		}
	} else {
		config, err = utils.ParseJSONContent[utils.ConfigDTO](configContent)
		if err != nil {
			r.runtime.HandleError(err, "Failed to parse config.json")
		}
		if config.MigratedLegacyCommands() {
			migratedLegacyCommands = true
		}
	}

	if err := r.syncEnvironment(config); err != nil {
		r.runtime.HandleError(err, "Failed to apply environment")
	}

	if features.Scripts {
		if err := scripts.SyncFiles(r.fileManager, &config.Scripts); err != nil {
			r.runtime.HandleError(err, "Failed to initialize scripts")
		}
	}

	if features.Notes {
		if err := notes.SyncContent(r.fileManager, &config.Notes); err != nil {
			r.runtime.HandleError(err, "Failed to initialize notes")
		}
	}

	if migratedLegacyCommands {
		if err := r.writeConfig(config); err != nil {
			r.runtime.HandleError(err, "Failed to migrate commands to scripts")
		}
	}

	nextPage := ""
	for {
		// Show multi-page view
		result := r.viewBuilder.NewMultiPageView(config, features, nextPage)
		nextPage = ""
		r.runtime.ValidateInput(result)

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
		case settings.PageName:
			// Handle settings toggle
			switch label {
			case settings.ClearFrequencyAction:
				// Clear the frequency history
				features.Frequencies = make(map[string]int)
				jsonStr, err := utils.ToJSON(features)
				if err != nil {
					r.runtime.HandleError(err, "Failed to serialize features")
				}
				if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
					r.runtime.HandleError(err, "Failed to write features")
				}

				println(styles.Text("✓ Frequency history cleared", styles.AquamarineColor))
			}
			return

		case settings.FeaturesPageName:
			switch label {
			case settings.FrequentGoToFeature:
				features.FrequentGoTo = !features.FrequentGoTo
			case settings.ScriptsFeature:
				features.Scripts = !features.Scripts
				if features.Scripts {
					if err := scripts.SyncFiles(r.fileManager, &config.Scripts); err != nil {
						r.runtime.HandleError(err, "Failed to initialize scripts")
					}
				}
			case settings.NotesFeature:
				features.Notes = !features.Notes
				if features.Notes {
					if err := notes.SyncContent(r.fileManager, &config.Notes); err != nil {
						r.runtime.HandleError(err, "Failed to initialize notes")
					}
				}
			}

			jsonStr, err := utils.ToJSON(features)
			if err != nil {
				r.runtime.HandleError(err, "Failed to serialize features")
			}
			if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
				r.runtime.HandleError(err, "Failed to write features")
			}

			continue

		case toolstab.PageName:
			switch label {
			case toolstab.SearchReplaceAction:
				nextPage = toolstab.PageName
				if err := r.searchReplace(styles); err != nil {
					if errors.Is(err, toolstab.ErrRipgrepNotFound) {
						fmt.Println(styles.Text("ripgrep (rg) not found in PATH; search/replace aborted", styles.ErrorColor))
						continue
					}
					r.runtime.HandleError(err, "Failed to run search/replace")
				}
				continue
			}

		case gototab.PageName, frequent.PageName:
			if page == gototab.PageName {
				switch label {
				case gototab.AddAction:
					if err := r.createGoTo(config); err != nil {
						r.runtime.HandleError(err, "Failed to create goTo")
					}
					continue
				case gototab.DeleteAction:
					if r.confirmDelete("goTo", value) {
						if err := r.deleteGoTo(config, features, value); err != nil {
							r.runtime.HandleError(err, "Failed to delete goTo")
						}
					}
					continue
				}
			}

			// Increment goTo frequency counter if it's a goTo navigation
			if features.FrequentGoTo {
				features.IncrementGoTo(label)
				jsonStr, err := utils.ToJSON(features)
				if err != nil {
					r.runtime.HandleError(err, "Failed to serialize features")
				}
				if err := r.fileManager.WriteFeaturesContent(jsonStr); err != nil {
					r.runtime.HandleError(err, "Failed to write features")
				}
			}

			// Expand ~ to home directory
			expandedPath := r.runtime.ExpandPath(value)

			// Write cd command to file
			command := fmt.Sprintf("cd %s", expandedPath)

			if err := r.writeShellCommand(config, command); err != nil {
				r.runtime.HandleError(err, "Failed to write command file")
			}
			return

		case envtab.PageName:
			nextPage = envtab.PageName
			switch label {
			case envtab.AddAction:
				if err := r.createEnv(config); err != nil {
					r.runtime.HandleError(err, "Failed to create env")
				}
			case envtab.DeleteAction:
				if r.confirmDelete("env", value) {
					if err := r.deleteEnv(config, value); err != nil {
						r.runtime.HandleError(err, "Failed to delete env")
					}
				}
			default:
				if err := r.toggleEnv(config, label); err != nil {
					r.runtime.HandleError(err, "Failed to toggle env")
				}
			}
			continue

		case scripts.PageName:
			switch label {
			case scripts.AddAction:
				if err := r.createScript(config); err != nil {
					r.runtime.HandleError(err, "Failed to create script")
				}
				continue
			case scripts.EditAction:
				if err := r.editScript(value); err != nil {
					r.runtime.HandleError(err, "Failed to edit script")
				}
				continue
			case scripts.DeleteAction:
				if r.confirmDelete("script", value) {
					if err := r.deleteScript(config, value); err != nil {
						r.runtime.HandleError(err, "Failed to delete script")
					}
				}
				continue
			}

			confirmed := r.confirmScriptExecution(label)
			if !confirmed {
				continue
			}

			if err := r.runScript(label, value); err != nil {
				r.runtime.HandleError(err, "Failed to run script")
			}
			return

		case notes.PageName:
			if label == notes.AddAction {
				if err := r.createNote(config); err != nil {
					r.runtime.HandleError(err, "Failed to create note")
				}
				continue
			}
			if label == notes.DeleteAction {
				if r.confirmDelete("note", value) {
					if err := r.deleteNote(config, value); err != nil {
						r.runtime.HandleError(err, "Failed to delete note")
					}
				}
				continue
			}

			noteContent, ok := config.Notes.Get(label)
			if !ok {
				noteContent = value
			}

			notePath, err := notes.EnsureFile(r.fileManager, label, noteContent)
			if err != nil {
				r.runtime.HandleError(err, "Failed to prepare note")
			}

			if err := r.runtime.OpenInNvim(notePath); err != nil {
				r.runtime.HandleError(err, "Failed to open note in nvim")
			}

			if err := notes.SyncContent(r.fileManager, &config.Notes); err != nil {
				r.runtime.HandleError(err, "Failed to reload notes")
			}
		}
	}
}

func (r *Runner) searchReplace(styles *ui.Styles) error {
	if err := toolstab.EnsureRipgrep(); err != nil {
		return err
	}

	search := r.viewBuilder.NewTextFieldView("search:", "term")
	if search == utils.ExitSignal {
		return nil
	}

	replace := r.viewBuilder.NewTextFieldView("replace:", "replacement")
	if replace == utils.ExitSignal {
		return nil
	}

	root, err := r.fileManager.GetCurrentPath()
	if err != nil {
		return err
	}

	files, err := toolstab.FindFiles(root, search)
	if err != nil {
		return err
	}

	items := toolstab.FileListItems(root, files)
	if len(items) == 0 {
		fmt.Println(styles.Text("No files found with the search term", styles.FooterColor))
		return nil
	}

	originalContent := make(map[string]string)
	for {
		selected := r.viewBuilder.NewSearchReplaceFilesView(
			fmt.Sprintf("search/replace: %q -> %q", search, replace),
			items,
			10,
		)

		switch selected.T {
		case utils.ExitSignal:
			return nil
		case toolstab.ReplaceAllAction:
			if err := r.replaceAllVisibleItems(items, search, replace, originalContent); err != nil {
				return err
			}
			continue
		}

		filePath := selected.D
		if filePath == "" {
			filePath = selected.T
		}

		if selected.Status == toolstab.ReplacedStatus {
			if err := r.restoreReplacedItem(items, filePath, originalContent); err != nil {
				return err
			}
			continue
		}

		if err := r.replacePendingItem(items, filePath, search, replace, originalContent); err != nil {
			return err
		}
	}
}

func (r *Runner) replaceAllVisibleItems(items []utils.ListItem, search, replace string, originalContent map[string]string) error {
	for _, item := range items {
		if item.Status == toolstab.ReplacedStatus {
			continue
		}

		filePath := item.D
		if filePath == "" {
			filePath = item.T
		}

		if err := r.replacePendingItem(items, filePath, search, replace, originalContent); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) replacePendingItem(items []utils.ListItem, filePath, search, replace string, originalContent map[string]string) error {
	content, err := toolstab.ReadFileContent(filePath)
	if err != nil {
		return err
	}

	count := strings.Count(content, search)
	if count == 0 {
		return nil
	}

	if _, ok := originalContent[filePath]; !ok {
		originalContent[filePath] = content
	}

	if err := toolstab.WriteFileContent(filePath, strings.ReplaceAll(content, search, replace)); err != nil {
		return err
	}

	markItemStatus(items, filePath, toolstab.ReplacedStatus)
	return nil
}

func (r *Runner) restoreReplacedItem(items []utils.ListItem, filePath string, originalContent map[string]string) error {
	content, ok := originalContent[filePath]
	if !ok {
		return nil
	}

	if err := toolstab.WriteFileContent(filePath, content); err != nil {
		return err
	}

	delete(originalContent, filePath)
	markItemStatus(items, filePath, "")
	return nil
}

func markItemStatus(items []utils.ListItem, filePath, status string) {
	for i := range items {
		if items[i].D == filePath || items[i].T == filePath {
			items[i].Status = status
			return
		}
	}
}

func (r *Runner) createGoTo(config *utils.ConfigDTO) error {
	currentPath, err := r.fileManager.GetCurrentPath()
	if err != nil {
		return err
	}

	contractedPath := r.runtime.ContractPath(currentPath)
	defaultName := filepath.Base(currentPath)
	if contractedPath == "~" {
		defaultName = "home"
	}
	if defaultName == "." || defaultName == string(filepath.Separator) {
		defaultName = "home"
	}

	goToName := strings.TrimSpace(r.viewBuilder.NewTextFieldView("New goTo name", defaultName))
	if goToName == utils.ExitSignal || goToName == "" {
		return nil
	}
	if utils.IsDividerKey(goToName) {
		return fmt.Errorf("goTo name cannot start with div")
	}

	if _, exists := config.GoTo.Get(goToName); exists {
		if !r.viewBuilder.NewConfirmView(fmt.Sprintf("Replace goTo %q?", goToName)) {
			return nil
		}
	}

	sectionKey, ok, err := r.selectGoToSection(config)
	if err != nil || !ok {
		return err
	}

	config.GoTo.InsertInSection(gototab.RootSection, sectionKey, goToName, contractedPath)
	return r.writeConfig(config)
}

func (r *Runner) selectGoToSection(config *utils.ConfigDTO) (string, bool, error) {
	for {
		selected := r.viewBuilder.NewSectionSelectView("Select goTo section", gototab.BuildSectionOptions(config.GoTo))
		if selected.T == utils.ExitSignal {
			return "", false, nil
		}

		if selected.T != gototab.AddSectionAction {
			return selected.T, true, nil
		}

		sectionName := strings.TrimSpace(r.viewBuilder.NewTextFieldView("New section name", "work"))
		if sectionName == utils.ExitSignal {
			return "", false, nil
		}
		if sectionName == "" {
			continue
		}

		return config.GoTo.AddDivider(sectionName), true, nil
	}
}

func (r *Runner) createNote(config *utils.ConfigDTO) error {
	noteName := strings.TrimSpace(r.viewBuilder.NewTextFieldView("New note name", "test_file_name.md"))
	if noteName == utils.ExitSignal || noteName == "" {
		return nil
	}

	noteFileName := notes.FileName(noteName)
	notePath, err := notes.EnsureFile(r.fileManager, noteFileName, "")
	if err != nil {
		return err
	}

	if err := r.runtime.OpenInNvim(notePath); err != nil {
		return err
	}

	content, err := r.fileManager.ReadFileContent(notePath)
	if err != nil {
		return err
	}

	config.Notes.Set(noteFileName, content)
	jsonStr, err := utils.ToJSON(config)
	if err != nil {
		return err
	}

	if err := r.fileManager.WriteConfigContent(jsonStr); err != nil {
		return err
	}

	return notes.SyncContent(r.fileManager, &config.Notes)
}

func (r *Runner) createScript(config *utils.ConfigDTO) error {
	scriptName := strings.TrimSpace(r.viewBuilder.NewTextFieldView("New script name", "get current branch"))
	if scriptName == utils.ExitSignal || scriptName == "" {
		return nil
	}

	description := strings.TrimSpace(r.viewBuilder.NewTextFieldView("Script description", "what this script does"))
	if description == utils.ExitSignal {
		return nil
	}
	if description == "" {
		description = "No description"
	}

	scriptPath, err := scripts.EnsureFile(r.fileManager, scriptName, description)
	if err != nil {
		return err
	}

	if err := r.runtime.OpenInNvim(scriptPath); err != nil {
		return err
	}

	config.Scripts.Set(scriptName, description)
	if err := r.writeConfig(config); err != nil {
		return err
	}

	return scripts.SyncFiles(r.fileManager, &config.Scripts)
}

func (r *Runner) createEnv(config *utils.ConfigDTO) error {
	key := r.viewBuilder.NewTextFieldView("Env key", "FOO")
	if key == utils.ExitSignal {
		return nil
	}
	key = strings.TrimSpace(key)
	if err := envtab.ValidateKey(key); err != nil {
		return err
	}

	value := r.viewBuilder.NewTextFieldView("Env value", "123")
	if value == utils.ExitSignal {
		return nil
	}

	if _, exists := config.Env.Get(key); exists {
		if !r.viewBuilder.NewConfirmView(fmt.Sprintf("Replace env %q?", key)) {
			return nil
		}
	}

	config.Env.Set(key, value, true)
	delete(r.pendingEnvUnsets, key)
	if err := r.writeConfig(config); err != nil {
		return err
	}
	return r.syncEnvironment(config)
}

func (r *Runner) toggleEnv(config *utils.ConfigDTO, key string) error {
	value, ok := config.Env.Get(key)
	if !ok {
		return nil
	}

	config.Env.Set(key, value.Value, !value.Active)
	delete(r.pendingEnvUnsets, key)
	if err := r.writeConfig(config); err != nil {
		return err
	}
	return r.syncEnvironment(config)
}

func (r *Runner) deleteEnv(config *utils.ConfigDTO, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if _, ok := config.Env.Get(key); !ok {
		return nil
	}

	config.Env.Delete(key)
	r.pendingEnvUnsets[key] = struct{}{}
	if err := r.runtime.UnsetEnv(key); err != nil {
		return err
	}
	if err := r.writeConfig(config); err != nil {
		return err
	}
	return r.writeShellCommand(config, "")
}

func (r *Runner) editScript(scriptName string) error {
	scriptName = strings.TrimSpace(scriptName)
	if scriptName == "" {
		return nil
	}

	scriptPath, err := scripts.EnsureFile(r.fileManager, scriptName, "")
	if err != nil {
		return err
	}

	return r.runtime.OpenInNvim(scriptPath)
}

func (r *Runner) deleteGoTo(config *utils.ConfigDTO, features *settings.FeaturesDTO, goToName string) error {
	goToName = strings.TrimSpace(goToName)
	if goToName == "" {
		return nil
	}

	config.GoTo.Delete(goToName)
	if features.Frequencies != nil {
		delete(features.Frequencies, goToName)
	}

	if err := r.writeConfig(config); err != nil {
		return err
	}
	return r.writeFeatures(features)
}

func (r *Runner) deleteNote(config *utils.ConfigDTO, noteName string) error {
	noteName = strings.TrimSpace(noteName)
	if noteName == "" {
		return nil
	}

	if err := notes.DeleteFile(r.fileManager, noteName); err != nil {
		return err
	}

	config.Notes.Delete(noteName)
	return r.writeConfig(config)
}

func (r *Runner) deleteScript(config *utils.ConfigDTO, scriptName string) error {
	scriptName = strings.TrimSpace(scriptName)
	if scriptName == "" {
		return nil
	}

	if err := scripts.DeleteFile(r.fileManager, scriptName); err != nil {
		return err
	}

	config.Scripts.Delete(scriptName)
	return r.writeConfig(config)
}

func (r *Runner) runScript(scriptName, description string) error {
	scriptPath, err := scripts.EnsureFile(r.fileManager, scriptName, description)
	if err != nil {
		return err
	}

	return r.runtime.RunLuaScript(scriptPath)
}

func (r *Runner) confirmScriptExecution(scriptName string) bool {
	return r.viewBuilder.NewConfirmView(fmt.Sprintf("Run script %q?", scriptName))
}

func (r *Runner) confirmDelete(kind, name string) bool {
	return r.viewBuilder.NewConfirmView(fmt.Sprintf("Delete %s %q?", kind, name))
}

func (r *Runner) writeConfig(config *utils.ConfigDTO) error {
	jsonStr, err := utils.ToJSON(config)
	if err != nil {
		return err
	}

	return r.fileManager.WriteConfigContent(jsonStr)
}

func (r *Runner) writeFeatures(features *settings.FeaturesDTO) error {
	jsonStr, err := utils.ToJSON(features)
	if err != nil {
		return err
	}

	return r.fileManager.WriteFeaturesContent(jsonStr)
}

func (r *Runner) syncEnvironment(config *utils.ConfigDTO) error {
	if err := envtab.Apply(config.Env, r.runtime); err != nil {
		return err
	}
	return r.writeShellCommand(config, "")
}

func (r *Runner) writeShellCommand(config *utils.ConfigDTO, command string) error {
	shell := os.Getenv(envtab.ShellIntegrationEnv)
	commands, err := envtab.ShellCommands(config.Env, shell)
	if err != nil {
		return err
	}

	pendingKeys := make([]string, 0, len(r.pendingEnvUnsets))
	for key := range r.pendingEnvUnsets {
		if _, exists := config.Env.Get(key); !exists {
			pendingKeys = append(pendingKeys, key)
		}
	}
	sort.Strings(pendingKeys)
	for _, key := range pendingKeys {
		unsetCommand, err := envtab.UnsetCommand(key, shell)
		if err != nil {
			return err
		}
		commands = append(commands, unsetCommand)
	}

	if command != "" {
		commands = append(commands, command)
	}

	cmdFile := r.fileManager.CommandExecPath()
	if len(commands) == 0 {
		return r.fileManager.DeleteFileIfExists(cmdFile)
	}
	return r.fileManager.WriteFileContent(cmdFile, strings.Join(commands, "\n"))
}

func (r *Runner) migrateLegacyFeatures(features *settings.FeaturesDTO) {
	optionsContent, err := r.fileManager.GetOptionsContent()
	if err == nil && strings.TrimSpace(optionsContent) != "" {
		options, err := utils.ParseJSONContent[settings.OptionsDTO](optionsContent)
		if err == nil {
			features.FrequentGoTo = options.FrequentGoTo
		}
	}

	goToFreqContent, err := r.fileManager.GetGoToFrequencyContent()
	if err == nil && strings.TrimSpace(goToFreqContent) != "" {
		goToFrequency, err := utils.ParseJSONContent[frequent.GoToFrequencyDTO](goToFreqContent)
		if err == nil && goToFrequency.Frequencies != nil {
			features.Frequencies = goToFrequency.Frequencies
		}
	}

	features.Normalize()
}
