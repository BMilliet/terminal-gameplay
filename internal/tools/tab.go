package tools

import "terminal-gameplay/internal/utils"

const (
	PageName                   = "tools"
	FuzzySearchAction          = "fuzzy search"
	WorkspaceFuzzySearchAction = "workspace fuzzy search"
	SearchReplaceAction        = "search/replace"
	ReplaceAllAction           = "__REPLACE_ALL__"
	ReplacedStatus             = "replaced"
	WorkspaceEnv               = "WORKSPACE"
)

func BuildList() []utils.ListItem {
	return []utils.ListItem{
		{
			T: FuzzySearchAction,
			D: "find files and directories under current directory",
		},
		{
			T: WorkspaceFuzzySearchAction,
			D: "find files and directories under $WORKSPACE",
		},
		{
			T: SearchReplaceAction,
			D: "find and replace text under current directory",
		},
	}
}
