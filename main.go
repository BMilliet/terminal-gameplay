package main

import (
	"log"

	"terminal-gameplay/internal/app"
	"terminal-gameplay/internal/ui"
	"terminal-gameplay/internal/utils"
)

func main() {
	fileManager, err := utils.NewFileManager()
	if err != nil {
		log.Fatalln(err, "Failed to initialize FileManager")
	}

	runtime := utils.NewUtils()
	viewBuilder := ui.NewViewBuilder()

	runner := app.NewRunner(fileManager, runtime, viewBuilder)

	runner.Start()
}
