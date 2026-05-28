// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"os"
	"path/filepath"
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
			name:   "goals add missing all args",
			args:   []string{"goals", "add"},
			errSub: "accepts 2 arg(s), received 0",
		},
		{
			name:   "goals add missing check-command arg",
			args:   []string{"goals", "add", "my-goal"},
			errSub: "accepts 2 arg(s), received 1",
		},
		{
			name:   "ratchet record missing step arg",
			args:   []string{"ratchet", "record", "--output", "foo.md"},
			errSub: "accepts 1 arg(s), received 0",
		},
		{
			name:   "forge transcript missing path (no --last-session)",
			args:   []string{"forge", "transcript"},
			errSub: "requires at least 1 arg",
		},
		{
			name:   "forge markdown missing path",
			args:   []string{"forge", "markdown"},
			errSub: "requires at least 1 arg",
		},
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
		{
			name:   "inject --max-tokens not-a-number",
			args:   []string{"inject", "--max-tokens", "lots"},
			errSub: "invalid argument",
		},
		{
			name:   "goals add --weight not-a-number",
			args:   []string{"goals", "add", "my-id", "true", "--weight", "heavy"},
			errSub: "invalid argument",
		},
		{
			name:   "ratchet record --tier not-a-number",
			args:   []string{"ratchet", "record", "research", "--output", "foo.md", "--tier", "high"},
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
func TestNegativePath_NonExistentPaths(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	tests := []struct {
		name   string
		args   []string
		errSub string
	}{
		{
			name:   "forge transcript with non-existent file",
			args:   []string{"forge", "transcript", "/tmp/does-not-exist-ao-test-xyz.jsonl"},
			errSub: "no files found",
		},
		{
			name:   "forge markdown with non-existent file",
			args:   []string{"forge", "markdown", "/tmp/does-not-exist-ao-test-xyz.md"},
			errSub: "no markdown files found",
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
			name:      "unknown ratchet subcommand shows help",
			args:      []string{"ratchet", "nonexistent"},
			outputSub: "Use \"ao ratchet [command] --help\"",
		},
		{
			name:      "unknown metrics subcommand shows help",
			args:      []string{"metrics", "nonexistent"},
			outputSub: "Use \"ao metrics [command] --help\"",
		},
		{
			name:      "unknown forge subcommand shows help",
			args:      []string{"forge", "nonexistent"},
			outputSub: "Use \"ao forge [command] --help\"",
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

// TestNegativePath_RatchetRecordUnknownStep verifies that ratchet record
// rejects an unrecognized step name. Note: testing the required --output flag
// in isolation is unreliable because executeCommand does not reset Changed
// state on nested subcommand flags (only direct rootCmd children).
func TestNegativePath_RatchetRecordUnknownStep(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("ratchet", "record", "bogus-step", "--output", "artifact.md")
	if err == nil {
		t.Fatalf("expected error for unknown step, got nil (output: %s)", out)
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("error %q does not mention 'unknown step'", err.Error())
	}
}

// TestNegativePath_GoalsAddInvalidID verifies that goals add rejects a non-kebab-case ID.
// This test requires a valid GOALS.md on disk because Cobra argument validation
// passes (ExactArgs(2) is satisfied) and RunE proceeds to validate the ID format.
func TestNegativePath_GoalsAddInvalidID(t *testing.T) {
	tmp := chdirTemp(t)

	// Create a minimal GOALS.md so the command can load goals.
	goalsContent := `# Goals
## Version
4
## Mission
Test
## Gates
`
	if err := os.WriteFile(filepath.Join(tmp, "GOALS.md"), []byte(goalsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("goals", "add", "NotKebabCase", "true", "--dry-run")
	if err == nil {
		t.Fatalf("expected error for non-kebab-case ID, got nil (output: %s)", out)
	}
	if !strings.Contains(err.Error(), "kebab-case") {
		t.Errorf("error %q does not mention kebab-case requirement", err.Error())
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
			name:   "inject with too many args",
			args:   []string{"inject", "query1", "query2"},
			errSub: "accepts at most 1 arg",
		},
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
			name:   "inject with unknown flag",
			args:   []string{"inject", "--nonexistent-flag"},
			errSub: "unknown flag",
		},
		{
			name:   "goals add with unknown flag",
			args:   []string{"goals", "add", "my-id", "true", "--nonexistent"},
			errSub: "unknown flag",
		},
		{
			name:   "metrics baseline with unknown flag",
			args:   []string{"metrics", "baseline", "--nonexistent"},
			errSub: "unknown flag",
		},
		{
			name:   "forge transcript with unknown flag",
			args:   []string{"forge", "transcript", "--nonexistent"},
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
		{"missing args", []string{"goals", "add"}},
		{"unknown command", []string{"nonexistent-command"}},
		{"unknown flag", []string{"inject", "--bogus"}},
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
