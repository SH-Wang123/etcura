// Package script runs a user-authored post-processing snippet over an etcd
// value.
//
// Model: the app wraps the snippet with bindings it generates — the raw value
// bytes are bound to the configured input variable, and the configured output
// variable is read back as the displayed result. Both names may be empty:
// then nothing is bound on that side and stdout is displayed as-is. The raw
// bytes are also always piped to stdin, so a snippet can read them directly.
//
// Trust model: the snippet is written by the user inside the app; etcd data is
// only ever injected as data (hex literal / stdin / temp file), never as code.
package script

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"etcura/internal/model"
)

const (
	// DefaultTimeout bounds one run so a hanging snippet cannot wedge the UI.
	DefaultTimeout = 15 * time.Second
	// MaxOutputBytes caps captured stdout/stderr.
	MaxOutputBytes = 8 << 20 // 8 MB
	// MaxInputBytes caps how much value data is injected.
	MaxInputBytes = 8 << 20
	// MaxBodyBytes caps the snippet size.
	MaxBodyBytes = 1 << 20
)

// language describes how to invoke and wrap one supported language.
type language struct {
	id          string
	interpreter string
	ext         string
	// comment is the line-comment prefix of the language ("#" vs "//").
	comment string
	// inputBinding returns the code that binds the value to `name`. For shell
	// the reference is a path to a file holding the raw bytes, because shell
	// variables cannot contain NUL.
	inputBinding func(name, ref string) string
	// outputBinding returns the code that emits `name`.
	outputBinding func(name string) string
	// shellInput switches the input reference to a temp file path.
	shellInput bool
}

var languages = map[string]language{
	model.ScriptPython: {
		id:          model.ScriptPython,
		interpreter: "python3",
		ext:         ".py",
		comment:     "#",
		inputBinding: func(name, ref string) string {
			return fmt.Sprintf("%s = bytes.fromhex(%q)", name, ref)
		},
		outputBinding: func(name string) string { return "print(" + name + ")" },
	},
	model.ScriptNode: {
		id:          model.ScriptNode,
		interpreter: "node",
		ext:         ".js",
		comment:     "//",
		inputBinding: func(name, ref string) string {
			return fmt.Sprintf("const %s = Buffer.from(%q, 'hex');", name, ref)
		},
		outputBinding: func(name string) string { return "console.log(" + name + ")" },
	},
	model.ScriptRuby: {
		id:          model.ScriptRuby,
		interpreter: "ruby",
		ext:         ".rb",
		comment:     "#",
		inputBinding: func(name, ref string) string {
			return fmt.Sprintf("%s = [%q].pack('H*')", name, ref)
		},
		outputBinding: func(name string) string { return "puts " + name },
	},
	model.ScriptShell: {
		id:          model.ScriptShell,
		interpreter: "sh",
		ext:         ".sh",
		comment:     "#",
		shellInput:  true,
		inputBinding: func(name, ref string) string {
			return fmt.Sprintf("%s=%q", name, ref)
		},
		outputBinding: func(name string) string { return `printf '%s\n' "$` + name + `"` },
	},
}

// Languages lists supported language ids in a stable order for the UI.
func Languages() []string {
	return []string{model.ScriptPython, model.ScriptNode, model.ScriptRuby, model.ScriptShell}
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Runner holds the configured snippet and executes it on demand. Safe for
// concurrent use.
type Runner struct {
	mu  sync.RWMutex
	cfg *model.ScriptConfig

	// Timeout bounds one run; zero means DefaultTimeout.
	Timeout time.Duration
}

func NewRunner() *Runner { return &Runner{} }

func (r *Runner) timeout() time.Duration {
	r.mu.RLock()
	t := r.Timeout
	r.mu.RUnlock()
	if t <= 0 {
		return DefaultTimeout
	}
	return t
}

// Validate checks a configuration without storing it.
func Validate(cfg model.ScriptConfig) error {
	if _, ok := languages[cfg.Language]; !ok {
		return model.NewInvalidArgument("unsupported script language %q (use %s)",
			cfg.Language, strings.Join(Languages(), ", "))
	}
	if strings.TrimSpace(cfg.Body) == "" {
		return model.NewInvalidArgument("script body must not be empty")
	}
	if len(cfg.Body) > MaxBodyBytes {
		return model.NewInvalidArgument("script body is too large (%d bytes, limit %d)", len(cfg.Body), MaxBodyBytes)
	}
	for _, v := range []struct{ label, name string }{
		{"input", cfg.InputVar},
		{"output", cfg.OutputVar},
	} {
		name := strings.TrimSpace(v.name)
		if name == "" {
			continue // empty is allowed: that side is simply not bound
		}
		if !identRe.MatchString(name) {
			return model.NewInvalidArgument(
				"%s variable name %q is not a valid identifier (letters, digits and _ only, not starting with a digit)", v.label, name)
		}
	}
	return nil
}

// Set validates and stores the script configuration.
func (r *Runner) Set(cfg model.ScriptConfig) error {
	cfg.Language = strings.TrimSpace(cfg.Language)
	cfg.InputVar = strings.TrimSpace(cfg.InputVar)
	cfg.OutputVar = strings.TrimSpace(cfg.OutputVar)
	if err := Validate(cfg); err != nil {
		return err
	}
	r.mu.Lock()
	r.cfg = &cfg
	r.mu.Unlock()
	return nil
}

// Restore re-applies a persisted configuration.
func (r *Runner) Restore(cfg *model.ScriptConfig) error {
	if cfg == nil {
		return nil
	}
	return r.Set(*cfg)
}

// Clear forgets the configured script.
func (r *Runner) Clear() {
	r.mu.Lock()
	r.cfg = nil
	r.mu.Unlock()
}

// Config returns a copy of the configuration, or nil when none is set.
func (r *Runner) Config() *model.ScriptConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.cfg == nil {
		return nil
	}
	cp := *r.cfg
	return &cp
}

// Info describes the configured script for the UI.
func (r *Runner) Info() *model.ScriptInfo {
	cfg := r.Config()
	if cfg == nil {
		return nil
	}
	lang := languages[cfg.Language]
	info := &model.ScriptInfo{
		Configured:  true,
		Language:    cfg.Language,
		InputVar:    cfg.InputVar,
		OutputVar:   cfg.OutputVar,
		Interpreter: lang.interpreter,
		Available:   true,
	}
	if _, err := exec.LookPath(lang.interpreter); err != nil {
		info.Available = false
		info.Warning = fmt.Sprintf("interpreter %q not found on PATH — install it or pick another language", lang.interpreter)
	}
	return info
}

// Run wraps and executes the snippet over data, returning the displayed result.
func (r *Runner) Run(ctx context.Context, data []byte) (model.ScriptOutput, error) {
	cfg := r.Config()
	if cfg == nil {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError, "No post-processing script is configured")
	}
	if len(data) > MaxInputBytes {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError,
			"Value is too large for the script (%d bytes, limit %d)", len(data), MaxInputBytes)
	}
	lang, ok := languages[cfg.Language]
	if !ok {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError, "Unsupported script language %q", cfg.Language)
	}
	if _, err := exec.LookPath(lang.interpreter); err != nil {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError,
			"Interpreter %q was not found on PATH", lang.interpreter).WithDetail(err.Error())
	}

	// A private temp dir keeps scratch files contained; shell snippets address
	// the value through a file inside it.
	workDir, err := os.MkdirTemp("", "etcura-script-*")
	if err != nil {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError, "Cannot create a temporary directory").WithDetail(err.Error())
	}
	defer os.RemoveAll(workDir)

	inputRef := hex.EncodeToString(data)
	if lang.shellInput {
		dataPath := filepath.Join(workDir, "value.bin")
		if werr := os.WriteFile(dataPath, data, 0o600); werr != nil {
			return model.ScriptOutput{}, model.NewError(model.CodeScriptError, "Cannot stage the value for the script").WithDetail(werr.Error())
		}
		inputRef = dataPath
	}

	source := buildSource(lang, cfg, inputRef)
	scriptPath := filepath.Join(workDir, "snippet"+lang.ext)
	if werr := os.WriteFile(scriptPath, []byte(source), 0o600); werr != nil {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError, "Cannot write the script").WithDetail(werr.Error())
	}

	timeout := r.timeout()
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, lang.interpreter, scriptPath)
	configureProcessGroup(cmd)
	cmd.Cancel = func() error {
		killProcessGroup(cmd)
		return nil
	}
	// Bound the wait for output pipes: a snippet that spawned children could
	// otherwise keep Wait() blocked past the deadline.
	cmd.WaitDelay = 3 * time.Second

	// The raw bytes always reach stdin as well, so a snippet without an input
	// variable can read them directly.
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = workDir

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if cctx.Err() == context.DeadlineExceeded {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError,
			"Script timed out after %s", timeout).WithDetail(stderr.String())
	}

	out := model.ScriptOutput{
		Output:     truncate(stdout.String()),
		Stderr:     truncate(stderr.String()),
		DurationMs: elapsed.Milliseconds(),
	}
	out.Truncated = len(stdout.String()) > MaxOutputBytes
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil && out.ExitCode == 0 {
		return model.ScriptOutput{}, model.NewError(model.CodeScriptError,
			"Could not run the script").WithDetail(runErr.Error())
	}
	return out, nil
}

// buildSource assembles the runnable source: input binding, the user's body,
// then the output binding.
func buildSource(lang language, cfg *model.ScriptConfig, inputRef string) string {
	comment := lang.comment
	if comment == "" {
		comment = "#"
	}

	var b strings.Builder
	frame := func(what string) {
		fmt.Fprintf(&b, "%s ---- generated %s binding ----\n", comment, what)
	}
	if cfg.InputVar != "" {
		frame("input")
		b.WriteString(lang.inputBinding(cfg.InputVar, inputRef))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s ---- your script ----\n", comment)
	b.WriteString(cfg.Body)
	if !strings.HasSuffix(cfg.Body, "\n") {
		b.WriteString("\n")
	}
	if cfg.OutputVar != "" {
		frame("output")
		b.WriteString(lang.outputBinding(cfg.OutputVar))
		b.WriteString("\n")
	}
	return b.String()
}

// truncate caps captured text for the UI.
func truncate(s string) string {
	if len(s) <= MaxOutputBytes {
		return s
	}
	return s[:MaxOutputBytes]
}
