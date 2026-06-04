package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"terminal-gameplay/internal/app"
	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/shellstate"
	"terminal-gameplay/internal/ui"
	"terminal-gameplay/internal/utils"
)

const shellInitFlag = "--shell-init"

func main() {
	fileManager, err := utils.NewFileManager()
	if err != nil {
		log.Fatalln(err, "Failed to initialize FileManager")
	}

	if len(os.Args) > 1 && os.Args[1] == shellInitFlag {
		commands, err := shellstate.LoadCommands(fileManager, os.Getenv(envtab.ShellIntegrationEnv))
		if err != nil {
			log.Fatalln(err, "Failed to initialize shell state")
		}
		fmt.Print(strings.Join(commands, "\n"))
		return
	}

	runtime := utils.NewUtils()
	viewBuilder := ui.NewViewBuilder()

	runner := app.NewRunner(fileManager, runtime, viewBuilder)

	runner.Start()
}
