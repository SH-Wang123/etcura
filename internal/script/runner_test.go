package script

import (
	"context"
	"strings"
	"testing"
	"time"

	"etcura/internal/model"
)

func newRunner(t *testing.T, cfg model.ScriptConfig) *Runner {
	t.Helper()
	r := NewRunner()
	r.Timeout = 10 * time.Second
	if err := r.Set(cfg); err != nil {
		t.Fatalf("Set: %v", err)
	}
	return r
}

func mustRun(t *testing.T, r *Runner, data []byte) model.ScriptOutput {
	t.Helper()
	out, err := r.Run(context.Background(), data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out
}

// The value is bound to the configured input variable and the configured
// output variable is displayed.
func TestPythonInputOutputVariables(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptPython,
		InputVar:  "payload",
		OutputVar: "result",
		Body:      "result = payload.hex().upper()",
	})
	out := mustRun(t, r, []byte{0x00, 0xff, 0x1a})
	if strings.TrimSpace(out.Output) != "00FF1A" {
		t.Fatalf("output = %q, want 00FF1A", out.Output)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit = %d, stderr=%s", out.ExitCode, out.Stderr)
	}
}

// Both variable names may be empty: no input binding, and stdout is shown.
func TestPythonNoVariablesUsesStdin(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language: model.ScriptPython,
		Body:     "import sys\nsys.stdout.write(sys.stdin.buffer.read().hex())",
	})
	out := mustRun(t, r, []byte{0xde, 0xad})
	if strings.TrimSpace(out.Output) != "dead" {
		t.Fatalf("output = %q, want dead", out.Output)
	}
}

// An input variable without an output variable: the snippet prints itself.
func TestInputOnly(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language: model.ScriptPython,
		InputVar: "payload",
		Body:     "print(len(payload))",
	})
	out := mustRun(t, r, []byte("abcd"))
	if strings.TrimSpace(out.Output) != "4" {
		t.Fatalf("output = %q, want 4", out.Output)
	}
}

// An output variable without an input variable: stdout from the binding only.
func TestOutputOnly(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptPython,
		OutputVar: "result",
		Body:      "import sys\nresult = 'len=' + str(len(sys.stdin.buffer.read()))",
	})
	out := mustRun(t, r, []byte("abcde"))
	if strings.TrimSpace(out.Output) != "len=5" {
		t.Fatalf("output = %q, want len=5", out.Output)
	}
}

func TestNodeVariables(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptNode,
		InputVar:  "payload",
		OutputVar: "result",
		Body:      "const result = payload.length + ':' + payload[0];",
	})
	out := mustRun(t, r, []byte{0x07, 0x08, 0x09})
	if strings.TrimSpace(out.Output) != "3:7" {
		t.Fatalf("output = %q, want 3:7", out.Output)
	}
}

func TestRubyVariables(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptRuby,
		InputVar:  "payload",
		OutputVar: "result",
		Body:      "result = payload.bytes.map { |b| b.to_s(16).rjust(2, '0') }.join",
	})
	out := mustRun(t, r, []byte{0x0a, 0xff})
	if strings.TrimSpace(out.Output) != "0aff" {
		t.Fatalf("output = %q, want 0aff", out.Output)
	}
}

// Shell cannot hold NUL in a variable, so the input variable is a path to a
// file containing the raw bytes.
func TestShellVariables(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptShell,
		InputVar:  "payload",
		OutputVar: "result",
		Body:      "result=$(od -An -tx1 < \"$payload\" | tr -d ' \\n')",
	})
	out := mustRun(t, r, []byte{0x00, 0x10, 0xff})
	if strings.TrimSpace(out.Output) != "0010ff" {
		t.Fatalf("output = %q, want 0010ff", out.Output)
	}
}

// A failing snippet reports its exit code and stderr rather than erroring out,
// so the UI can show what went wrong.
func TestFailureReported(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language: model.ScriptPython,
		Body:     "import sys\nprint('boom', file=sys.stderr)\nsys.exit(3)",
	})
	out := mustRun(t, r, []byte("x"))
	if out.ExitCode != 3 {
		t.Fatalf("exit = %d, want 3", out.ExitCode)
	}
	if !strings.Contains(out.Stderr, "boom") {
		t.Fatalf("stderr = %q", out.Stderr)
	}
}

// An undefined output variable is a normal failure the user can see.
func TestUndefinedOutputVariable(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptPython,
		InputVar:  "payload",
		OutputVar: "never_set",
		Body:      "pass",
	})
	out := mustRun(t, r, []byte("x"))
	if out.ExitCode == 0 {
		t.Fatal("expected a non-zero exit for an undefined output variable")
	}
	if !strings.Contains(out.Stderr, "never_set") {
		t.Fatalf("stderr should name the variable: %q", out.Stderr)
	}
}

// A hanging snippet is killed, including its grandchildren.
func TestTimeout(t *testing.T) {
	r := NewRunner()
	r.Timeout = 2 * time.Second
	if err := r.Set(model.ScriptConfig{
		Language: model.ScriptPython,
		Body:     "import time\ntime.sleep(30)",
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := r.Run(context.Background(), []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		name string
		cfg  model.ScriptConfig
		want string
	}{
		{"bad language", model.ScriptConfig{Language: "brainfuck", Body: "x"}, "unsupported script language"},
		{"empty body", model.ScriptConfig{Language: model.ScriptPython, Body: "   "}, "must not be empty"},
		{"bad input var", model.ScriptConfig{Language: model.ScriptPython, Body: "x", InputVar: "1bad"}, "input variable name"},
		{"bad output var", model.ScriptConfig{Language: model.ScriptPython, Body: "x", OutputVar: "has space"}, "output variable name"},
	}
	for _, c := range cases {
		err := Validate(c.cfg)
		if err == nil {
			t.Fatalf("%s: expected an error", c.name)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: error = %v, want it to mention %q", c.name, err, c.want)
		}
	}

	// empty names are explicitly allowed
	if err := Validate(model.ScriptConfig{Language: model.ScriptPython, Body: "print(1)"}); err != nil {
		t.Fatalf("empty variable names must be valid: %v", err)
	}
	// and so are valid identifiers
	if err := Validate(model.ScriptConfig{Language: model.ScriptPython, Body: "x", InputVar: "_in", OutputVar: "out2"}); err != nil {
		t.Fatalf("valid identifiers rejected: %v", err)
	}
}

func TestInfoAndClear(t *testing.T) {
	r := NewRunner()
	if r.Info() != nil {
		t.Fatal("Info must be nil before Set")
	}
	if _, err := r.Run(context.Background(), nil); err == nil {
		t.Fatal("Run without a script must fail")
	}
	if err := r.Set(model.ScriptConfig{Language: model.ScriptPython, Body: "print(1)", InputVar: "v"}); err != nil {
		t.Fatal(err)
	}
	info := r.Info()
	if info == nil || !info.Configured || info.Interpreter != "python3" || info.InputVar != "v" {
		t.Fatalf("info = %+v", info)
	}
	r.Clear()
	if r.Info() != nil || r.Config() != nil {
		t.Fatal("Clear must reset the runner")
	}
}

// The value never becomes code: a payload that looks like source is only data.
func TestValueIsDataNotCode(t *testing.T) {
	r := newRunner(t, model.ScriptConfig{
		Language:  model.ScriptPython,
		InputVar:  "payload",
		OutputVar: "result",
		Body:      "result = payload.decode('utf-8', 'replace')",
	})
	out := mustRun(t, r, []byte("__import__('os').system('echo pwned')"))
	if strings.Contains(out.Output, "pwned\n") && out.Output == "pwned" {
		t.Fatal("value was executed as code")
	}
	if !strings.Contains(out.Output, "__import__") {
		t.Fatalf("value should be returned as text, got %q", out.Output)
	}
}
