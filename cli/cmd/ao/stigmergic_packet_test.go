// practices: [ai-assisted-dev, team-topologies]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleStigmergicPacket_RanksCompiledSignalsAndQueueItems(t *testing.T) {
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
id: "f-2026-03-11-001"
title: "Rank next-work before planning"
status: "active"
applicable_when: ["task","plan-shape"]
scope_tags: ["workflow","next-work"]
---
# Finding

Improve next-work ranking and planning packet relevance.
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "findings", "f-2026-03-11-001.md"), []byte(finding), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := `---
id: "f-2026-03-11-001"
---
# Planning Rule

- Ask: Did the plan rank next-work items before decomposition?
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "planning-rules", "f-2026-03-11-001.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	check := `---
id: "f-2026-03-11-001"
---
# Pre-Mortem Check

- Ask: Did this review load ranked prior findings?
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "pre-mortem-checks", "f-2026-03-11-001.md"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}

	queue := `{"source_epic":"ag-h83","timestamp":"2026-03-11T17:00:00Z","items":[{"title":"Rank next-work backlog in discovery","type":"task","severity":"high","source":"council-finding","description":"Improve planning packet and next-work ranking","target_repo":"agentops","consumed":false,"claim_status":"available"},{"title":"Unrelated release work","type":"chore","severity":"low","source":"retro-learning","description":"release polish","target_repo":"other-repo","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	packet, err := assembleStigmergicPacket(dir, StigmergicTarget{
		GoalText:   "rank next-work backlog before planning",
		IssueType:  "task",
		Files:      []string{"skills/plan/SKILL.md", "cli/cmd/ao/rpi_loop.go"},
		ActiveEpic: "ag-h83",
		Repo:       "agentops",
	})
	if err != nil {
		t.Fatalf("assembleStigmergicPacket: %v", err)
	}

	if len(packet.AppliedFindings) != 1 || packet.AppliedFindings[0] != "f-2026-03-11-001" {
		t.Fatalf("AppliedFindings = %v, want ranked finding", packet.AppliedFindings)
	}
	if len(packet.PlanningRules) != 1 || packet.PlanningRules[0] == "" {
		t.Fatalf("PlanningRules = %v, want compiled planning summary", packet.PlanningRules)
	}
	if len(packet.KnownRisks) != 1 || packet.KnownRisks[0] == "" {
		t.Fatalf("KnownRisks = %v, want compiled pre-mortem summary", packet.KnownRisks)
	}
	if len(packet.PriorFindings) != 1 || packet.PriorFindings[0].Title != "Rank next-work backlog in discovery" {
		t.Fatalf("PriorFindings = %+v, want matching queue item", packet.PriorFindings)
	}
}

func TestLoadStigmergicScorecard_CountsCompiledAndQueueState(t *testing.T) {
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

	for _, rel := range []string{
		filepath.Join(".agents", "findings", "f-1.md"),
		filepath.Join(".agents", "planning-rules", "f-1.md"),
		filepath.Join(".agents", "pre-mortem-checks", "f-1.md"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	queue := `{"source_epic":"ag-h83","timestamp":"2026-03-11T17:00:00Z","items":[{"title":"High one","type":"task","severity":"high","source":"council-finding","description":"d1","target_repo":"agentops","consumed":false},{"title":"Low one","type":"task","severity":"low","source":"retro-learning","description":"d2","target_repo":"agentops","consumed":false}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
{"source_epic":"ag-old","timestamp":"2026-03-10T17:00:00Z","items":[{"title":"Claimed","type":"task","severity":"high","source":"council-finding","description":"claimed","target_repo":"agentops","consumed":false,"claim_status":"in_progress"}],"consumed":false,"claim_status":"in_progress","claimed_by":"worker","claimed_at":"2026-03-10T17:05:00Z","consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	scorecard, err := loadStigmergicScorecard(dir)
	if err != nil {
		t.Fatalf("loadStigmergicScorecard: %v", err)
	}

	if scorecard.PromotedFindings != 1 || scorecard.PlanningRules != 1 || scorecard.PreMortemChecks != 1 {
		t.Fatalf("scorecard compiled counts = %+v, want 1/1/1", scorecard)
	}
	if scorecard.QueueEntries != 1 || scorecard.UnconsumedBatches != 1 {
		t.Fatalf("queue entry counts = %+v, want one selectable batch", scorecard)
	}
	if scorecard.UnconsumedItems != 2 {
		t.Fatalf("UnconsumedItems = %d, want 2", scorecard.UnconsumedItems)
	}
	if scorecard.HighSeverityUnconsumed != 1 {
		t.Fatalf("HighSeverityUnconsumed = %d, want 1", scorecard.HighSeverityUnconsumed)
	}
}

func TestLoadStigmergicScorecard_ExcludesProofBackedSuppressedRows(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "rpi"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeCompletedLoopRegistryRun(t, dir, "run-proof", "ag-proof", "Already done backlog item")

	queue := `{"source_epic":"ag-proof","timestamp":"2026-03-11T17:00:00Z","items":[{"title":"Already done backlog item","type":"task","severity":"high","source":"council-finding","description":"proof-backed stale work","target_repo":"` + detectRepoName(dir) + `","proof_ref":{"kind":"completed_run","run_id":"run-proof"},"consumed":false,"claim_status":"available"},{"title":"Fresh sibling","type":"task","severity":"medium","source":"retro-learning","description":"still actionable","target_repo":"` + detectRepoName(dir) + `","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	scorecard, err := loadStigmergicScorecard(dir)
	if err != nil {
		t.Fatalf("loadStigmergicScorecard: %v", err)
	}
	if scorecard.QueueEntries != 1 || scorecard.UnconsumedBatches != 1 {
		t.Fatalf("queue entry counts = %+v, want one visible batch", scorecard)
	}
	if scorecard.UnconsumedItems != 1 {
		t.Fatalf("UnconsumedItems = %d, want 1 visible item", scorecard.UnconsumedItems)
	}
	if scorecard.HighSeverityUnconsumed != 0 {
		t.Fatalf("HighSeverityUnconsumed = %d, want proof-backed high severity item suppressed", scorecard.HighSeverityUnconsumed)
	}
}

func TestCollectRankedContextBundle_ExcludesProofBackedNextWork(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "rpi"), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := detectRepoName(dir)
	writeCompletedLoopRegistryRun(t, dir, "run-proof", "ag-proof", "Already done backlog item")

	queue := `{"source_epic":"ag-proof","timestamp":"2026-03-11T17:00:00Z","items":[{"title":"Already done backlog item","type":"task","severity":"high","source":"council-finding","description":"proof-backed stale work","target_repo":"` + repo + `","proof_ref":{"kind":"completed_run","run_id":"run-proof"},"consumed":false,"claim_status":"available"},{"title":"Fresh sibling","type":"task","severity":"medium","source":"retro-learning","description":"still actionable","target_repo":"` + repo + `","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle := collectRankedContextBundle(dir, "backlog item", 5)
	if len(bundle.NextWork) != 1 {
		t.Fatalf("NextWork = %+v, want only the live sibling", bundle.NextWork)
	}
	if bundle.NextWork[0].Title != "Fresh sibling" {
		t.Fatalf("ranked next-work = %+v, want live sibling only", bundle.NextWork)
	}
	if len(bundle.Packet.PriorFindings) != 1 || bundle.Packet.PriorFindings[0].Title != "Fresh sibling" {
		t.Fatalf("PriorFindings = %+v, want proof-backed item suppressed before ranking", bundle.Packet.PriorFindings)
	}
}

func TestRankStigmergicFindings_PrefersChangedFileOverlap(t *testing.T) {
	dir := t.TempDir()
	findingsDir := filepath.Join(dir, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	matchingFile := `---
id: "f-file"
title: "Status backlog report"
status: "active"
applicable_when: ["task"]
scope_tags: ["status"]
compiler_targets: ["status"]
---
# Finding

Improve backlog reporting in status output.
`
	if err := os.WriteFile(filepath.Join(findingsDir, "f-file.md"), []byte(matchingFile), 0o644); err != nil {
		t.Fatal(err)
	}

	textOnly := `---
id: "f-text"
title: "Status review guidance"
status: "active"
applicable_when: ["task"]
scope_tags: ["workflow"]
---
# Finding

Improve status review guidance without touching status files.
`
	if err := os.WriteFile(filepath.Join(findingsDir, "f-text.md"), []byte(textOnly), 0o644); err != nil {
		t.Fatal(err)
	}

	ranked, err := rankStigmergicFindings(dir, StigmergicTarget{
		GoalText:  "improve status backlog",
		IssueType: "task",
		Files:     []string{"cli/cmd/ao/status.go"},
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("rankStigmergicFindings: %v", err)
	}
	if len(ranked) < 2 {
		t.Fatalf("ranked = %v, want 2 findings", ranked)
	}
	if ranked[0].ID != "f-file" {
		t.Fatalf("top finding = %q, want file-overlap match first", ranked[0].ID)
	}
}
