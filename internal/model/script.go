package model

// Error code for post-processing script failures.
const CodeScriptError = "SCRIPT_ERROR"

// Supported script languages.
const (
	ScriptPython = "python"
	ScriptNode   = "node"
	ScriptRuby   = "ruby"
	ScriptShell  = "shell"
)

// ScriptConfig is the post-processing script definition. The body is written
// inside the app (never loaded from etcd data), and the app wraps it with the
// input/output variable bindings before running it.
type ScriptConfig struct {
	Language string `json:"language"`

	// InputVar is the variable the raw value bytes are bound to. Empty means
	// nothing is injected; the script can still read stdin.
	InputVar string `json:"inputVar"`

	// OutputVar is the variable read back as the displayed result. Empty means
	// the script's stdout is displayed as-is.
	OutputVar string `json:"outputVar"`

	Body string `json:"body"`
}

// ScriptInfo is the runtime view of the configured script for the UI.
type ScriptInfo struct {
	Configured  bool   `json:"configured"`
	Language    string `json:"language"`
	InputVar    string `json:"inputVar"`
	OutputVar   string `json:"outputVar"`
	Interpreter string `json:"interpreter"`
	Available   bool   `json:"available"`
	Warning     string `json:"warning,omitempty"`
}

// ScriptOutput is the result of running the script over one value.
type ScriptOutput struct {
	Output     string `json:"output"` // what the UI displays
	Stderr     string `json:"stderr"` // shown when the script fails
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	Truncated  bool   `json:"truncated"`
}
