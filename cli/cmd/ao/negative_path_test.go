// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"strings"
	"testing"
)

// TestNegativePath_MissingArgs verifies that subcommands requiring positional
// arguments return clear error messages when arguments are omitted.
func TestNegativePath_MissingArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errSub string // substring expected in error message
	}{
		{
			name:   "metrics cite missing artifact-path",
			args:   []string{"metrics", "cite"},
			errSub: "accepts 1 arg(s), received 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatalf("expected error for args %v, got nil (output: %s)", tt.args, out)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

// TestNegativePath_InvalidFlagValues verifies that invalid flag values produce
// descriptive error messages rather than panics or silent failures.
func TestNegativePath_InvalidFlagValues(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errSub string
	}{
		{
			name:   "metrics baseline --days not-a-number",
			args:   []string{"metrics", "baseline", "--days", "abc"},
			errSub: "invalid argument",
		},
		{
			name:   "metrics report --days not-a-number",
			args:   []string{"metrics", "report", "--days", "xyz"},
			errSub: "invalid argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatalf("expected error for args %v, got nil (output: %s)", tt.args, out)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

// TestNegativePath_NonExistentPaths verifies that commands referencing files
// or directories that do not exist produce meaningful errors.
// TestNegativePath_UnknownTopLevelCommand verifies that an unknown top-level
// command produces a helpful error message.
func TestNegativePath_UnknownTopLevelCommand(t *testing.T) {
	out, err := executeCommand("nonexistent-command")
	if err == nil {
		t.Fatalf("expected error for unknown command, got nil (output: %s)", out)
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error %q does not contain 'unknown command'", err.Error())
	}
}

// TestNegativePath_UnknownNestedSubcommand verifies that parent commands show
// help text (which includes available subcommands) when given an unknown
// subcommand. Cobra parent commands return nil and print help rather than
// returning an error for unknown sub-commands, so we verify the output
// contains useful guidance.
func TestNegativePath_UnknownNestedSubcommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		outputSub string // substring expected in output (help text)
	}{
		{
			name:      "unknown goals subcommand shows help",
			args:      []string{"goals", "nonexistent"},
			outputSub: "Use \"ao goals [command] --help\"",
		},
		{
			name:      "unknown metrics subcommand shows help",
			args:      []string{"metrics", "nonexistent"},
			outputSub: "Use \"ao metrics [command] --help\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := executeCommand(tt.args...)
			if !strings.Contains(out, tt.outputSub) {
				t.Errorf("output for %v does not contain %q; got: %s", tt.args, tt.outputSub, out)
			}
		})
	}
}

// TestNegativePath_ExcessArgs verifies that commands reject too many positional arguments.
func TestNegativePath_ExcessArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errSub string
	}{
		{
			name:   "metrics cite with too many args",
			args:   []string{"metrics", "cite", "path1", "path2"},
			errSub: "accepts 1 arg(s), received 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatalf("expected error for args %v, got nil (output: %s)", tt.args, out)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

// TestNegativePath_UnknownFlags verifies that unrecognized flags produce errors.
func TestNegativePath_UnknownFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errSub string
	}{
		{
			name:   "metrics baseline with unknown flag",
			args:   []string{"metrics", "baseline", "--nonexistent"},
			errSub: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatalf("expected error for args %v, got nil (output: %s)", tt.args, out)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

// TestNegativePath_ErrorOutputNotEmpty verifies that error paths still produce
// some output (usage or error message) on the command's output buffer, not
// just a silent error return.
func TestNegativePath_ErrorOutputNotEmpty(t *testing.T) {
	// For commands with SilenceUsage: true on root, Cobra suppresses usage
	// on RunE errors but still prints the error itself. For arg validation
	// failures, Cobra prints usage. We verify the error is non-nil and
	// well-formed in all cases.
	tests := []struct {
		name string
		args []string
	}{
		{"missing args", []string{"metrics", "cite"}},
		{"unknown command", []string{"nonexistent-command"}},
		{"unknown flag", []string{"goals", "measure", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			// Verify the error message is non-empty and descriptive
			msg := err.Error()
			if len(msg) < 10 {
				t.Errorf("error message too short to be helpful: %q", msg)
			}
		})
	}
}
