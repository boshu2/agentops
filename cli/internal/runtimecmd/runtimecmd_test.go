// practices: [design-by-contract, ai-assisted-dev]
package runtimecmd

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantExec string
		wantArgs []string
	}{
		{"simple", "codex", "codex", []string{}},
		{"with prefix args", "env -i claude", "env", []string{"-i", "claude"}},
		{"extra whitespace", "  codex   exec  ", "codex", []string{"exec"}},
		{"empty", "", "", nil},
		{"whitespace only", "   ", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExec, gotArgs := Split(tt.command)
			if gotExec != tt.wantExec {
				t.Errorf("Split(%q) exec = %q, want %q", tt.command, gotExec, tt.wantExec)
			}
			if len(gotArgs) != 0 || len(tt.wantArgs) != 0 {
				if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
					t.Errorf("Split(%q) args = %v, want %v", tt.command, gotArgs, tt.wantArgs)
				}
			}
		})
	}
}

func TestBinaryName(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"codex", "codex"},
		{"/usr/local/bin/Codex", "codex"},
		{"claude.exe", "claude"},
		{"env -i codex exec", "env"}, // base name of the FIRST token
		{"", ""},
	}
	for _, tt := range tests {
		if got := BinaryName(tt.command); got != tt.want {
			t.Errorf("BinaryName(%q) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestDirectArgs(t *testing.T) {
	// codex uses `exec <prompt>`; everything else uses `-p <prompt>`.
	if got := DirectArgs("codex", "hello"); !reflect.DeepEqual(got, []string{"exec", "hello"}) {
		t.Errorf("DirectArgs codex = %v, want [exec hello]", got)
	}
	if got := DirectArgs("claude", "hello"); !reflect.DeepEqual(got, []string{"-p", "hello"}) {
		t.Errorf("DirectArgs claude = %v, want [-p hello]", got)
	}
	// prefix args are preserved; binary name is the FIRST token ("env" here, not
	// "codex"), so this takes the non-codex -p path.
	if got := DirectArgs("env -i codex", "hi"); !reflect.DeepEqual(got, []string{"-i", "codex", "-p", "hi"}) {
		t.Errorf("DirectArgs with prefix = %v, want [-i codex -p hi]", got)
	}
}
