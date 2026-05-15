package src

const (
	ExitSignal           = "EXIT_SIGNAL"
	AddGoToAction        = "__ADD_GOTO__"
	DeleteGoToAction     = "__DELETE_GOTO__"
	AddGoToSectionAction = "__ADD_GOTO_SECTION__"
	RootGoToSection      = "__ROOT_GOTO_SECTION__"
	AddNoteAction        = "__ADD_NOTE__"
	DeleteNoteAction     = "__DELETE_NOTE__"
	AddScriptAction      = "__ADD_SCRIPT__"
	EditScriptAction     = "__EDIT_SCRIPT__"
	DeleteScriptAction   = "__DELETE_SCRIPT__"

	// Directory and file names
	AppDirName            = ".terminal-gameplay"
	NotesDirName          = "notes"
	ScriptsDirName        = "scripts"
	ConfigFileName        = "config.json"
	FeaturesFileName      = "features.json"
	OptionsFileName       = "options.json"
	GoToFrequencyFileName = "goto_frequency.json"
	NoteFileExtension     = ".md"
	ScriptFileExtension   = ".lua"
)
