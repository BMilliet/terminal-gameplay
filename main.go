package main

import (
	"log"
	"terminal-gameplay/src"
)

func main() {
	fileManager, err := src.NewFileManager()
	if err != nil {
		log.Fatalln(err, "Failed to initialize FileManager")
	}

	utils := src.NewUtils()
	viewBuilder := src.NewViewBuilder()

	runner := src.NewRunner(fileManager, utils, viewBuilder)

	runner.Start()
}
