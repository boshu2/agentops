package main

import (
	"encoding/json"
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

func TestExecuteDreamMorningPackets_RecordsYieldTelemetry(t *testing.T) {
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

	if summary.Yield == nil {
		t.Fatal("yield telemetry unexpectedly nil")
	}
	if summary.Yield.PacketCountBefore != 0 {
		t.Fatalf("packet_count_before = %d, want 0", summary.Yield.PacketCountBefore)
	}
	if summary.Yield.PacketCountAfter != 1 {
		t.Fatalf("packet_count_after = %d, want 1", summary.Yield.PacketCountAfter)
	}
	if summary.Yield.QueueBackedCount != 1 {
		t.Fatalf("queue_backed_count = %d, want 1", summary.Yield.QueueBackedCount)
	}
	if summary.Yield.SyntheticCount != 0 {
		t.Fatalf("synthetic_count = %d, want 0", summary.Yield.SyntheticCount)
	}
	if summary.Yield.BeadSyncCount != 1 {
		t.Fatalf("bead_sync_count = %d, want 1", summary.Yield.BeadSyncCount)
	}
	if !summary.Yield.QueueBackedWon {
		t.Fatal("queue_backed_won = false, want true")
	}
	if summary.Yield.TopPacketConfidenceAfter != "high" {
		t.Fatalf("top_packet_confidence_after = %q, want high", summary.Yield.TopPacketConfidenceAfter)
	}
	if got := summary.Yield.ConfidenceMix["high"]; got != 1 {
		t.Fatalf("confidence_mix[high] = %d, want 1", got)
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

func TestExecuteDreamMorningPackets_AppliesPacketCorroboration(t *testing.T) {
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

	goal := "stabilize Dream handoff"
	summary := newDreamPacketTestSummary(t, tmpDir, goal)
	summary.packetCorroboration = map[string]dreamPacketCorroboration{
		dreamPacketID("goal", goal): {
			Confidence:  "high",
			Evidence:    []string{"Briefing available: fallback", "Retrieval coverage stayed healthy at 0.90"},
			TargetFiles: []string{"briefing-fallback.json"},
		},
	}

	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.MorningPackets) != 1 {
		t.Fatalf("morning packets = %d, want 1", len(summary.MorningPackets))
	}
	packet := summary.MorningPackets[0]
	if packet.Confidence != "high" {
		t.Fatalf("confidence = %q, want high", packet.Confidence)
	}
	if !strings.Contains(strings.Join(packet.Evidence, "\n"), "Briefing available: fallback") {
		t.Fatalf("evidence = %#v, want corroboration marker", packet.Evidence)
	}
	if !strings.Contains(strings.Join(packet.TargetFiles, "\n"), "briefing-fallback.json") {
		t.Fatalf("target_files = %#v, want corroboration target", packet.TargetFiles)
	}
}

// TestExecuteDreamMorningPackets_SuppressesPacketWhenTargetFileExists guards
// the 2026-04-26 retro fix: the curator must run a tractability probe before
// emitting a packet whose first_move grep would match an existing tracked
// file. The synthetic queue item below points at cli/cmd/ao/overnight.go,
// which is shipped — so the curator should suppress instead of emit and
// surface a dream-curator-suppressed entry.
func TestExecuteDreamMorningPackets_SuppressesPacketWhenTargetFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Drop a sentinel file inside the tmp repo and have the queue item
	// cite it as target_files. Probe should hit, packet should be suppressed.
	relTarget := "cli/cmd/ao/overnight.go"
	if err := os.MkdirAll(filepath.Join(tmpDir, "cli", "cmd", "ao"), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, relTarget), []byte("// already shipped\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	queue := `{"source_epic":"dream-stale","timestamp":"2026-04-26T12:00:00Z","items":[{"id":"stale-1","title":"Ship overnight.go feature already done","type":"task","severity":"high","source":"council-finding","description":"Cited target file already exists.","target_files":["cli/cmd/ao/overnight.go"],"consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available"}`
	if err := os.WriteFile(nextWorkPath, []byte(queue+"\n"), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	binDir := t.TempDir()
	writeExecutable(t, binDir, "bd", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir)

	summary := newDreamPacketTestSummary(t, tmpDir, "")
	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.MorningPackets) != 0 {
		t.Fatalf("expected suppression, got %d packets: %+v", len(summary.MorningPackets), summary.MorningPackets)
	}
	if len(summary.CuratorSuppressed) != 1 {
		t.Fatalf("expected 1 suppression, got %d: %+v", len(summary.CuratorSuppressed), summary.CuratorSuppressed)
	}
	got := summary.CuratorSuppressed[0]
	if got.PacketID != "stale-1" {
		t.Errorf("suppression packet_id = %q, want stale-1", got.PacketID)
	}
	if got.Match != "cli/cmd/ao/overnight.go" {
		t.Errorf("suppression match = %q, want cli/cmd/ao/overnight.go", got.Match)
	}
	if got.Reason == "" {
		t.Error("suppression reason should be non-empty")
	}
}

func TestExecuteDreamMorningPackets_PersistsProbeResultsAndDegradesOnStaleRate(t *testing.T) {
	tmpDir := t.TempDir()
	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rel := range []string{
		"cli/cmd/ao/already-one.go",
		"cli/cmd/ao/already-two.go",
		"cli/cmd/ao/already-three.go",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, rel)), 0o755); err != nil {
			t.Fatalf("mkdir target %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, rel), []byte("// shipped\n"), 0o644); err != nil {
			t.Fatalf("write target %s: %v", rel, err)
		}
	}

	queue := strings.Join([]string{
		`{"id":"stale-1","title":"Ship stale one","type":"task","severity":"high","source":"council-finding","description":"Already exists.","target_files":["cli/cmd/ao/already-one.go"],"consumed":false,"claim_status":"available"}`,
		`{"id":"stale-2","title":"Ship stale two","type":"task","severity":"high","source":"council-finding","description":"Already exists.","target_files":["cli/cmd/ao/already-two.go"],"consumed":false,"claim_status":"available"}`,
		`{"id":"stale-3","title":"Ship stale three","type":"task","severity":"high","source":"council-finding","description":"Already exists.","target_files":["cli/cmd/ao/already-three.go"],"consumed":false,"claim_status":"available"}`,
	}, ",")
	if err := os.WriteFile(nextWorkPath, []byte(`{"source_epic":"dream-stale","timestamp":"2026-04-26T12:00:00Z","items":[`+queue+`],"consumed":false,"claim_status":"available"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	binDir := t.TempDir()
	writeExecutable(t, binDir, "bd", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir)

	summary := newDreamPacketTestSummary(t, tmpDir, "")
	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.CuratorSuppressed) != 3 {
		t.Fatalf("expected 3 suppressions, got %d: %+v", len(summary.CuratorSuppressed), summary.CuratorSuppressed)
	}
	if !strings.Contains(strings.Join(summary.Degraded, "\n"), "dream-curator-degraded") {
		t.Fatalf("summary degraded = %v, want dream-curator-degraded finding", summary.Degraded)
	}

	probePath := filepath.Join(tmpDir, ".agents", "dream", "probe-results.jsonl")
	data, err := os.ReadFile(probePath)
	if err != nil {
		t.Fatalf("read probe results: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("probe results lines = %d, want 3\n%s", len(lines), data)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("unmarshal probe result: %v", err)
	}
	if record["packet_id"] != "stale-1" || record["stale"] != true || record["reason"] == "" || record["match"] == "" {
		t.Fatalf("unexpected first probe result: %#v", record)
	}
}

func TestBuildDreamMorningPacketPlans_CapsProbeResults(t *testing.T) {
	tmpDir := t.TempDir()
	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rel := range []string{
		"cli/cmd/ao/stale-one.go",
		"cli/cmd/ao/stale-two.go",
		"cli/cmd/ao/stale-three.go",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, rel)), 0o755); err != nil {
			t.Fatalf("mkdir target %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, rel), []byte("// shipped\n"), 0o644); err != nil {
			t.Fatalf("write target %s: %v", rel, err)
		}
	}
	queue := strings.Join([]string{
		`{"id":"stale-1","title":"Ship stale one","type":"task","severity":"high","source":"council-finding","target_files":["cli/cmd/ao/stale-one.go"],"consumed":false,"claim_status":"available"}`,
		`{"id":"stale-2","title":"Ship stale two","type":"task","severity":"high","source":"council-finding","target_files":["cli/cmd/ao/stale-two.go"],"consumed":false,"claim_status":"available"}`,
		`{"id":"stale-3","title":"Ship stale three","type":"task","severity":"high","source":"council-finding","target_files":["cli/cmd/ao/stale-three.go"],"consumed":false,"claim_status":"available"}`,
	}, ",")
	if err := os.WriteFile(nextWorkPath, []byte(`{"source_epic":"dream-stale","timestamp":"2026-04-26T12:00:00Z","items":[`+queue+`],"consumed":false,"claim_status":"available"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	summary := newDreamPacketTestSummary(t, tmpDir, "advance goal")
	summary.RetrievalLive = map[string]any{"coverage": 0.1}
	summary.MetricsHealth = map[string]any{"escape_velocity": false}
	summary.Degraded = []string{"retrieval failed"}
	for _, key := range []string{"retrieval_live", "metrics_health", "summary_json"} {
		path := summary.Artifacts[key]
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir artifact %s: %v", key, err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write artifact %s: %v", key, err)
		}
	}

	_, _, probeResults, err := buildDreamMorningPacketPlans(tmpDir, summary)
	if err != nil {
		t.Fatalf("buildDreamMorningPacketPlans: %v", err)
	}
	if len(probeResults) != maxDreamPacketProbeResults {
		t.Fatalf("probe result count = %d, want capped count %d", len(probeResults), maxDreamPacketProbeResults)
	}
	for _, result := range probeResults {
		if result.Source == "dream-degraded" {
			t.Fatalf("dream-degraded fallback was probed after cap: %+v", result)
		}
	}
}

// TestExecuteDreamMorningPackets_EmitsWhenProbeInconclusive verifies that
// the probe does NOT suppress when no cited surface resolves on disk —
// inconclusive evidence should let the packet through normally.
func TestExecuteDreamMorningPackets_EmitsWhenProbeInconclusive(t *testing.T) {
	tmpDir := t.TempDir()
	nextWorkPath := filepath.Join(tmpDir, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queue := `{"source_epic":"dream-fresh","timestamp":"2026-04-26T12:00:00Z","items":[{"id":"fresh-1","title":"Implement brand new module","type":"feature","severity":"high","source":"council-finding","description":"No file cited yet.","target_files":["cli/internal/totally-new-package/foo.go"],"consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available"}`
	if err := os.WriteFile(nextWorkPath, []byte(queue+"\n"), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	binDir := t.TempDir()
	writeExecutable(t, binDir, "bd", `#!/bin/sh
case "$1" in
  list) echo '[]' ;;
  create) echo '[{"id":"na-fresh","status":"open","title":"Implement brand new module"}]' ;;
  update) echo '[{"id":"na-fresh","status":"open","title":"Implement brand new module"}]' ;;
  *) echo "unexpected" >&2; exit 1 ;;
esac
`)
	t.Setenv("PATH", binDir)

	summary := newDreamPacketTestSummary(t, tmpDir, "")
	executeDreamMorningPackets(tmpDir, &summary)

	if len(summary.MorningPackets) != 1 {
		t.Fatalf("expected 1 packet to emit, got %d", len(summary.MorningPackets))
	}
	if len(summary.CuratorSuppressed) != 0 {
		t.Fatalf("expected 0 suppressions, got %+v", summary.CuratorSuppressed)
	}
}

// TestProbeDreamPacketStaleness_SchemaRef checks that schemas/<x>.json
// references in the morning command get the same staleness treatment as
// scripts/<x>.sh references — both are repo paths whose existence
// indicates the curator is suggesting work that's already shipped.
func TestProbeDreamPacketStaleness_SchemaRef(t *testing.T) {
	tmpDir := t.TempDir()
	schemaRel := filepath.Join("schemas", "next-work.v1.4.json")
	if err := os.MkdirAll(filepath.Join(tmpDir, "schemas"), 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, schemaRel), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	packet := overnightMorningPacket{
		ID:             "schema-stale",
		Title:          "Add domain enum to schemas/next-work.v1.4.json",
		MorningCommand: `ao rpi phased "Add domain enum to schemas/next-work.v1.4.json"`,
	}
	reason, match, ok := probeDreamPacketStaleness(tmpDir, packet)
	if !ok {
		t.Fatalf("expected probe to flag stale schema ref; got reason=%q match=%q", reason, match)
	}
	if !strings.Contains(reason, "schemas/") {
		t.Errorf("reason = %q, want it to mention schemas/", reason)
	}
	if match != "schemas/next-work.v1.4.json" {
		t.Errorf("match = %q, want schemas/next-work.v1.4.json", match)
	}
}

// TestProbeDreamPacketStaleness_SkillLineLimitClaim verifies the curator
// rejects "Decompose skills/<name>/SKILL.md to under N-line limit" packets
// when the cited skill exists and is below the canonical fail threshold
// (warn>500 fail>800 from scripts/check-skill-size.sh). This is the
// 2026-04-30 dream-curator-degraded finding pattern.
func TestProbeDreamPacketStaleness_SkillLineLimitClaim(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "crank")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	// Write a 400-line SKILL.md — well below the 800 fail threshold.
	body := strings.Repeat("line of skill content\n", 400)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	packet := overnightMorningPacket{
		ID:    "fictional-line-limit",
		Title: "Decompose skills/crank/SKILL.md to under 248-line limit",
	}
	reason, match, ok := probeDreamPacketStaleness(tmpDir, packet)
	if !ok {
		t.Fatalf("expected probe to flag fictional line-limit claim; got reason=%q match=%q", reason, match)
	}
	if !strings.Contains(reason, "fail threshold") {
		t.Errorf("reason = %q, want it to mention fail threshold", reason)
	}
	if match != "skills/crank/SKILL.md" {
		t.Errorf("match = %q, want skills/crank/SKILL.md", match)
	}
}

// TestProbeDreamPacketStaleness_SkillRefWithoutLineLimit verifies the
// probe does NOT flag a packet that mentions skills/<name>/SKILL.md but
// has no numeric line-limit phrase — modifying an existing skill is
// legitimate work, only the fictional-line-limit pattern is stale.
func TestProbeDreamPacketStaleness_SkillRefWithoutLineLimit(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "crank")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# crank\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	packet := overnightMorningPacket{
		ID:    "legit-skill-edit",
		Title: "Add a section to skills/crank/SKILL.md describing wave-6 enrichment",
	}
	if reason, match, ok := probeDreamPacketStaleness(tmpDir, packet); ok {
		t.Fatalf("probe should NOT flag this packet: reason=%q match=%q", reason, match)
	}
}

// TestProbeDreamPacketStaleness_AddToSkillClaim_AlreadyImplemented covers
// the 2026-05-02 stale-packet shape: "Add binary-deployment gate to
// /implement skill" while skills/implement/SKILL.md already documents
// the gate. The probe should mark the packet stale because the cited
// phrase already appears verbatim (modulo case/hyphen normalization)
// in the SKILL.md.
func TestProbeDreamPacketStaleness_AddToSkillClaim_AlreadyImplemented(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "implement")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	body := "# /implement\n\n## Step 5.5: Binary-Deployment Gate (CLI/Hook Bug Fixes) — MANDATORY\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	packet := overnightMorningPacket{
		ID:    "add-to-skill-already-done",
		Title: "Add binary-deployment gate to /implement skill",
	}
	reason, match, ok := probeDreamPacketStaleness(tmpDir, packet)
	if !ok {
		t.Fatalf("expected probe to flag stale add-to-skill claim; got reason=%q match=%q", reason, match)
	}
	if match != "skills/implement/SKILL.md" {
		t.Errorf("match = %q, want skills/implement/SKILL.md", match)
	}
	if !strings.Contains(reason, "already implements") {
		t.Errorf("reason = %q, want it to mention 'already implements'", reason)
	}
}

// TestProbeDreamPacketStaleness_AddToSkillClaim_NewAddition confirms the
// probe does NOT flag a packet whose phrase is genuinely missing from
// the cited skill's SKILL.md — that's legitimate work, not staleness.
func TestProbeDreamPacketStaleness_AddToSkillClaim_NewAddition(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "implement")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# /implement\n\nplain content\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	packet := overnightMorningPacket{
		ID:    "add-to-skill-new",
		Title: "Add binary-deployment gate to /implement skill",
	}
	if reason, match, ok := probeDreamPacketStaleness(tmpDir, packet); ok {
		t.Fatalf("probe should NOT flag a genuinely missing addition: reason=%q match=%q", reason, match)
	}
}

// TestProbeDreamPacketStaleness_AddToSkillClaim_NonAddVerb confirms the
// probe rejects titles that don't start with "Add ..." — the
// "phrase-already-in-SKILL.md" heuristic is only safe for additions, not
// for refactor/fix/wire/implement verbs.
func TestProbeDreamPacketStaleness_AddToSkillClaim_NonAddVerb(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "implement")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	body := "# /implement\n\n## Binary-Deployment Gate\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	for _, title := range []string{
		"Refactor binary-deployment gate in /implement skill",
		"Fix binary-deployment gate in /implement skill",
		"Wire binary-deployment gate to /implement skill",
	} {
		t.Run(title, func(t *testing.T) {
			packet := overnightMorningPacket{ID: "non-add-verb", Title: title}
			if reason, match, ok := probeDreamPacketStaleness(tmpDir, packet); ok {
				t.Fatalf("probe should NOT flag non-Add verbs: reason=%q match=%q", reason, match)
			}
		})
	}
}

// TestExtractAddToSkillClaim_ParseShapes pins the title parser surface so
// the probe can rely on it.
func TestExtractAddToSkillClaim_ParseShapes(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantPath   string
		wantPhrase string
		wantOK     bool
	}{
		{
			"canonical",
			"Add binary-deployment gate to /implement skill",
			"skills/implement/SKILL.md", "binary-deployment gate", true,
		},
		{
			"with-the",
			"Add closure proof to the /post-mortem skill",
			"skills/post-mortem/SKILL.md", "closure proof", true,
		},
		{
			"non-add-verb",
			"Wire foo to /bar skill",
			"", "", false,
		},
		{
			"missing-skill-suffix",
			"Add foo to /bar",
			"", "", false,
		},
		{
			"slash-only",
			"Add foo to /",
			"", "", false,
		},
		{
			"empty",
			"",
			"", "", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotPhrase, gotOK := extractAddToSkillClaim(tc.in)
			if gotOK != tc.wantOK || gotPath != tc.wantPath || gotPhrase != tc.wantPhrase {
				t.Errorf("extractAddToSkillClaim(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, gotPath, gotPhrase, gotOK, tc.wantPath, tc.wantPhrase, tc.wantOK)
			}
		})
	}
}

// TestSkillContainsAddPhrase_Normalization pins the matcher's normalization
// rules: case-insensitive, hyphens/underscores treated as spaces, repeated
// whitespace collapsed.
func TestSkillContainsAddPhrase_Normalization(t *testing.T) {
	cases := []struct {
		name    string
		content string
		phrase  string
		want    bool
	}{
		{"exact", "binary-deployment gate", "binary-deployment gate", true},
		{"case", "Binary-Deployment Gate", "binary-deployment gate", true},
		{"hyphen-vs-space", "binary deployment gate", "binary-deployment gate", true},
		{"underscore-vs-space", "binary_deployment gate", "binary-deployment gate", true},
		{"missing", "deployment only", "binary-deployment gate", false},
		{"partial-token", "binary thing only", "binary-deployment gate", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillContainsAddPhrase(tc.content, tc.phrase); got != tc.want {
				t.Errorf("skillContainsAddPhrase(%q, %q) = %v, want %v", tc.content, tc.phrase, got, tc.want)
			}
		})
	}
}

// TestExtractRepoRef_TerminationCharacters confirms the helper terminates
// on whitespace, quotes, parens, and backticks but not on alphanumerics —
// the same surface area extractScriptsRef previously covered, now
// generalized.
func TestExtractRepoRef_TerminationCharacters(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		prefix string
		suffix string
		want   string
	}{
		{"plain", "see scripts/foo.sh now", "scripts/", ".sh", "scripts/foo.sh"},
		{"quoted", `run "scripts/bar.sh" later`, "scripts/", ".sh", "scripts/bar.sh"},
		{"paren-trail", "ao rpi phased(scripts/baz.sh)", "scripts/", ".sh", "scripts/baz.sh"},
		{"missing-suffix", "see scripts/foo.txt", "scripts/", ".sh", ""},
		{"no-match", "no path here", "scripts/", ".sh", ""},
		{"schema", "see schemas/foo.json", "schemas/", ".json", "schemas/foo.json"},
		{"empty-prefix", "anything", "", ".sh", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRepoRef(tc.in, tc.prefix, tc.suffix)
			if got != tc.want {
				t.Errorf("extractRepoRef(%q,%q,%q) = %q, want %q", tc.in, tc.prefix, tc.suffix, got, tc.want)
			}
		})
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
