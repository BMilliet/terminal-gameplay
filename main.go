package main

import (
	"terminal-gameplay/src"
)

func main() {
	utils := src.NewUtils()
	viewBuilder := src.NewViewBuilder()

	runner := src.NewRunner(utils, viewBuilder)

	runner.Start()
}
