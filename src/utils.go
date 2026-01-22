package src

import (
	"fmt"
	"os"
)

type UtilsInterface interface {
	ValidateInput(input string)
	ExitWithError(message string)
	HandleError(err error, message string)
}

type Utils struct{}

func NewUtils() *Utils {
	return &Utils{}
}

func (u *Utils) ValidateInput(input string) {
	if input == ExitSignal {
		fmt.Println("\nExiting...")
		os.Exit(0)
	}
}

func (u *Utils) ExitWithError(message string) {
	styles := DefaultStyles()
	fmt.Println(styles.Text(message, styles.ErrorColor))
	os.Exit(1)
}

func (u *Utils) HandleError(err error, message string) {
	if err != nil {
		styles := DefaultStyles()
		fullMessage := fmt.Sprintf("%s: %v", message, err)
		fmt.Println(styles.Text(fullMessage, styles.ErrorColor))
		os.Exit(1)
	}
}
