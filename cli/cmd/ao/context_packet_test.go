package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextPacketCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "rpi"),
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	finding := `---
id: "f-packet-001"
title: "Test packet finding"
status: "active"
applicable_when: ["task"]
scope_tags: ["cli","packet"]
---
# Finding

Test finding for packet inspection.
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "findings", "f-packet-001.md"), []byte(finding), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := `---
id: "f-packet-001"
---
# Planning Rule

- Verify packet ranking logic before planning.
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "planning-rules", "f-packet-001.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	queue := `{"source_epic":"ag-test","timestamp":"2026-03-28T12:00:00Z","items":[{"title":"Packet CLI work","type":"task","severity":"high","source":"council-finding","description":"Add packet inspection","target_repo":"agentops","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so the command picks it up.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"context", "packet", "--json", "--goal", "packet cli inspection", "--epic", "ag-test", "--repo", "agentops"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, buf.String())
	}

	var packet StigmergicPacket
	if err := json.Unmarshal(buf.Bytes(), &packet); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}

	if packet.Scorecard.PromotedFindings != 1 {
		t.Errorf("Scorecard.PromotedFindings = %d, want 1", packet.Scorecard.PromotedFindings)
	}
	if packet.Scorecard.QueueEntries != 1 {
		t.Errorf("Scorecard.QueueEntries = %d, want 1", packet.Scorecard.QueueEntries)
	}
	if packet.Scorecard.HighSeverityUnconsumed != 1 {
		t.Errorf("Scorecard.HighSeverityUnconsumed = %d, want 1", packet.Scorecard.HighSeverityUnconsumed)
	}

	if len(packet.AppliedFindings) != 1 || packet.AppliedFindings[0] != "f-packet-001" {
		t.Errorf("AppliedFindings = %v, want [f-packet-001]", packet.AppliedFindings)
	}
	if len(packet.PlanningRules) != 1 {
		t.Errorf("PlanningRules length = %d, want 1", len(packet.PlanningRules))
	}
	if len(packet.PriorFindings) != 1 || packet.PriorFindings[0].Title != "Packet CLI work" {
		t.Errorf("PriorFindings = %+v, want matched queue item", packet.PriorFindings)
	}
}

func TestContextPacketCmd_HumanOutput(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "rpi"),
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Reset package-level flag state from prior test runs.
	contextPacketFlags.json = false
	contextPacketFlags.goal = ""
	contextPacketFlags.epic = ""
	contextPacketFlags.repo = ""
	contextPacketFlags.limit = defaultStigmergicPacketLimit
	cmd.SetArgs([]string{"context", "packet"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, buf.String())
	}

	output := buf.String()
	for _, section := range []string{"## Scorecard", "## Applied Findings", "## Planning Rules", "## Known Risks", "## Matched Next-Work"} {
		if !strings.Contains(output, section) {
			t.Errorf("output missing section %q", section)
		}
	}
}

func TestDetectRepoName_ReturnsBaseDirWithoutGit(t *testing.T) {
	dir := t.TempDir()
	name := detectRepoName(dir)
	base := filepath.Base(dir)
	if name != base {
		t.Errorf("detectRepoName(%q) = %q, want %q", dir, name, base)
	}
}
