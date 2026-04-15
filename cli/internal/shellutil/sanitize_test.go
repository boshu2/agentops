package shellutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizedEnv_StripsBashEnvAndEnv(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"BASH_ENV=/tmp/leak.sh",
		"HOME=/home/test",
		"ENV=/tmp/posix-leak.sh",
		"USER=test",
	}
	got := SanitizedEnv(in)
	for _, entry := range got {
		if strings.HasPrefix(entry, "BASH_ENV=") {
			t.Errorf("BASH_ENV should be stripped; saw %q", entry)
		}
		if strings.HasPrefix(entry, "ENV=") {
			t.Errorf("ENV should be stripped; saw %q", entry)
		}
	}
	// Verify retained vars survive.
	want := map[string]bool{
		"PATH=/usr/bin":  false,
		"HOME=/home/test": false,
		"USER=test":      false,
	}
	for _, entry := range got {
		if _, ok := want[entry]; ok {
			want[entry] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected env entry %q to be retained, was missing", k)
		}
	}
}

func TestSanitizedBashCommand_HasExpectedArgs(t *testing.T) {
	cmd := SanitizedBashCommand(nil, "echo hi")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if len(cmd.Args) < 4 {
		t.Fatalf("expected at least 4 args, got %d: %v", len(cmd.Args), cmd.Args)
	}
	// cmd.Args[0] is the bash program name; assert flags are present.
	hasNoProfile := false
	hasNoRc := false
	hasDashC := false
	for _, a := range cmd.Args {
		switch a {
		case "--noprofile":
			hasNoProfile = true
		case "--norc":
			hasNoRc = true
		case "-c":
			hasDashC = true
		}
	}
	if !hasNoProfile {
		t.Error("missing --noprofile flag")
	}
	if !hasNoRc {
		t.Error("missing --norc flag")
	}
	if !hasDashC {
		t.Error("missing -c flag")
	}
}

func TestSanitizedBashCommand_StripsBashEnvFromCmdEnv(t *testing.T) {
	t.Setenv("BASH_ENV", "/tmp/should-be-stripped.sh")
	t.Setenv("ENV", "/tmp/posix-should-be-stripped.sh")
	cmd := SanitizedBashCommand(context.Background(), "true")
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, "BASH_ENV=") {
			t.Errorf("cmd.Env still contains BASH_ENV: %q", entry)
		}
		if strings.HasPrefix(entry, "ENV=") {
			t.Errorf("cmd.Env still contains ENV: %q", entry)
		}
	}
}

// TestSanitizedBashCommand_AliasesDoNotLeak proves that an alias defined in a
// fake rcfile pointed to by BASH_ENV does NOT take effect when the script runs
// through SanitizedBashCommand. This is the regression test for the worker
// alias-leak bug.
func TestSanitizedBashCommand_AliasesDoNotLeak(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		if _, err := os.Stat("/usr/bin/bash"); err != nil {
			t.Skip("bash not available")
		}
	}

	tmp := t.TempDir()
	rcfile := filepath.Join(tmp, "leaky.sh")
	// This rcfile, if sourced, would make `echo` print "HIJACKED" instead of
	// the script's argument. If our sanitizer fails, the test will detect it.
	rcContents := `shopt -s expand_aliases
alias echo='printf HIJACKED\n #'
`
	if err := os.WriteFile(rcfile, []byte(rcContents), 0o600); err != nil {
		t.Fatalf("write rcfile: %v", err)
	}
	t.Setenv("BASH_ENV", rcfile)

	cmd := SanitizedBashCommand(context.Background(), "echo expected-output")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd failed: %v (output=%q)", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	if got != "expected-output" {
		t.Errorf("alias leak: expected %q, got %q (rcfile from BASH_ENV was sourced)", "expected-output", got)
	}
	if strings.Contains(got, "HIJACKED") {
		t.Errorf("alias leak detected: output contains HIJACKED")
	}
}

// TestSanitizedBashCommand_ContextCancellation verifies that cancelling the
// context terminates the command.
func TestSanitizedBashCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := SanitizedBashCommand(ctx, "sleep 60")
	err := cmd.Run()
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
