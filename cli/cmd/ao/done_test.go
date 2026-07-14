package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
)

func TestDoneCompositionUsesSharedResolvedCommand(t *testing.T) {
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "seed")
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(output))
	workDir := filepath.Join(root, "nested")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)

	script := filepath.Join(root, "tracker")
	scriptBody := `#!/bin/sh
printf 'cwd=%s beads=%s args=%s\n' "$PWD" "${BEADS_DIR-unset}" "$*" >> "$DONE_TRACKER_LOG"
printf 'done-stdout\n'
printf 'done-stderr\n' >&2
exit 23
`
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatal(err)
	}
	originalLookPath := trackerLookPath
	t.Cleanup(func() { trackerLookPath = originalLookPath })
	trackerLookPath = func(name string) (string, error) {
		if name == trackerBR || name == trackerBD {
			return script, nil
		}
		return "", exec.ErrNotFound
	}
	t.Setenv("HOME", t.TempDir())

	for _, testCase := range []struct {
		name      string
		tracker   string
		wantDir   string
		wantBeads string
	}{
		{name: "br", tracker: trackerBR, wantDir: workDir, wantBeads: filepath.Join(root, "ledger")},
		{name: "bd", tracker: trackerBD, wantDir: root, wantBeads: "unset"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			logPath := filepath.Join(root, testCase.name+".log")
			t.Setenv("AGENTOPS_TRACKER", testCase.tracker)
			t.Setenv("BEADS_DIR", filepath.Join(root, "ledger"))
			t.Setenv("DONE_TRACKER_LOG", logPath)

			_, executeErr := newDoneService().Execute(context.Background(), doneapp.Request{
				BeadID: "age-done", SHA: sha, Reason: "Finish", ForceNoVerdict: true,
			})
			var exit *trackerexec.ExitError
			if !errors.As(executeErr, &exit) || exit.ExitCode() != 23 {
				t.Fatalf("done error = %T %v, want shared *trackerexec.ExitError(23)", executeErr, executeErr)
			}
			if !strings.Contains(executeErr.Error(), "done-stdout") || !strings.Contains(executeErr.Error(), "done-stderr") {
				t.Fatalf("done error = %q, want combined tracker output", executeErr)
			}
			logBody, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			want := "cwd=" + testCase.wantDir + " beads=" + testCase.wantBeads +
				" args=close age-done -r Finish [verdict:" + sha[:doneapp.MinimumSHAPrefix] + ":UNVERIFIED]"
			if !strings.Contains(string(logBody), want) {
				t.Fatalf("tracker log = %q, want %q", logBody, want)
			}
		})
	}

	cancelLog := filepath.Join(root, "canceled.log")
	t.Setenv("AGENTOPS_TRACKER", trackerBR)
	t.Setenv("BEADS_DIR", filepath.Join(root, "ledger"))
	t.Setenv("DONE_TRACKER_LOG", cancelLog)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, cancelErr := newDoneService().Execute(canceled, doneapp.Request{
		BeadID: "age-canceled", SHA: sha, Reason: "Finish", ForceNoVerdict: true,
	})
	if !errors.Is(cancelErr, context.Canceled) {
		t.Fatalf("canceled done error = %T %v, want context.Canceled", cancelErr, cancelErr)
	}
	if _, err := os.Stat(cancelLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled done launched tracker child: stat error %v", err)
	}
}

func TestDoneCommandCompositionPreservesSurface(t *testing.T) {
	if doneCommand.Use != "done <bead-id>" || doneCommand.GroupID != "" {
		t.Fatalf("done command = use %q group %q", doneCommand.Use, doneCommand.GroupID)
	}
	for _, flag := range []string{"sha", "reason", "force-no-verdict", "json"} {
		if doneCommand.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag %q", flag)
		}
	}
	command, _, err := rootCmd.Find([]string{"done"})
	if err != nil || command != doneCommand {
		t.Fatalf("root registration = %p want %p err=%v", command, doneCommand, err)
	}
}

func TestDoneCommandCompositionRejectsInvalidSHAWithoutEffects(t *testing.T) {
	command := doneModule.Command()
	command.SetArgs([]string{"age-test", "--sha", "bad"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "is not a commit sha") {
		t.Fatalf("error = %v", err)
	}
}
