package tools

import "terminal-gameplay/internal/utils"

const (
	PageName            = "tools"
	SearchReplaceAction = "search/replace"
	ReplaceAllAction    = "__REPLACE_ALL__"
	ReplacedStatus      = "replaced"
)

func BuildList() []utils.ListItem {
	return []utils.ListItem{
		{
			T: SearchReplaceAction,
			D: "find and replace text under current directory",
		},
	}
}
