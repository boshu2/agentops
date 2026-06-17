package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

func TestRunNextWorkMaterializeUsesBRTrackerPort(t *testing.T) {
	origAvailable := nextWorkTrackerAvailable
	origExec := execNextWorkTracker
	origFile := nextWorkMaterializeFile
	origDryRun := nextWorkMaterializeDryRun
	origJSON := nextWorkMaterializeJSON
	origSourceEpic := nextWorkMaterializeSourceEpic
	origMaterializedBy := nextWorkMaterializeMaterialBy
	t.Cleanup(func() {
		nextWorkTrackerAvailable = origAvailable
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
	nextWorkTrackerAvailable = func() bool { return true }
	execNextWorkTracker = func(args ...string) ([]byte, error) {
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
		t.Fatalf("expected br create and br show calls, got %v", calls)
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
