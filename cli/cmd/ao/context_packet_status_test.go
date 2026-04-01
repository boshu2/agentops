package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

func TestBuildPacketStatusResult_ExperimentalByDefault(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{
		filepath.Join(tmp, ".agents", "topics"),
		filepath.Join(tmp, ".agents", "packets", "source-manifests"),
		filepath.Join(tmp, ".agents", "packets", "promoted"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "packets", "promoted", "alpha.md"), []byte("# Alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCitations(tmp, []types.CitationEvent{
		{ArtifactPath: filepath.Join(tmp, ".agents", "packets", "promoted", "alpha.md"), WorkspacePath: tmp, SessionID: "s1", CitedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	result := buildPacketStatusResult(tmp, "startup context", "startup", rankedContextBundle{
		CWD:   tmp,
		Query: "startup context",
	})

	if got, want := result.Rollout.Stage, "experimental"; got != want {
		t.Fatalf("Rollout.Stage = %q, want %q", got, want)
	}
	if len(result.Families) != 3 {
		t.Fatalf("expected 3 packet families, got %d", len(result.Families))
	}
}

func TestContextPacketStatusJSONOutput(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	contextPacketStatusFlags.task = ""
	contextPacketStatusFlags.phase = "startup"
	contextPacketStatusFlags.limit = defaultStigmergicPacketLimit

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runContextPacketStatus(cmd, nil); err != nil {
		t.Fatalf("runContextPacketStatus: %v", err)
	}

	var result packetStatusResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, out.String())
	}
	if result.Rollout.Stage == "" {
		t.Fatalf("expected rollout stage in JSON output")
	}
}

func TestPrintPacketStatusHuman(t *testing.T) {
	cmd := rootCmd
	var sb strings.Builder
	cmd.SetOut(&sb)
	printPacketStatusHuman(cmd, packetStatusResult{
		Query:    "startup",
		Phase:    "startup",
		Payload:  contextExplainPayloadHealth{Status: "thin", SelectedCount: 2, Reason: "thin"},
		Rollout:  packetRolloutStatus{Stage: "experimental", Guidance: "keep gated"},
		Families: []packetFamilyStatus{{Family: "promoted-packets", Status: "manual_review", Count: 1, Reason: "thin"}},
		Metrics:  packetRolloutMetrics{ThinFamilies: 1, TotalFamilies: 3, ThinRatio: 0.33},
	})
	if !strings.Contains(sb.String(), "## Packet Status") {
		t.Fatalf("expected packet status heading, got:\n%s", sb.String())
	}
}
