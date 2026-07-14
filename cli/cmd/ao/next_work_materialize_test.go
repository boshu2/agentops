package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/spf13/cobra"
)

func TestNextWorkMaterializePropagatesCommandContext(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	binDir := t.TempDir()
	tracePath := filepath.Join(t.TempDir(), "tracker.trace")
	stub := `#!/bin/sh
printf 'binary=%s|pwd=%s|beads=%s|verb=%s|arg1=%s\n' "${0##*/}" "$(pwd -P)" "${BEADS_DIR-<unset>}" "$1" "$2" >> "$TRACKER_TRACE"
case "$1" in
  create) printf 'age-materialized\n' ;;
  show) printf 'id: age-materialized\n' ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "br"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("TRACKER_TRACE", tracePath)
	t.Setenv("AGENTOPS_TRACKER", "br")
	t.Setenv("BEADS_DIR", filepath.Join(root, "_beads"))

	entry := rpi.NextWorkEntry{
		SourceEpic: "age-context",
		Items: []rpi.NextWorkItem{{
			Title: "preserve live command context", Type: "task", Severity: "medium",
			Source: "validation", Description: "the tracker child must not outlive Cobra",
		}},
		ClaimStatus: "available",
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	queuePath := filepath.Join(t.TempDir(), "next-work.jsonl")
	if err := os.WriteFile(queuePath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	origFile, origDryRun := nextWorkMaterializeFile, nextWorkMaterializeDryRun
	origJSON, origSourceEpic := nextWorkMaterializeJSON, nextWorkMaterializeSourceEpic
	origMaterializedBy := nextWorkMaterializeMaterialBy
	nextWorkMaterializeFile = queuePath
	nextWorkMaterializeDryRun = false
	nextWorkMaterializeJSON = false
	nextWorkMaterializeSourceEpic = ""
	nextWorkMaterializeMaterialBy = defaultMaterializedBy
	t.Cleanup(func() {
		nextWorkMaterializeFile, nextWorkMaterializeDryRun = origFile, origDryRun
		nextWorkMaterializeJSON, nextWorkMaterializeSourceEpic = origJSON, origSourceEpic
		nextWorkMaterializeMaterialBy = origMaterializedBy
	})

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(canceled)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	_ = runNextWorkMaterialize(cmd, nil)
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Fatalf("pre-canceled command launched tracker: %v", err)
	}

	cmd = &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := runNextWorkMaterialize(cmd, nil); err != nil {
		t.Fatalf("run selected BR materialization: %v", err)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("BR trace lines = %q, want create and show", lines)
	}
	brPrefix := "binary=br|pwd=" + physicalRoot + "|beads=" + filepath.Join(root, "_beads") + "|"
	if lines[0] != brPrefix+"verb=create|arg1=preserve live command context" || lines[1] != brPrefix+"verb=show|arg1=age-materialized" {
		t.Fatalf("BR trace = %q, want canonical create/show", lines)
	}

	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	bdQueue := filepath.Join(t.TempDir(), "next-work.jsonl")
	if err := os.WriteFile(bdQueue, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	nextWorkMaterializeFile = bdQueue
	t.Setenv("AGENTOPS_TRACKER", "bd")
	if err := runNextWorkMaterialize(cmd, nil); err != nil {
		t.Fatalf("run selected BD materialization: %v", err)
	}
	data, err = os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("BD trace lines = %q, want create and show", lines)
	}
	bdPrefix := "binary=bd|pwd=" + physicalRoot + "|beads=<unset>|"
	if lines[0] != bdPrefix+"verb=create|arg1=preserve live command context" || lines[1] != bdPrefix+"verb=show|arg1=age-materialized" {
		t.Fatalf("BD trace = %q, want canonical create/show", lines)
	}

	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	directQueue := filepath.Join(t.TempDir(), "next-work.jsonl")
	if err := os.WriteFile(directQueue, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	nextWorkMaterializeFile = directQueue
	directCmd := &cobra.Command{}
	directCmd.SetOut(&bytes.Buffer{})
	directCmd.SetErr(&bytes.Buffer{})
	if err := runNextWorkMaterialize(directCmd, nil); err != nil {
		t.Fatalf("direct RunE with nil context: %v", err)
	}
	data, err = os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != bdPrefix+"verb=create|arg1=preserve live command context" || lines[1] != bdPrefix+"verb=show|arg1=age-materialized" {
		t.Fatalf("direct RunE trace = %q, want canonical create/show", lines)
	}

	exitStub := "#!/bin/sh\nexit 23\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(exitStub), 0o755); err != nil {
		t.Fatal(err)
	}
	resolution, err := resolveNextWorkMaterializeTracker(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = execNextWorkTracker(context.Background(), resolution, "show", "age-materialized")
	var exitErr *trackerexec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Fatalf("tracker error = %T %v, want *trackerexec.ExitError(23)", err, err)
	}
}

func TestRunNextWorkMaterializeUsesTrackerPort(t *testing.T) {
	origResolve := resolveNextWorkMaterializeTracker
	origExec := execNextWorkTracker
	origFile := nextWorkMaterializeFile
	origDryRun := nextWorkMaterializeDryRun
	origJSON := nextWorkMaterializeJSON
	origSourceEpic := nextWorkMaterializeSourceEpic
	origMaterializedBy := nextWorkMaterializeMaterialBy
	t.Cleanup(func() {
		resolveNextWorkMaterializeTracker = origResolve
		execNextWorkTracker = origExec
		nextWorkMaterializeFile = origFile
		nextWorkMaterializeDryRun = origDryRun
		nextWorkMaterializeJSON = origJSON
		nextWorkMaterializeSourceEpic = origSourceEpic
		nextWorkMaterializeMaterialBy = origMaterializedBy
	})

	item := rpi.NextWorkItem{
		Title:       "Wire materialize through br",
		Type:        "bug",
		Severity:    "high",
		Source:      "post-mortem-finding",
		Description: "The command must not call the retired bd helper.",
	}
	entry := rpi.NextWorkEntry{
		SourceEpic:  "age-6sg",
		Timestamp:   "2026-06-17T12:00:00Z",
		Items:       []rpi.NextWorkItem{item},
		ClaimStatus: "available",
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal queue entry: %v", err)
	}
	queuePath := filepath.Join(t.TempDir(), "next-work.jsonl")
	if err := os.WriteFile(queuePath, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	var calls [][]string
	resolveNextWorkMaterializeTracker = func(string) (trackerResolution, error) {
		return trackerResolution{Tracker: trackerBR, Binary: trackerBR, WorkDir: t.TempDir(), ChildEnv: os.Environ()}, nil
	}
	execNextWorkTracker = func(_ context.Context, _ trackerResolution, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case len(args) > 0 && args[0] == "create":
			return []byte("age-real\n"), nil
		case len(args) > 1 && args[0] == "show" && args[1] == "age-real":
			return []byte("id: age-real\n"), nil
		default:
			return nil, fmt.Errorf("unexpected tracker call: %v", args)
		}
	}

	nextWorkMaterializeFile = queuePath
	nextWorkMaterializeDryRun = false
	nextWorkMaterializeJSON = false
	nextWorkMaterializeSourceEpic = ""
	nextWorkMaterializeMaterialBy = defaultMaterializedBy

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := runNextWorkMaterialize(cmd, nil); err != nil {
		t.Fatalf("run materialize: %v\n%s", err, out.String())
	}
	if !hasNextWorkTrackerCall(calls, "create") || !hasNextWorkTrackerCall(calls, "show") {
		t.Fatalf("expected tracker create and show calls, got %v", calls)
	}
}

func hasNextWorkTrackerCall(calls [][]string, verb string) bool {
	for _, call := range calls {
		if len(call) > 0 && call[0] == verb {
			return true
		}
	}
	return false
}
