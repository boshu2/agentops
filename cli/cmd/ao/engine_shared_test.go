// practices: [design-by-contract, ai-assisted-dev]
package main

import (
	"reflect"
	"testing"
)

// TestSplitRuntimeCommand covers the cmd/ao wrapper after the runtime-command
// helpers were extracted from internal/rpi to internal/runtimecmd (age-tlj6).
// The wrapper must split on the FIRST token and preserve prefix args.
func TestSplitRuntimeCommand(t *testing.T) {
	tests := []struct {
		command  string
		wantExec string
		wantArgs []string
	}{
		{"codex", "codex", []string{}},
		{"env -i claude", "env", []string{"-i", "claude"}},
		{"  codex   exec ", "codex", []string{"exec"}},
		{"/usr/local/bin/claude --flag", "/usr/local/bin/claude", []string{"--flag"}},
		{"", "", nil},
	}
	for _, tt := range tests {
		gotExec, gotArgs := splitRuntimeCommand(tt.command)
		if gotExec != tt.wantExec {
			t.Errorf("splitRuntimeCommand(%q) exec = %q, want %q", tt.command, gotExec, tt.wantExec)
		}
		if (len(gotArgs) != 0 || len(tt.wantArgs) != 0) && !reflect.DeepEqual(gotArgs, tt.wantArgs) {
			t.Errorf("splitRuntimeCommand(%q) args = %v, want %v", tt.command, gotArgs, tt.wantArgs)
		}
	}
}
