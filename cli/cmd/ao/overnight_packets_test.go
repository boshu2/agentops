package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteDreamMorningPackets_AnnotatesQueueAndCreatesBead(t *testing.T) {
	tmpDir := t.TempDir()
	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); err != nil {
		t.Fatalf("mkdir next-work dir: %v", err)
	}
	queue := `{"source_epic":"dream-findings-router","timestamp":"2026-04-14T12:00:00Z","items":[{"title":"Repair Dream packet ranking","type":"bug","severity":"high","source":"finding-router","description":"Queue-backed packet should become actionable morning work.","evidence":"packet evidence","source_path":"cli/cmd/ao/overnight.go","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available"}`
	if err := os.WriteFile(nextWorkPath, []byte(queue+"\n"), 0o644); err != nil {
		t.Fatalf("write next-work: %v", err)
	}

	binDir := t.TempDir()
	writeExecutable(t, binDir, "bd", `#!/bin/sh
case "$1" in
  list)
    echo '[]'
    ;;
  create)
    echo '[{"id":"na-pkt1","status":"open","title":"Repair Dream packet ranking"}]'
    ;;
  update)
    echo '[{"id":"na-pkt1","status":"open","title":"Repair Dream packet ranking"}]'
    ;;
  *)
    echo "unexpected bd command: $1" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("PATH", binDir)

	summary := newDreamPacketTestSummary(t, tmpDir, "")
	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.MorningPackets) != 1 {
		t.Fatalf("morning packets = %d, want 1", len(summary.MorningPackets))
	}
	packet := summary.MorningPackets[0]
	if packet.BeadID != "na-pkt1" {
		t.Fatalf("packet bead_id = %q, want na-pkt1", packet.BeadID)
	}
	if packet.ArtifactPath == "" {
		t.Fatal("packet artifact path is empty")
	}
	if !strings.Contains(renderOvernightSummaryMarkdown(summary), "Morning Packets") {
		t.Fatal("rendered summary missing Morning Packets section")
	}

	entries, err := readQueueEntries(nextWorkPath)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	if len(entries) != 1 || len(entries[0].Items) != 1 {
		t.Fatalf("queue entries = %+v, want 1 item", entries)
	}
	item := entries[0].Items[0]
	if item.BeadID != "na-pkt1" {
		t.Fatalf("queue bead_id = %q, want na-pkt1", item.BeadID)
	}
	if item.PacketPath == "" {
		t.Fatal("queue packet_path is empty")
	}
	if item.MorningCmd == "" {
		t.Fatal("queue morning_command is empty")
	}
	if item.ID == "" {
		t.Fatal("queue item id is empty")
	}

	if _, err := os.Stat(summary.Artifacts["morning_packets_json"]); err != nil {
		t.Fatalf("missing morning packet json artifact: %v", err)
	}
	if _, err := os.Stat(summary.Artifacts["morning_packets_markdown"]); err != nil {
		t.Fatalf("missing morning packet markdown artifact: %v", err)
	}
}

func TestExecuteDreamMorningPackets_SynthesizesFallbackQueueItem(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := t.TempDir()
	writeExecutable(t, binDir, "bd", `#!/bin/sh
case "$1" in
  list)
    echo '[]'
    ;;
  create)
    echo '[{"id":"na-goal1","status":"open","title":"Advance overnight goal"}]'
    ;;
  update)
    echo '[{"id":"na-goal1","status":"open","title":"Advance overnight goal"}]'
    ;;
  *)
    echo "unexpected bd command: $1" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("PATH", binDir)

	summary := newDreamPacketTestSummary(t, tmpDir, "stabilize Dream handoff")
	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.MorningPackets) == 0 {
		t.Fatal("expected fallback morning packet")
	}
	packet := summary.MorningPackets[0]
	if !strings.Contains(packet.Title, "stabilize Dream handoff") {
		t.Fatalf("packet title = %q, want goal text", packet.Title)
	}
	if packet.BeadID != "na-goal1" {
		t.Fatalf("packet bead_id = %q, want na-goal1", packet.BeadID)
	}

	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	entries, err := readQueueEntries(nextWorkPath)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	if len(entries) != 1 || len(entries[0].Items) != 1 {
		t.Fatalf("queue entries = %+v, want synthetic fallback item", entries)
	}
	item := entries[0].Items[0]
	if item.Source != "dream-goal" {
		t.Fatalf("queue source = %q, want dream-goal", item.Source)
	}
	if item.BeadID != "na-goal1" {
		t.Fatalf("queue bead_id = %q, want na-goal1", item.BeadID)
	}
	if item.MorningCmd == "" {
		t.Fatal("synthetic fallback missing morning command")
	}
}

func TestShouldEscalateDreamDegradation(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "recovery: cleaned up stale DONE marker", want: false},
		{value: "knowledge-brief: knowledge brief requires topic packets under /tmp/.agents/topics", want: false},
		{value: "claude council run failed: timeout waiting for runner", want: true},
		{value: "metrics-health: retrieval endpoint unreachable", want: true},
	}

	for _, tt := range tests {
		if got := shouldEscalateDreamDegradation(tt.value); got != tt.want {
			t.Fatalf("shouldEscalateDreamDegradation(%q) = %t, want %t", tt.value, got, tt.want)
		}
	}
}

func TestShouldSkipDreamQueueSelection(t *testing.T) {
	if !shouldSkipDreamQueueSelection(nextWorkItem{
		Title:  "Investigate Dream degradation: recovery: cleaned up stale DONE marker",
		Source: "dream-degraded",
	}) {
		t.Fatal("expected benign recovery degradation packet to be skipped")
	}
	if shouldSkipDreamQueueSelection(nextWorkItem{
		Title:  "Investigate Dream degradation: claude council run failed: timeout waiting for runner",
		Source: "dream-degraded",
	}) {
		t.Fatal("expected actionable degradation packet to remain selectable")
	}
}

func TestBuildDreamQueuePacket_PreservesExistingPacketIdentity(t *testing.T) {
	summary := overnightSummary{}
	sel := queueSelection{
		Item: nextWorkItem{
			ID:          "dream-existing-id",
			Title:       "Advance overnight goal: validate Dream morning packet handoff",
			Type:        "task",
			Severity:    "high",
			Source:      "dream-goal",
			Description: "Queue-backed goal packet",
			Evidence:    "goal evidence",
			Confidence:  "medium",
			WhyNow:      "Preserve prior packet context.",
			MorningCmd:  `ao rpi phased "validate Dream morning packet handoff"`,
		},
		SourceEpic: "dream-goal",
	}

	packet := buildDreamQueuePacket(summary, sel, 1)
	if packet.ID != "dream-existing-id" {
		t.Fatalf("packet id = %q, want existing id", packet.ID)
	}
	if packet.MorningCommand != `ao rpi phased "validate Dream morning packet handoff"` {
		t.Fatalf("packet morning_command = %q", packet.MorningCommand)
	}
	if packet.WhyNow != "Preserve prior packet context." {
		t.Fatalf("packet why_now = %q", packet.WhyNow)
	}
}

func newDreamPacketTestSummary(t *testing.T, repoRoot, goal string) overnightSummary {
	t.Helper()

	oldGoal := overnightGoal
	overnightGoal = goal
	t.Cleanup(func() { overnightGoal = oldGoal })

	settings := overnightSettings{
		OutputDir:  filepath.Join(repoRoot, ".agents", "overnight", "dream-packet-test"),
		RunTimeout: time.Hour,
	}
	return newOvernightStartSummary(repoRoot, settings, time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC))
}
