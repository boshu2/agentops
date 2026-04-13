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
	"time"
)

func writeCompletedLoopRegistryRun(t *testing.T, rootDir, runID, epicID, goal string) {
	t.Helper()
	runDir := filepath.Join(rootDir, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir registry run dir: %v", err)
	}

	state := map[string]any{
		"schema_version": 1,
		"run_id":         runID,
		"epic_id":        epicID,
		"goal":           goal,
		"phase":          3,
		"started_at":     time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal registry state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0o644); err != nil {
		t.Fatalf("write registry state: %v", err)
	}
}

func writeEvidenceOnlyClosurePacket(t *testing.T, rootDir, targetID string) string {
	t.Helper()
	packetDir := filepath.Join(rootDir, ".agents", "releases", "evidence-only-closures")
	if err := os.MkdirAll(packetDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence-only closure dir: %v", err)
	}

	packetPath := filepath.Join(packetDir, strings.ReplaceAll(targetID, "/", "_")+".json")
	packet := map[string]any{
		"schema_version": 1,
		"artifact_id":    "evidence-only-closure-" + targetID,
		"target_id":      targetID,
		"target_type":    "task",
		"created_at":     time.Now().UTC().Format(time.RFC3339),
		"producer":       "rpi-loop-test",
		"evidence_mode":  "staged",
		"validation_commands": []string{
			"bash scripts/validate-go-fast.sh",
		},
		"repo_state": map[string]any{
			"repo_root":       ".",
			"git_branch":      "main",
			"git_dirty":       false,
			"head_sha":        "deadbeef",
			"modified_files":  []string{},
			"staged_files":    []string{},
			"unstaged_files":  []string{},
			"untracked_files": []string{},
		},
		"evidence": map[string]any{
			"summary":   "proof-backed closure",
			"artifacts": []string{".agents/releases/evidence-only-closures/" + strings.ReplaceAll(targetID, "/", "_") + ".json"},
			"notes":     []string{},
		},
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal evidence-only closure packet: %v", err)
	}
	if err := os.WriteFile(packetPath, data, 0o644); err != nil {
		t.Fatalf("write evidence-only closure packet: %v", err)
	}
	return packetPath
}

func TestReadUnconsumedItems_NoFile(t *testing.T) {
	items, err := readUnconsumedItems("/nonexistent/path/next-work.jsonl", "")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestReadUnconsumedItems_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestReadUnconsumedItems_ConsumedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-test",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Should be skipped", Severity: "high"},
		},
		Consumed: true,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items from consumed entry, got %d", len(items))
	}
}

func TestReadUnconsumedItems_UnconsumedWithItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-test",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
			{Title: "Item B", Severity: "low"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "Item A" {
		t.Errorf("expected first item 'Item A', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_EmptyItemsArray(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-empty",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{},
		Consumed:   false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items from empty items array, got %d", len(items))
	}
}

func TestReadUnconsumedItems_MultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	consumed := nextWorkEntry{
		SourceEpic: "ag-old",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{{Title: "Old item", Severity: "low"}},
		Consumed:   true,
	}
	unconsumed := nextWorkEntry{
		SourceEpic: "ag-new",
		Timestamp:  "2026-02-10T01:00:00Z",
		Items:      []nextWorkItem{{Title: "New item", Severity: "medium"}},
		Consumed:   false,
	}

	d1, err := json.Marshal(consumed)
	if err != nil {
		t.Fatalf("marshal consumed: %v", err)
	}
	d2, err := json.Marshal(unconsumed)
	if err != nil {
		t.Fatalf("marshal unconsumed: %v", err)
	}
	content := string(d1) + "\n" + string(d2) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (only unconsumed), got %d", len(items))
	}
	if items[0].Title != "New item" {
		t.Errorf("expected 'New item', got %q", items[0].Title)
	}
}

func TestSelectHighestSeverityItem(t *testing.T) {
	tests := []struct {
		name     string
		items    []nextWorkItem
		expected string
	}{
		{
			name:     "empty",
			items:    nil,
			expected: "",
		},
		{
			name: "single item",
			items: []nextWorkItem{
				{Title: "Only one", Severity: "low"},
			},
			expected: "Only one",
		},
		{
			name: "high beats medium and low",
			items: []nextWorkItem{
				{Title: "Low item", Severity: "low"},
				{Title: "High item", Severity: "high"},
				{Title: "Medium item", Severity: "medium"},
			},
			expected: "High item",
		},
		{
			name: "medium beats low",
			items: []nextWorkItem{
				{Title: "Low item", Severity: "low"},
				{Title: "Medium item", Severity: "medium"},
			},
			expected: "Medium item",
		},
		{
			name: "unknown severity ranks lowest",
			items: []nextWorkItem{
				{Title: "Unknown", Severity: "critical"},
				{Title: "Low item", Severity: "low"},
			},
			expected: "Low item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectHighestSeverityItem(tt.items)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity string
		rank     int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			if got := severityRank(tt.severity); got != tt.rank {
				t.Errorf("severityRank(%q) = %d, want %d", tt.severity, got, tt.rank)
			}
		})
	}
}

func TestRepoAffinityRank(t *testing.T) {
	tests := []struct {
		name       string
		repoFilter string
		item       nextWorkItem
		want       int
	}{
		{
			name:       "empty filter disables affinity ranking",
			repoFilter: "",
			item:       nextWorkItem{TargetRepo: "agentops"},
			want:       0,
		},
		{
			name:       "exact repo wins",
			repoFilter: "agentops",
			item:       nextWorkItem{TargetRepo: "agentops"},
			want:       3,
		},
		{
			name:       "wildcard is second",
			repoFilter: "agentops",
			item:       nextWorkItem{TargetRepo: "*"},
			want:       2,
		},
		{
			name:       "legacy empty target_repo is third",
			repoFilter: "agentops",
			item:       nextWorkItem{},
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repoAffinityRank(tt.item, tt.repoFilter); got != tt.want {
				t.Fatalf("repoAffinityRank(%+v, %q) = %d, want %d", tt.item, tt.repoFilter, got, tt.want)
			}
		})
	}
}

func TestWorkTypeRank(t *testing.T) {
	tests := []struct {
		itemType string
		want     int
	}{
		{itemType: "feature", want: 2},
		{itemType: "improvement", want: 2},
		{itemType: "tech-debt", want: 2},
		{itemType: "pattern-fix", want: 2},
		{itemType: "bug", want: 2},
		{itemType: "task", want: 2},
		{itemType: "process-improvement", want: 1},
		{itemType: "unknown", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			if got := workTypeRank(nextWorkItem{Type: tt.itemType}); got != tt.want {
				t.Fatalf("workTypeRank(%q) = %d, want %d", tt.itemType, got, tt.want)
			}
		})
	}
}

func TestPreflightQueueSelection_ConsumesMatchingCompletedRun(t *testing.T) {
	tmpDir := t.TempDir()
	writeCompletedLoopRegistryRun(t, tmpDir, "run-stale", "ag-stale", "Already done goal")

	decision, err := preflightQueueSelection(tmpDir, &queueSelection{
		Item: nextWorkItem{
			Title: "Already done goal",
		},
		SourceEpic: "ag-stale",
	}, rpiLoopSupervisorConfig{})
	if err != nil {
		t.Fatalf("preflightQueueSelection returned error: %v", err)
	}
	if !decision.Consume {
		t.Fatal("expected preflight to consume queue item backed by completed run proof")
	}
	if !strings.Contains(decision.Reason, "completed RPI run run-stale") {
		t.Fatalf("unexpected preflight reason: %q", decision.Reason)
	}
}

func TestPreflightQueueSelection_ConsumesMatchingEvidenceOnlyClosurePacket(t *testing.T) {
	tmpDir := t.TempDir()
	packetPath := writeEvidenceOnlyClosurePacket(t, tmpDir, "ag-proof.2")

	decision, err := preflightQueueSelection(tmpDir, &queueSelection{
		Item: nextWorkItem{
			Title:       "Close already-proven follow-up",
			Description: "Closure evidence is stored in .agents/releases/evidence-only-closures/ag-proof.2.json.",
		},
		SourceEpic: "ag-parent",
	}, rpiLoopSupervisorConfig{})
	if err != nil {
		t.Fatalf("preflightQueueSelection returned error: %v", err)
	}
	if !decision.Consume {
		t.Fatal("expected preflight to consume queue item backed by evidence-only closure proof")
	}
	if !strings.Contains(decision.Reason, packetPath) {
		t.Fatalf("unexpected preflight reason: %q", decision.Reason)
	}
}

func TestClassifyNextWorkCompletionProof_UsesProofRefBeforeTextFallback(t *testing.T) {
	tmpDir := t.TempDir()
	packetPath := writeEvidenceOnlyClosurePacket(t, tmpDir, "ag-proof.2")

	proof := classifyNextWorkCompletionProof(tmpDir, "ag-parent", nextWorkItem{
		Title:       "Close already-proven follow-up",
		Description: "Legacy notes mention .agents/releases/evidence-only-closures/ag-wrong.9.json, but proof_ref should win.",
		ProofRef: &nextWorkProofRef{
			Kind:     "evidence_only_closure",
			TargetID: "ag-proof.2",
		},
	})
	if !proof.Complete {
		t.Fatal("expected explicit proof_ref to classify the item as complete")
	}
	if proof.Source != "evidence_only_closure" {
		t.Fatalf("unexpected proof source: %+v", proof)
	}
	if !strings.Contains(proof.Detail, packetPath) {
		t.Fatalf("expected proof detail to cite explicit proof_ref packet path, got %q", proof.Detail)
	}
	if strings.Contains(proof.Detail, "ag-wrong.9") {
		t.Fatalf("expected proof_ref precedence over text fallback, got %q", proof.Detail)
	}
}

func TestClassifyNextWorkCompletionProof_UsesExecutionPacketPath(t *testing.T) {
	tmpDir := t.TempDir()
	relPacketPath := filepath.Join(".agents", "rpi", "runs", "run-path", "execution-packet.json")
	absPacketPath := filepath.Join(tmpDir, relPacketPath)
	if err := os.MkdirAll(filepath.Dir(absPacketPath), 0o755); err != nil {
		t.Fatalf("mkdir execution packet dir: %v", err)
	}
	packet := map[string]any{
		"objective": "consume proof-backed next-work item",
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal execution packet: %v", err)
	}
	if err := os.WriteFile(absPacketPath, data, 0o644); err != nil {
		t.Fatalf("write execution packet: %v", err)
	}

	proof := classifyNextWorkCompletionProof(tmpDir, "ag-parent", nextWorkItem{
		Title: "Consume stale execution packet item",
		ProofRef: &nextWorkProofRef{
			Kind:  "execution_packet",
			RunID: "run-path",
			Path:  relPacketPath,
		},
	})
	if !proof.Complete {
		t.Fatal("expected execution packet path proof to classify the item as complete")
	}
	if proof.Source != "execution_packet" {
		t.Fatalf("unexpected proof source: %+v", proof)
	}
	if proof.Detail != "run-path" {
		t.Fatalf("proof detail = %q, want run-path", proof.Detail)
	}
}

func TestPreflightQueueSelection_DoesNotConsumeWithoutExplicitProof(t *testing.T) {
	decision, err := preflightQueueSelection(t.TempDir(), &queueSelection{
		Item: nextWorkItem{
			Title: "Merge maturity_deep_test.go and fire_deep_test.go into canonical test files",
		},
		SourceEpic: "ag-bn9",
	}, rpiLoopSupervisorConfig{})
	if err != nil {
		t.Fatalf("preflightQueueSelection returned error: %v", err)
	}
	if decision.Consume {
		t.Fatalf("expected proof-less merge item to remain actionable, got decision %+v", decision)
	}
}

func TestResolveLoopGoal_PreflightConsumesSkippedItemAndAdvances(t *testing.T) {
	prevPreflight := preflightQueueSelectionFn
	defer func() { preflightQueueSelectionFn = prevPreflight }()
	preflightQueueSelectionFn = func(_ string, sel *queueSelection, _ rpiLoopSupervisorConfig) (queuePreflightDecision, error) {
		if sel != nil && sel.Item.Title == "Already done" {
			return queuePreflightDecision{Consume: true, Reason: "already satisfied"}, nil
		}
		return queuePreflightDecision{}, nil
	}

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRepoFilter := rpiRepoFilter
	rpiRepoFilter = ""
	defer func() { rpiRepoFilter = prevRepoFilter }()

	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-skip",
			Items:      []nextWorkItem{{Title: "Already done", Severity: "high"}},
		},
		{
			SourceEpic: "ag-next",
			Items:      []nextWorkItem{{Title: "Do this now", Severity: "medium"}},
		},
	})

	goal, sel, action, err := resolveLoopGoal(tmpDir, "", queuePath, rpiLoopSupervisorConfig{})
	if err != nil {
		t.Fatalf("resolveLoopGoal returned error: %v", err)
	}
	if action != loopContinue {
		t.Fatalf("action = %v, want %v", action, loopContinue)
	}
	if goal != "Do this now" {
		t.Fatalf("goal = %q, want %q", goal, "Do this now")
	}
	if sel == nil || sel.Item.Title != "Do this now" {
		t.Fatalf("selected item = %+v, want Do this now", sel)
	}

	after := readJSONLEntries(t, queuePath)
	if !after[0].Items[0].Consumed {
		t.Fatal("expected preflight-skipped item to be consumed")
	}
	if after[0].Items[0].ConsumedBy == nil || *after[0].Items[0].ConsumedBy != queuePreflightConsumedBy {
		t.Fatalf("consumed_by = %v, want %q", after[0].Items[0].ConsumedBy, queuePreflightConsumedBy)
	}
	if after[1].Items[0].Consumed {
		t.Fatal("expected next actionable item to remain unconsumed before execution")
	}
}

func TestResolveLoopGoal_KeepsLegacyFailedEntryWithoutProof(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	writeJSONL(t, queuePath, []nextWorkEntry{{
		SourceEpic: "ag-legacy",
		Items: []nextWorkItem{{
			Title:       "Retryable legacy item",
			Type:        "bug",
			Severity:    "high",
			Source:      "retro-learning",
			Description: "No proof exists yet.",
		}},
		FailedAt: &failedAt,
	}})

	var (
		goal   string
		sel    *queueSelection
		action loopCycleResult
	)
	output, err := captureStdout(t, func() error {
		var innerErr error
		goal, sel, action, innerErr = resolveLoopGoal(tmpDir, "", queuePath, rpiLoopSupervisorConfig{})
		return innerErr
	})
	if err != nil {
		t.Fatalf("resolveLoopGoal returned error: %v", err)
	}
	if action != loopContinue {
		t.Fatalf("action = %v, want %v", action, loopContinue)
	}
	if goal != "Retryable legacy item" {
		t.Fatalf("goal = %q, want retryable legacy item", goal)
	}
	if sel == nil || sel.Item.Title != "Retryable legacy item" {
		t.Fatalf("selected item = %+v, want retryable legacy item", sel)
	}
	if !strings.Contains(output, "From queue: Retryable legacy item") {
		t.Fatalf("expected queue selection output for legacy item, got:\n%s", output)
	}
}

func TestResolveLoopGoal_PreflightConsumesLegacyFailedEntryWithCompletedRun(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	writeJSONL(t, queuePath, []nextWorkEntry{{
		SourceEpic: "ag-stale",
		Items: []nextWorkItem{{
			Title:       "Already done goal",
			Type:        "task",
			Severity:    "high",
			Source:      "retro-learning",
			Description: "Legacy failed row with completed-run proof.",
		}},
		FailedAt: &failedAt,
	}})
	writeCompletedLoopRegistryRun(t, tmpDir, "run-stale", "ag-stale", "Already done goal")

	var (
		goal   string
		sel    *queueSelection
		action loopCycleResult
	)
	output, err := captureStdout(t, func() error {
		var innerErr error
		goal, sel, action, innerErr = resolveLoopGoal(tmpDir, "", queuePath, rpiLoopSupervisorConfig{})
		return innerErr
	})
	if err != nil {
		t.Fatalf("resolveLoopGoal returned error: %v", err)
	}
	if action != loopBreak {
		t.Fatalf("action = %v, want %v", action, loopBreak)
	}
	if goal != "" {
		t.Fatalf("goal = %q, want empty after proof-backed consume", goal)
	}
	if sel != nil {
		t.Fatalf("expected no selected item after queue empties, got %+v", sel)
	}
	if !strings.Contains(output, `Queue preflight consumed "Already done goal": matched completed RPI run run-stale`) {
		t.Fatalf("expected completed-run preflight message, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if len(after) != 1 {
		t.Fatalf("expected one queue entry, got %d", len(after))
	}
	if !after[0].Consumed {
		t.Fatalf("expected legacy row consumed by preflight, got %+v", after[0])
	}
	if len(after[0].Items) != 1 || !after[0].Items[0].Consumed {
		t.Fatalf("expected legacy batch item consumed by preflight, got %+v", after[0])
	}
	if after[0].Items[0].ConsumedBy == nil || *after[0].Items[0].ConsumedBy != queuePreflightConsumedBy {
		t.Fatalf("consumed_by = %v, want %q", after[0].Items[0].ConsumedBy, queuePreflightConsumedBy)
	}
}

func TestResolveLoopGoal_PreflightConsumesLegacyFailedEntryWithEvidenceOnlyClosure(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	writeJSONL(t, queuePath, []nextWorkEntry{{
		SourceEpic: "ag-parent",
		Items: []nextWorkItem{{
			Title:       "Already proven item",
			Type:        "task",
			Severity:    "high",
			Source:      "retro-learning",
			Description: "See .agents/releases/evidence-only-closures/ag-proof.2.json.",
		}},
		FailedAt: &failedAt,
	}})
	writeEvidenceOnlyClosurePacket(t, tmpDir, "ag-proof.2")

	var (
		goal   string
		sel    *queueSelection
		action loopCycleResult
	)
	output, err := captureStdout(t, func() error {
		var innerErr error
		goal, sel, action, innerErr = resolveLoopGoal(tmpDir, "", queuePath, rpiLoopSupervisorConfig{})
		return innerErr
	})
	if err != nil {
		t.Fatalf("resolveLoopGoal returned error: %v", err)
	}
	if action != loopBreak {
		t.Fatalf("action = %v, want %v", action, loopBreak)
	}
	if goal != "" {
		t.Fatalf("goal = %q, want empty after proof-backed consume", goal)
	}
	if sel != nil {
		t.Fatalf("expected no selected item after queue empties, got %+v", sel)
	}
	if !strings.Contains(output, `Queue preflight consumed "Already proven item": matched evidence-only closure proof for ag-proof.2`) {
		t.Fatalf("expected evidence-only preflight message, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if len(after) != 1 {
		t.Fatalf("expected one queue entry, got %d", len(after))
	}
	if !after[0].Consumed {
		t.Fatalf("expected legacy row consumed by evidence-only proof, got %+v", after[0])
	}
	if len(after[0].Items) != 1 || !after[0].Items[0].Consumed {
		t.Fatalf("expected legacy batch item consumed by evidence-only proof, got %+v", after[0])
	}
	if after[0].Items[0].ConsumedBy == nil || *after[0].Items[0].ConsumedBy != queuePreflightConsumedBy {
		t.Fatalf("consumed_by = %v, want %q", after[0].Items[0].ConsumedBy, queuePreflightConsumedBy)
	}
}

func TestReadUnconsumedItems_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-valid",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{{Title: "Valid", Severity: "high"}},
		Consumed:   false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	content := "not json at all\n" + string(data) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (skip malformed), got %d", len(items))
	}
	if items[0].Title != "Valid" {
		t.Errorf("expected 'Valid', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Match(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For agentops", Severity: "high", TargetRepo: "agentops"},
			{Title: "For olympus", Severity: "medium", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item matching repo filter, got %d", len(items))
	}
	if items[0].Title != "For agentops" {
		t.Errorf("expected 'For agentops', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Exclude(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For olympus only", Severity: "high", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items (filtered out), got %d", len(items))
	}
}

func TestReadUnconsumedItems_RepoFilter_Wildcard(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For all repos", Severity: "high", TargetRepo: "*"},
			{Title: "For olympus", Severity: "low", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (wildcard passes, olympus excluded), got %d", len(items))
	}
	if items[0].Title != "For all repos" {
		t.Errorf("expected 'For all repos', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Legacy(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	// Legacy items have no target_repo field (empty string after deserialization)
	entry := nextWorkEntry{
		SourceEpic: "ag-legacy",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Legacy item", Severity: "medium"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Legacy items (no target_repo) should pass any filter
	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (legacy passes all filters), got %d", len(items))
	}
	if items[0].Title != "Legacy item" {
		t.Errorf("expected 'Legacy item', got %q", items[0].Title)
	}
}

func TestReadQueueEntries_LegacyFlatEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	line := `{"id":"nw-legacy-1","source_epic":"ag-legacy","title":"Legacy item","type":"tech-debt","severity":"high","source":"retro-learning","description":"legacy flat row","target_repo":"agentops","consumed":false,"created_at":"2026-02-11T11:04:30-05:00"}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Timestamp != "2026-02-11T11:04:30-05:00" {
		t.Fatalf("timestamp = %q, want created_at fallback", entry.Timestamp)
	}
	if entry.LegacyID != "nw-legacy-1" {
		t.Fatalf("legacy id = %q, want nw-legacy-1", entry.LegacyID)
	}
	if len(entry.Items) != 1 {
		t.Fatalf("expected synthesized single item, got %d", len(entry.Items))
	}
	if entry.Items[0].Title != "Legacy item" {
		t.Fatalf("item title = %q, want Legacy item", entry.Items[0].Title)
	}
}

func TestSelectHighestSeverityEntry_LegacyFlatEntry(t *testing.T) {
	entries := []nextWorkEntry{
		{
			SourceEpic: "ag-legacy",
			LegacyID:   "nw-legacy-1",
			Items: []nextWorkItem{{
				Title:    "Legacy high",
				Severity: "high",
			}},
			QueueIndex: 0,
		},
		{
			SourceEpic: "ag-batch",
			Items: []nextWorkItem{{
				Title:    "Batch low",
				Severity: "low",
			}},
			QueueIndex: 1,
		},
	}

	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Legacy high" {
		t.Fatalf("selected %q, want Legacy high", sel.Item.Title)
	}
	if sel.EntryIndex != 0 {
		t.Fatalf("entry index = %d, want 0", sel.EntryIndex)
	}
}

func TestReadUnconsumedItems_RepoFilter_EmptyFilter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For agentops", Severity: "high", TargetRepo: "agentops"},
			{Title: "For olympus", Severity: "medium", TargetRepo: "olympus"},
			{Title: "Legacy", Severity: "low"},
		},
		Consumed: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty filter means no filtering - all items pass
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items (no filter), got %d", len(items))
	}
}

// ---- Queue mark semantics ----

func writeJSONL(t *testing.T, path string, entries []nextWorkEntry) {
	t.Helper()
	var out strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		out.Write(data)
		out.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(out.String()), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readJSONLEntries(t *testing.T, path string) []nextWorkEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var entries []nextWorkEntry
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e nextWorkEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestQueueMarkConsumed_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Timestamp: "2026-02-10T00:00:00Z", Items: []nextWorkItem{{Title: "Item 1", Severity: "high"}}, Consumed: false},
		{SourceEpic: "ag-2", Timestamp: "2026-02-10T01:00:00Z", Items: []nextWorkItem{{Title: "Item 2", Severity: "low"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryConsumed(path, 0, "test-runner"); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if !got[0].Consumed {
		t.Errorf("entry 0: expected Consumed=true")
	}
	if got[0].ConsumedAt == nil {
		t.Errorf("entry 0: expected ConsumedAt to be set")
	}
	if got[0].ConsumedBy == nil || *got[0].ConsumedBy != "test-runner" {
		t.Errorf("entry 0: expected ConsumedBy=test-runner")
	}
	if got[0].CompletionEvidence != "bead_closed" {
		t.Errorf("entry 0: expected CompletionEvidence=bead_closed, got %q", got[0].CompletionEvidence)
	}
	if got[0].CompletionEvidenceAt == nil {
		t.Errorf("entry 0: expected CompletionEvidenceAt to be set")
	}
	// Entry 1 should be untouched.
	if got[1].Consumed {
		t.Errorf("entry 1: should not be consumed")
	}
}

func strPtr(s string) *string { return &s }

func TestShouldSkipLegacyFailedEntry_ProofBacked(t *testing.T) {
	// Entries with CompletionEvidence should be skipped regardless of other metadata.
	failedAt := "2026-04-04T00:00:00Z"
	worker := "worker-1"
	entry := nextWorkEntry{
		FailedAt:           &failedAt,
		CompletionEvidence: "bead_closed",
		ClaimStatus:        "available",
		Items:              []nextWorkItem{{Title: "done", ClaimedBy: &worker}},
	}
	if !shouldSkipLegacyFailedEntry(entry) {
		t.Error("entry with CompletionEvidence should be skipped")
	}
}

func TestShouldSkipLegacyFailedEntry_NoProofKeepsAvailable(t *testing.T) {
	// Entries with FailedAt but lifecycle metadata and no CompletionEvidence remain available.
	failedAt := "2026-04-04T00:00:00Z"
	worker := "worker-1"
	entry := nextWorkEntry{
		FailedAt:    &failedAt,
		ClaimStatus: "available",
		Items:       []nextWorkItem{{Title: "retry me", ClaimedBy: &worker}},
	}
	if shouldSkipLegacyFailedEntry(entry) {
		t.Error("entry with lifecycle metadata but no CompletionEvidence should remain available")
	}
}

func TestShouldSkipLegacyFailedEntry_NoMetadataNoProofStaysAvailable(t *testing.T) {
	// Pre-v2.34 heuristic would have skipped entries with no lifecycle metadata.
	// After the proof-backed change, these entries remain available for retry
	// because there is no CompletionEvidence proving the work was done.
	failedAt := "2026-03-15T00:00:00Z"
	entry := nextWorkEntry{
		FailedAt: &failedAt,
		Items:    []nextWorkItem{{Title: "bare failed item"}},
	}
	if shouldSkipLegacyFailedEntry(entry) {
		t.Error("entry with FailedAt but no CompletionEvidence should remain available (no heuristic suppression)")
	}
}

func TestMarkEntryConsumed_LegacyFlatEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	line := `{"id":"nw-legacy-1","source_epic":"ag-legacy","title":"Legacy item","type":"tech-debt","severity":"high","consumed":false,"created_at":"2026-02-11T11:04:30-05:00"}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := markItemConsumed(path, 0, 0, "ao-rpi-loop"); err != nil {
		t.Fatalf("markItemConsumed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("unmarshal rewritten legacy line: %v", err)
	}
	if consumed, _ := entry["consumed"].(bool); !consumed {
		t.Fatalf("consumed = %v, want true", entry["consumed"])
	}
	if _, ok := entry["items"]; ok {
		t.Fatal("legacy flat row should remain flat after rewrite")
	}
	if entry["title"] != "Legacy item" {
		t.Fatalf("title = %v, want Legacy item", entry["title"])
	}
}

func TestQueueMarkConsumed_SecondEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Items: []nextWorkItem{{Title: "First"}}, Consumed: false},
		{SourceEpic: "ag-2", Items: []nextWorkItem{{Title: "Second"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryConsumed(path, 1, "loop"); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if got[0].Consumed {
		t.Errorf("entry 0 should not be consumed")
	}
	if !got[1].Consumed {
		t.Errorf("entry 1 should be consumed")
	}
}

func TestQueueMarkFailed_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-fail", Items: []nextWorkItem{{Title: "Failing item"}}, Consumed: false, ClaimStatus: "available"},
	}
	writeJSONL(t, path, entries)

	if err := markItemFailed(path, 0, 0); err != nil {
		t.Fatalf("markItemFailed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if got[0].Consumed {
		t.Errorf("failed entry should not be marked consumed")
	}
	if got[0].FailedAt == nil {
		t.Errorf("expected FailedAt to be set")
	}
	if got[0].Items[0].FailedAt == nil {
		t.Errorf("expected item-level FailedAt to be set")
	}
	if got[0].Items[0].ClaimStatus != "available" {
		t.Errorf("failed item claim_status = %q, want available", got[0].Items[0].ClaimStatus)
	}
}

func TestMarkEntryFailed_LegacyFlatEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	line := `{"id":"nw-legacy-1","source_epic":"ag-legacy","title":"Legacy item","type":"tech-debt","severity":"high","consumed":false,"created_at":"2026-02-11T11:04:30-05:00"}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := markEntryFailed(path, 0); err != nil {
		t.Fatalf("markEntryFailed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("unmarshal rewritten legacy line: %v", err)
	}
	if _, ok := entry["failed_at"]; !ok {
		t.Fatal("expected failed_at on rewritten legacy row")
	}
	if _, ok := entry["items"]; ok {
		t.Fatal("legacy flat row should remain flat after failure rewrite")
	}
}

func TestQueueMarkFailed_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-fail", Items: []nextWorkItem{{Title: "Failing item"}}, Consumed: false, ClaimStatus: "available"},
	}
	writeJSONL(t, path, entries)

	if err := markItemFailed(path, 0, 0); err != nil {
		t.Fatalf("first markItemFailed: %v", err)
	}
	first := readJSONLEntries(t, path)
	firstTime := *first[0].FailedAt

	// Mark again (idempotent - updates timestamp but remains non-consumed).
	if err := markItemFailed(path, 0, 0); err != nil {
		t.Fatalf("second markItemFailed: %v", err)
	}
	second := readJSONLEntries(t, path)
	if second[0].Consumed {
		t.Errorf("should not be consumed after double-failure")
	}
	// Second call may update the timestamp; it should still be a valid timestamp.
	if second[0].FailedAt == nil {
		t.Errorf("FailedAt should still be set after second call")
	}
	_ = firstTime // both are valid
}

func TestMarkItemClaimedAndReleased(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic:  "ag-claim",
		ClaimStatus: "available",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
			{Title: "Item B", Severity: "medium"},
		},
	}
	writeJSONL(t, path, []nextWorkEntry{entry})

	if err := markItemClaimed(path, 0, 0, "loop:cycle-1"); err != nil {
		t.Fatalf("markItemClaimed: %v", err)
	}
	claimed := readJSONLEntries(t, path)
	if claimed[0].ClaimStatus != "in_progress" {
		t.Fatalf("entry claim_status = %q, want in_progress", claimed[0].ClaimStatus)
	}
	if claimed[0].Items[0].ClaimStatus != "in_progress" {
		t.Fatalf("item claim_status = %q, want in_progress", claimed[0].Items[0].ClaimStatus)
	}
	if claimed[0].Items[1].ClaimStatus != "available" {
		t.Fatalf("untouched sibling claim_status = %q, want available", claimed[0].Items[1].ClaimStatus)
	}

	if err := releaseItemClaim(path, 0, 0); err != nil {
		t.Fatalf("releaseItemClaim: %v", err)
	}
	released := readJSONLEntries(t, path)
	if released[0].ClaimStatus != "available" {
		t.Fatalf("entry claim_status = %q, want available", released[0].ClaimStatus)
	}
	if released[0].Items[0].ClaimStatus != "available" {
		t.Fatalf("item claim_status = %q, want available", released[0].Items[0].ClaimStatus)
	}
	if released[0].Items[0].ClaimedBy != nil || released[0].Items[0].ClaimedAt != nil {
		t.Fatal("released item should clear claimed_by/claimed_at")
	}
}

func TestMarkItemClaimedConflict(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic:  "ag-claim",
		ClaimStatus: "available",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
		},
	}
	writeJSONL(t, path, []nextWorkEntry{entry})

	if err := markItemClaimed(path, 0, 0, "loop:cycle-1"); err != nil {
		t.Fatalf("first markItemClaimed: %v", err)
	}
	err := markItemClaimed(path, 0, 0, "loop:cycle-2")
	if !errors.Is(err, errQueueClaimConflict) {
		t.Fatalf("second markItemClaimed error = %v, want errQueueClaimConflict", err)
	}

	claimed := readJSONLEntries(t, path)
	if claimed[0].Items[0].ClaimStatus != "in_progress" {
		t.Fatalf("item claim_status = %q, want in_progress", claimed[0].Items[0].ClaimStatus)
	}
	if claimed[0].Items[0].ClaimedBy == nil || *claimed[0].Items[0].ClaimedBy != "loop:cycle-1" {
		t.Fatalf("claimed_by = %v, want loop:cycle-1", claimed[0].Items[0].ClaimedBy)
	}
}

func TestMarkItemClaimedConcurrentSingleWinner(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic:  "ag-claim",
		ClaimStatus: "available",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
		},
	}
	writeJSONL(t, path, []nextWorkEntry{entry})

	claimers := []string{"loop:cycle-1", "loop:cycle-2"}
	start := make(chan struct{})
	errs := make(chan error, len(claimers))

	for _, claimer := range claimers {
		claimer := claimer
		go func() {
			<-start
			errs <- markItemClaimed(path, 0, 0, claimer)
		}()
	}
	close(start)

	successes := 0
	conflicts := 0
	for range claimers {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, errQueueClaimConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent claim error: %v", err)
		}
	}

	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}

	claimed := readJSONLEntries(t, path)
	if claimed[0].Items[0].ClaimStatus != "in_progress" {
		t.Fatalf("item claim_status = %q, want in_progress", claimed[0].Items[0].ClaimStatus)
	}
	if claimed[0].Items[0].ClaimedBy == nil {
		t.Fatal("claimed_by = nil, want winning claimer")
	}
	if *claimed[0].Items[0].ClaimedBy != claimers[0] && *claimed[0].Items[0].ClaimedBy != claimers[1] {
		t.Fatalf("claimed_by = %q, want one of the concurrent claimers", *claimed[0].Items[0].ClaimedBy)
	}
}

func TestMarkItemConsumedOwnedConflict(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic:  "ag-claim",
		ClaimStatus: "available",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
		},
	}
	writeJSONL(t, path, []nextWorkEntry{entry})

	if err := markItemClaimed(path, 0, 0, "loop:cycle-1"); err != nil {
		t.Fatalf("markItemClaimed: %v", err)
	}
	err := markItemConsumedOwned(path, 0, 0, "ao-rpi-loop", "loop:cycle-2")
	if !errors.Is(err, errQueueClaimConflict) {
		t.Fatalf("markItemConsumedOwned error = %v, want errQueueClaimConflict", err)
	}

	claimed := readJSONLEntries(t, path)
	if claimed[0].Items[0].ClaimStatus != "in_progress" {
		t.Fatalf("item claim_status = %q, want in_progress", claimed[0].Items[0].ClaimStatus)
	}
	if claimed[0].Items[0].Consumed {
		t.Fatal("item should remain unconsumed after owner mismatch")
	}
}

func TestRunCycleWithRetries_ClaimConflictContinuesQueue(t *testing.T) {
	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic:  "ag-claim",
		ClaimStatus: "available",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
		},
	}
	writeJSONL(t, path, []nextWorkEntry{entry})

	entries, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected queue selection, got nil")
	}

	if err := markItemClaimed(path, sel.EntryIndex, sel.ItemIndex, "other-consumer"); err != nil {
		t.Fatalf("markItemClaimed(other-consumer): %v", err)
	}

	called := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		called++
		return nil
	}

	result, err := runCycleWithRetries(context.Background(), tmpDir, sel.Item.Title, 1, 1, path, sel, "", rpiLoopSupervisorConfig{})
	if err != nil {
		t.Fatalf("runCycleWithRetries error = %v, want nil", err)
	}
	if result != loopContinue {
		t.Fatalf("runCycleWithRetries result = %v, want loopContinue", result)
	}
	if called != 0 {
		t.Fatalf("runRPISupervisedCycleFn called %d times, want 0", called)
	}
}

func TestQueueMarkConsumed_MissingFile(t *testing.T) {
	// Missing file returns an error (callers distinguish missing queue from no-op).
	err := markEntryConsumed("/nonexistent/path/next-work.jsonl", 0, "loop")
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestQueueMarkFailed_MissingFile(t *testing.T) {
	// Missing file is a no-op for markEntryFailed (best-effort warning semantics).
	err := markEntryFailed("/nonexistent/path/next-work.jsonl", 0)
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

// ---- readQueueEntries ----

func TestReadQueueEntries_SkipsConsumed(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Items: []nextWorkItem{{Title: "Consumed"}}, Consumed: true},
		{SourceEpic: "ag-2", Items: []nextWorkItem{{Title: "Open"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].SourceEpic != "ag-2" {
		t.Errorf("expected ag-2, got %q", got[0].SourceEpic)
	}
}

func TestReadQueueEntries_KeepsLegacyFailedEntriesSelectable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	entries := []nextWorkEntry{
		// Failed WITH proof → skipped (proof-backed completion)
		{SourceEpic: "ag-done", Items: []nextWorkItem{{Title: "Done item"}}, Consumed: false, FailedAt: &failedAt, CompletionEvidence: "bead_closed"},
		// Failed WITHOUT proof → stays available for retry
		{SourceEpic: "ag-retry", Items: []nextWorkItem{{Title: "Retry item"}}, Consumed: false, FailedAt: &failedAt},
		{SourceEpic: "ag-open", Items: []nextWorkItem{{Title: "Open item"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// readQueueEntries returns all non-consumed entries; proof filtering is downstream
	if len(got) != 3 {
		t.Fatalf("expected 3 entries from readQueueEntries, got %d", len(got))
	}
	// shouldSkipLegacyFailedEntry filters proof-backed entries
	if !shouldSkipLegacyFailedEntry(got[0]) {
		t.Errorf("ag-done with CompletionEvidence should be skipped")
	}
	if shouldSkipLegacyFailedEntry(got[1]) {
		t.Errorf("ag-retry without proof should NOT be skipped")
	}
	if shouldSkipLegacyFailedEntry(got[2]) {
		t.Errorf("ag-open (not failed) should NOT be skipped")
	}
}

func TestReadQueueEntries_KeepsPerItemFailedEntriesSelectable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	entries := []nextWorkEntry{
		{
			SourceEpic:  "ag-batch",
			ClaimStatus: "available",
			FailedAt:    &failedAt,
			Items: []nextWorkItem{
				{Title: "Retry me later", Severity: "high", ClaimStatus: "available", FailedAt: &failedAt},
				{Title: "Fresh sibling", Severity: "medium"},
			},
		},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 eligible entry, got %d", len(got))
	}
	sel := selectHighestSeverityEntry(got, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Fresh sibling" {
		t.Fatalf("selected %q, want fresh sibling before retryable failed item", sel.Item.Title)
	}
}

func TestReadQueueEntries_SkipsEmptyItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-empty", Items: []nextWorkItem{}, Consumed: false},
		{SourceEpic: "ag-ok", Items: []nextWorkItem{{Title: "Has items"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (empty-items skipped), got %d", len(got))
	}
	if got[0].SourceEpic != "ag-ok" {
		t.Errorf("expected ag-ok, got %q", got[0].SourceEpic)
	}
}

// ---- selectHighestSeverityEntry ----

func TestSelectHighestSeverityEntry_Empty(t *testing.T) {
	sel := selectHighestSeverityEntry(nil, "")
	if sel != nil {
		t.Errorf("expected nil for empty entries")
	}
}

func TestSelectHighestSeverityEntry_PicksHighest(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{{Title: "Low item", Severity: "low"}}},
		{Items: []nextWorkItem{{Title: "High item", Severity: "high"}}},
		{Items: []nextWorkItem{{Title: "Medium item", Severity: "medium"}}},
	}
	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "High item" {
		t.Errorf("expected 'High item', got %q", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_RepoFilter(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{
			{Title: "For olympus", Severity: "high", TargetRepo: "olympus"},
			{Title: "For agentops", Severity: "medium", TargetRepo: "agentops"},
		}},
	}
	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "For agentops" {
		t.Errorf("expected 'For agentops' (filtered by repo), got %q", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_PrefersExactRepoOverWildcard(t *testing.T) {
	entries := []nextWorkEntry{
		{QueueIndex: 0, Items: []nextWorkItem{
			{Title: "Wildcard process", Severity: "high", Type: "process-improvement", TargetRepo: "*"},
			{Title: "Exact repo fix", Severity: "high", Type: "tech-debt", TargetRepo: "agentops"},
		}},
	}

	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Exact repo fix" {
		t.Fatalf("selected %q, want exact repo item", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_PrefersWildcardOverLegacyWhenSeverityTied(t *testing.T) {
	entries := []nextWorkEntry{
		{QueueIndex: 0, Items: []nextWorkItem{
			{Title: "Legacy unscoped", Severity: "medium", Type: "tech-debt"},
			{Title: "Wildcard scoped", Severity: "medium", Type: "tech-debt", TargetRepo: "*"},
		}},
	}

	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Wildcard scoped" {
		t.Fatalf("selected %q, want wildcard item", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_SeverityStillWinsWithinAffinityBucket(t *testing.T) {
	entries := []nextWorkEntry{
		{QueueIndex: 0, Items: []nextWorkItem{
			{Title: "Exact medium", Severity: "medium", Type: "tech-debt", TargetRepo: "agentops"},
			{Title: "Exact high", Severity: "high", Type: "process-improvement", TargetRepo: "agentops"},
		}},
	}

	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Exact high" {
		t.Fatalf("selected %q, want higher severity exact-repo item", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_PrefersImplementationWorkTypeOnTie(t *testing.T) {
	entries := []nextWorkEntry{
		{QueueIndex: 0, Items: []nextWorkItem{
			{Title: "Process chore", Severity: "high", Type: "process-improvement", TargetRepo: "agentops"},
			{Title: "Code fix", Severity: "high", Type: "tech-debt", TargetRepo: "agentops"},
		}},
	}

	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "Code fix" {
		t.Fatalf("selected %q, want implementation-oriented item", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_RepoFilter_NoneMatch(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{
			{Title: "For olympus", Severity: "high", TargetRepo: "olympus"},
		}},
	}
	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel != nil {
		t.Errorf("expected nil (no matching items), got %+v", sel)
	}
}

func TestSelectHighestSeverityEntry_EntryIndexCorrect(t *testing.T) {
	entries := []nextWorkEntry{
		{SourceEpic: "ag-0", QueueIndex: 0, Items: []nextWorkItem{{Title: "Entry 0", Severity: "low"}}},
		{SourceEpic: "ag-1", QueueIndex: 1, Items: []nextWorkItem{{Title: "Entry 1", Severity: "high"}}},
	}
	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.EntryIndex != 1 {
		t.Errorf("expected EntryIndex=1 (high severity), got %d", sel.EntryIndex)
	}
}

func TestSelectHighestSeverityEntry_UsesParseableQueueIndex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	consumed := nextWorkEntry{
		SourceEpic: "ag-consumed",
		Items:      []nextWorkItem{{Title: "Consumed", Severity: "low"}},
		Consumed:   true,
	}
	open := nextWorkEntry{
		SourceEpic: "ag-open",
		Items:      []nextWorkItem{{Title: "Open", Severity: "high"}},
		Consumed:   false,
	}
	consumedData, err := json.Marshal(consumed)
	if err != nil {
		t.Fatalf("marshal consumed: %v", err)
	}
	openData, err := json.Marshal(open)
	if err != nil {
		t.Fatalf("marshal open: %v", err)
	}
	content := string(consumedData) + "\nnot-json\n" + string(openData) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	entries, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one eligible entry, got %d", len(entries))
	}
	if entries[0].QueueIndex != 1 {
		t.Fatalf("expected parseable queue index 1, got %d", entries[0].QueueIndex)
	}

	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.EntryIndex != 1 {
		t.Fatalf("expected queue entry index 1, got %d", sel.EntryIndex)
	}
}

// clearRPIEnv neutralizes environment variables that leak into RPILoop tests.
func clearRPIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
}

// ---- RPILoop dry-run ----

func TestRPILoop_DryRun_ExplicitGoal(t *testing.T) {
	clearRPIEnv(t)
	// The loop should not call the phased engine in dry-run mode.
	// It should print what it would do and return nil.
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 0
	defer func() { rpiMaxCycles = prevMaxCycles }()

	// Provide an explicit goal so we don't need a next-work.jsonl file.
	err := runRPILoop(nil, []string{"test goal"})
	if err != nil {
		t.Errorf("expected nil error in dry-run, got: %v", err)
	}
}

func TestRPILoop_DryRun_EmptyQueue(t *testing.T) {
	clearRPIEnv(t)
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	// No next-work.jsonl in temp dir, so queue is empty.
	// Loop should detect empty queue before dry-run branch is reached.
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err := runRPILoop(nil, nil)
	if err != nil {
		t.Errorf("expected nil error for empty queue, got: %v", err)
	}
}

func TestRPILoop_DryRun_FromQueue(t *testing.T) {
	clearRPIEnv(t)
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 0
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a queue with one item.
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entry := nextWorkEntry{
		SourceEpic: "ag-dryrun",
		Items:      []nextWorkItem{{Title: "Dry run goal", Severity: "high"}},
		Consumed:   false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	if err := os.WriteFile(queuePath, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	err = runRPILoop(nil, nil)
	if err != nil {
		t.Errorf("expected nil error in dry-run, got: %v", err)
	}

	// In dry-run, the queue entry should NOT be marked consumed.
	after := readJSONLEntries(t, queuePath)
	if after[0].Consumed {
		t.Errorf("queue entry should not be consumed in dry-run mode")
	}
}

func TestRPILoop_InfraFailure_DoesNotMarkQueueFailed(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-infra",
		Items:      []nextWorkItem{{Title: "Infra failing goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 1
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	attempts := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", fmt.Errorf("transient network"))
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected loop to fail under failure-policy=stop")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (1 retry), got %d", attempts)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil {
		t.Fatal("infra failures should not mark queue entry failed")
	}
	if after[0].Consumed {
		t.Fatal("infra failures should not mark queue entry consumed")
	}
}

func TestRPILoop_InfraFailure_ContinuePolicy_RetriesUntilMaxCycles(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-infra-continue",
		Items:      []nextWorkItem{{Title: "Infra continue goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyContinue
	rpiCycleRetries = 1
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	attempts := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", fmt.Errorf("simulated rebase conflict"))
	}

	if err := runRPILoop(nil, nil); err != nil {
		t.Fatalf("expected nil error under failure-policy=continue, got: %v", err)
	}
	if attempts != 4 {
		t.Fatalf("expected 4 attempts (2 cycles x 2 attempts), got %d", attempts)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil {
		t.Fatal("infra failures should not mark queue entry failed under continue policy")
	}
	if after[0].Consumed {
		t.Fatal("infra failures should not mark queue entry consumed under continue policy")
	}
}

func TestRPILoop_TaskFailure_MarksQueueFailed(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-task",
		Items:      []nextWorkItem{{Title: "Task failing goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		return wrapCycleFailure(cycleFailureTask, "phased engine", fmt.Errorf("validation failed"))
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected loop to fail under failure-policy=stop")
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt == nil || after[0].Items[0].FailedAt == nil {
		t.Fatal("task failures should record failed_at for the item")
	}
	if after[0].Consumed {
		t.Fatal("failed queue entry should remain unconsumed")
	}
	if after[0].ClaimStatus != "available" || after[0].Items[0].ClaimStatus != "available" {
		t.Fatal("failed queue item should be released back to available state")
	}
}

func TestRPILoop_TaskFailure_ContinuePolicy_AdvancesAfterFailingEntry(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-task-continue-fail",
			Items:      []nextWorkItem{{Title: "Task failing goal", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-task-continue-pass",
			Items:      []nextWorkItem{{Title: "Task succeeding goal", Severity: "medium"}},
			Consumed:   false,
		},
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyContinue
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	var goals []string
	runRPISupervisedCycleFn = func(_ context.Context, _ string, goal string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		goals = append(goals, goal)
		if goal == "Task failing goal" {
			return wrapCycleFailure(cycleFailureTask, "phased engine", fmt.Errorf("intentional task failure"))
		}
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, nil)
	})
	if err != nil {
		t.Fatalf("expected nil error under continue policy, got: %v", err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected 2 cycle executions, got %d (%v)", len(goals), goals)
	}
	if goals[0] != "Task failing goal" || goals[1] != "Task succeeding goal" {
		t.Fatalf("unexpected goal progression: %v", goals)
	}
	if !strings.Contains(output, "RPI loop finished after 2 cycle(s).") {
		t.Fatalf("expected 2-cycle summary, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt == nil || after[0].Items[0].FailedAt == nil {
		t.Fatal("task failure should record failed_at on first queue item")
	}
	if after[0].Consumed {
		t.Fatal("failed queue entry should remain unconsumed")
	}
	if !after[1].Consumed {
		t.Fatal("second queue entry should be consumed after continuing")
	}
}

func TestRPILoop_TaskFailure_StopPolicy_DoesNotAdvanceQueue(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-task-stop-fail",
			Items:      []nextWorkItem{{Title: "Task failing goal", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-task-stop-next",
			Items:      []nextWorkItem{{Title: "Never reached goal", Severity: "medium"}},
			Consumed:   false,
		},
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	var goals []string
	runRPISupervisedCycleFn = func(_ context.Context, _ string, goal string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		goals = append(goals, goal)
		return wrapCycleFailure(cycleFailureTask, "phased engine", fmt.Errorf("intentional task failure"))
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected error under stop policy")
	}
	if len(goals) != 1 {
		t.Fatalf("expected one cycle attempt before stop, got %d (%v)", len(goals), goals)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt == nil || after[0].Items[0].FailedAt == nil {
		t.Fatal("first queue item should be marked failed")
	}
	if after[1].FailedAt != nil || after[1].Consumed {
		t.Fatal("second queue entry should be untouched when stop policy is active")
	}
}

func TestRPILoop_TaskFailure_ContinuePolicy_AdvancesToSiblingItemInSameEntry(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic:  "ag-task-batch",
			ClaimStatus: "available",
			Items: []nextWorkItem{
				{Title: "Failing item", Severity: "high"},
				{Title: "Fresh sibling", Severity: "medium"},
			},
		},
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyContinue
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	var goals []string
	runRPISupervisedCycleFn = func(_ context.Context, _ string, goal string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		goals = append(goals, goal)
		if goal == "Failing item" {
			return wrapCycleFailure(cycleFailureTask, "phased engine", fmt.Errorf("intentional task failure"))
		}
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, nil)
	})
	if err != nil {
		t.Fatalf("expected nil error under continue policy, got: %v", err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected 2 cycle executions, got %d (%v)", len(goals), goals)
	}
	if goals[0] != "Failing item" || goals[1] != "Fresh sibling" {
		t.Fatalf("unexpected goal progression: %v", goals)
	}
	if !strings.Contains(output, "RPI loop finished after 2 cycle(s).") {
		t.Fatalf("expected 2-cycle summary, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].Items[0].FailedAt == nil {
		t.Fatal("failing item should record failed_at")
	}
	if after[0].Items[0].Consumed {
		t.Fatal("failing item should remain unconsumed")
	}
	if after[0].Items[0].ClaimStatus != "available" {
		t.Fatalf("failing item claim_status = %q, want available", after[0].Items[0].ClaimStatus)
	}
	if !after[0].Items[1].Consumed {
		t.Fatal("fresh sibling should be consumed after continue policy advances")
	}
	if after[0].Consumed {
		t.Fatal("batch entry should remain unconsumed until all sibling items are done")
	}
}

func TestRPILoop_KillSwitchDuringRetry_StopsWithoutQueueMutation(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-kill-switch",
		Items:      []nextWorkItem{{Title: "Kill switch retry goal", Severity: "high"}},
		Consumed:   false,
	})

	killPath := filepath.Join(tmpDir, ".agents", "rpi", "KILL")
	if err := os.MkdirAll(filepath.Dir(killPath), 0755); err != nil {
		t.Fatalf("mkdir kill-switch dir: %v", err)
	}

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 2
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute
	rpiKillSwitchPath = killPath

	attempts := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		if attempts == 1 {
			if err := os.WriteFile(killPath, []byte("stop\n"), 0644); err != nil {
				t.Fatalf("write kill switch: %v", err)
			}
		}
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", fmt.Errorf("transient infra failure"))
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, nil)
	})
	if err != nil {
		t.Fatalf("expected kill-switch exit to return nil, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one attempt before kill-switch stop, got %d", attempts)
	}
	if !strings.Contains(output, "Stopping loop before cycle execution") {
		t.Fatalf("expected kill-switch stop message, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil {
		t.Fatal("kill-switch interruption should not mark queue entry failed")
	}
	if after[0].Consumed {
		t.Fatal("kill-switch interruption should not consume queue entry")
	}
	if after[0].ClaimStatus == "in_progress" || after[0].Items[0].ClaimStatus == "in_progress" {
		t.Fatal("kill-switch interruption should release any in-progress claim")
	}
}

func TestRPILoop_ExplicitGoalReportsExecutedCycles(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiMaxCycles = 0
	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, []string{"count cycles"})
	})
	if err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if !strings.Contains(output, "Explicit goal completed.") {
		t.Fatalf("expected explicit goal completion message, got:\n%s", output)
	}
	if !strings.Contains(output, "RPI loop finished after 1 cycle(s).") {
		t.Fatalf("expected cycle count message, got:\n%s", output)
	}
}

func TestRPILoop_PreflightCompletedRunConsumesStaleItemAndAdvances(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-stale",
			Items:      []nextWorkItem{{Title: "Already done goal", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-fresh",
			Items:      []nextWorkItem{{Title: "Fresh goal", Severity: "medium"}},
			Consumed:   false,
		},
	})
	writeCompletedLoopRegistryRun(t, tmpDir, "run-stale", "ag-stale", "Already done goal")

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	var goals []string
	runRPISupervisedCycleFn = func(_ context.Context, _ string, goal string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		goals = append(goals, goal)
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, nil)
	})
	if err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if len(goals) != 1 || goals[0] != "Fresh goal" {
		t.Fatalf("expected only fresh goal to execute, got %v", goals)
	}
	if !strings.Contains(output, `Queue preflight consumed "Already done goal": matched completed RPI run run-stale`) {
		t.Fatalf("expected completed-run preflight consume message, got:\n%s", output)
	}
	if !strings.Contains(output, "RPI loop finished after 1 cycle(s).") {
		t.Fatalf("expected executed cycle count to stay at 1, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if !after[0].Consumed || !after[0].Items[0].Consumed {
		t.Fatalf("expected stale entry consumed, got %+v", after[0])
	}
	if after[0].Items[0].ConsumedBy == nil || *after[0].Items[0].ConsumedBy != queuePreflightConsumedBy {
		t.Fatalf("expected stale item consumed_by preflight marker, got %+v", after[0].Items[0])
	}
	if !after[1].Consumed || !after[1].Items[0].Consumed {
		t.Fatalf("expected fresh entry consumed after execution, got %+v", after[1])
	}
}

func TestRPILoop_PreflightEvidenceOnlyClosureConsumesStaleItemAndAdvances(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-parent",
			Items: []nextWorkItem{
				{
					Title:       "Already proven item",
					Severity:    "high",
					Description: "See .agents/releases/evidence-only-closures/ag-proof.2.json.",
				},
				{Title: "Fresh sibling", Severity: "medium"},
			},
			Consumed: false,
		},
	})
	writeEvidenceOnlyClosurePacket(t, tmpDir, "ag-proof.2")

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	var goals []string
	runRPISupervisedCycleFn = func(_ context.Context, _ string, goal string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		goals = append(goals, goal)
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, nil)
	})
	if err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if len(goals) != 1 || goals[0] != "Fresh sibling" {
		t.Fatalf("expected only fresh sibling to execute after proof-backed preflight, got %v", goals)
	}
	if !strings.Contains(output, `Queue preflight consumed "Already proven item": matched evidence-only closure proof for ag-proof.2`) {
		t.Fatalf("expected evidence-only preflight consume message, got:\n%s", output)
	}

	after := readJSONLEntries(t, queuePath)
	if !after[0].Items[0].Consumed {
		t.Fatalf("expected proof-backed sibling consumed, got %+v", after[0].Items[0])
	}
	if after[0].Items[0].ConsumedBy == nil || *after[0].Items[0].ConsumedBy != queuePreflightConsumedBy {
		t.Fatalf("expected stale sibling consumed_by preflight marker, got %+v", after[0].Items[0])
	}
	if !after[0].Items[1].Consumed {
		t.Fatalf("expected fresh sibling to execute and be consumed, got %+v", after[0].Items[1])
	}
	if !after[0].Consumed {
		t.Fatalf("expected parent batch to be consumed after both sibling items resolved, got %+v", after[0])
	}
}

func TestRPILoop_KillSwitchStopsBeforeCycleExecution(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	killPath := filepath.Join(tmpDir, ".agents", "rpi", "KILL")
	if err := os.MkdirAll(filepath.Dir(killPath), 0755); err != nil {
		t.Fatalf("mkdir kill switch dir: %v", err)
	}
	if err := os.WriteFile(killPath, []byte("stop\n"), 0644); err != nil {
		t.Fatalf("write kill switch: %v", err)
	}

	rpiMaxCycles = 1
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute
	rpiKillSwitchPath = killPath

	attempts := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		return nil
	}

	output, err := captureStdout(t, func() error {
		return runRPILoop(nil, []string{"stopped by kill switch"})
	})
	if err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if attempts != 0 {
		t.Fatalf("expected kill switch to stop loop before execution; attempts=%d", attempts)
	}
	if !strings.Contains(output, "Kill switch detected") {
		t.Fatalf("expected kill switch message, got:\n%s", output)
	}
	if !strings.Contains(output, "RPI loop finished after 0 cycle(s).") {
		t.Fatalf("expected zero cycle summary, got:\n%s", output)
	}
}

func TestRPILoop_CompileCadence_RunsOncePerInterval(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevCompileTick := runCompileProducerTickFn
	defer func() { runCompileProducerTickFn = prevCompileTick }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	writeJSONL(t, queuePath, []nextWorkEntry{
		{
			SourceEpic: "ag-compile-1",
			Items:      []nextWorkItem{{Title: "Compile cadence goal 1", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-compile-2",
			Items:      []nextWorkItem{{Title: "Compile cadence goal 2", Severity: "medium"}},
			Consumed:   false,
		},
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute
	rpiCompile = true
	rpiCompileInterval = time.Hour
	rpiCompileSince = "26h"
	rpiCompileDefrag = false

	compileTicks := 0
	runCompileProducerTickFn = func(_ string, cfg rpiLoopSupervisorConfig) error {
		compileTicks++
		if cfg.CompileSince != "26h" {
			t.Fatalf("CompileSince = %q, want 26h", cfg.CompileSince)
		}
		return nil
	}

	executed := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		executed++
		return nil
	}

	if err := runRPILoop(nil, nil); err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if executed != 2 {
		t.Fatalf("expected 2 cycle executions, got %d", executed)
	}
	if compileTicks != 1 {
		t.Fatalf("expected one Compile producer tick due to interval gating, got %d", compileTicks)
	}

	after := readJSONLEntries(t, queuePath)
	if len(after) != 2 || !after[0].Consumed || !after[1].Consumed {
		t.Fatalf("expected both queue entries consumed, got: %+v", after)
	}
}

func TestRPILoop_CompileCadence_ProducerFailure_ContinuePolicy(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevCompileTick := runCompileProducerTickFn
	defer func() { runCompileProducerTickFn = prevCompileTick }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-compile-continue",
		Items:      []nextWorkItem{{Title: "Compile continue goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyContinue
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute
	rpiCompile = true
	rpiCompileInterval = 0
	rpiCompileSince = "26h"
	rpiCompileDefrag = false

	runCompileProducerTickFn = func(_ string, _ rpiLoopSupervisorConfig) error {
		return fmt.Errorf("mine unavailable")
	}

	executed := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		executed++
		return nil
	}

	if err := runRPILoop(nil, nil); err != nil {
		t.Fatalf("expected nil error under continue policy, got: %v", err)
	}
	if executed != 1 {
		t.Fatalf("expected one cycle execution after Compile producer failure, got %d", executed)
	}

	after := readJSONLEntries(t, queuePath)
	if !after[0].Consumed {
		t.Fatal("queue item should still be consumed under continue policy")
	}
}

func TestRPILoop_CompileCadence_ProducerFailure_StopPolicy(t *testing.T) {
	clearRPIEnv(t)
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevCompileTick := runCompileProducerTickFn
	defer func() { runCompileProducerTickFn = prevCompileTick }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-compile-stop",
		Items:      []nextWorkItem{{Title: "Compile stop goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute
	rpiCompile = true
	rpiCompileInterval = 0
	rpiCompileSince = "26h"
	rpiCompileDefrag = false

	runCompileProducerTickFn = func(_ string, _ rpiLoopSupervisorConfig) error {
		return fmt.Errorf("mine unavailable")
	}

	executed := 0
	runRPISupervisedCycleFn = func(_ context.Context, _ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		executed++
		return nil
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected stop policy to return producer failure")
	}
	if !strings.Contains(err.Error(), "compile producer") {
		t.Fatalf("expected compile producer error context, got: %v", err)
	}
	if executed != 0 {
		t.Fatalf("expected zero cycle executions when producer fails under stop policy, got %d", executed)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil || after[0].Consumed {
		t.Fatalf("queue should remain unmodified when producer fails before cycle execution: %+v", after[0])
	}
}

func setupSingleQueueEntry(t *testing.T, tmpDir string, entry nextWorkEntry) string {
	t.Helper()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(queuePath, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}
	return queuePath
}

// ---- phasedEngineOptions defaults ----

func TestDefaultPhasedEngineOptions(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	if opts.From != "discovery" {
		t.Errorf("expected From=discovery, got %q", opts.From)
	}
	if opts.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", opts.MaxRetries)
	}
	if !opts.SwarmFirst {
		t.Errorf("expected SwarmFirst=true")
	}
	if opts.PhaseTimeout == 0 {
		t.Errorf("expected non-zero PhaseTimeout")
	}
}

func TestMarkItemConsumed_SingleItem(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "test",
		Timestamp:  "2026-03-01T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
			{Title: "Item B", Severity: "medium"},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0644)

	if err := markItemConsumed(path, 0, 0, "test-agent"); err != nil {
		t.Fatalf("markItemConsumed error: %v", err)
	}

	raw, _ := os.ReadFile(path)
	var result nextWorkEntry
	json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &result)

	if !result.Items[0].Consumed {
		t.Error("expected item 0 consumed=true")
	}
	if result.Items[1].Consumed {
		t.Error("expected item 1 consumed=false")
	}
	if result.Consumed {
		t.Error("entry should not be consumed when only 1 of 2 items consumed")
	}
}

func TestMarkItemConsumed_AllItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "test",
		Timestamp:  "2026-03-01T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
			{Title: "Item B", Severity: "medium"},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0644)

	markItemConsumed(path, 0, 0, "test-agent")
	markItemConsumed(path, 0, 1, "test-agent")

	raw, _ := os.ReadFile(path)
	var result nextWorkEntry
	json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &result)

	if !result.Items[0].Consumed || !result.Items[1].Consumed {
		t.Error("expected both items consumed=true")
	}
	if !result.Consumed {
		t.Error("entry should be consumed when all items are consumed")
	}
	if result.ConsumedBy == nil || *result.ConsumedBy != "test-agent" {
		t.Errorf("expected consumed_by=test-agent, got %v", result.ConsumedBy)
	}
}

func TestSelectHighestSeverityEntry_SkipsConsumedItems(t *testing.T) {
	entries := []nextWorkEntry{
		{
			SourceEpic: "test",
			QueueIndex: 0,
			Items: []nextWorkItem{
				{Title: "Consumed high", Severity: "high", Consumed: true},
				{Title: "Available medium", Severity: "medium"},
			},
		},
	}

	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected a selection, got nil")
	}
	if sel.Item.Title != "Available medium" {
		t.Errorf("expected 'Available medium', got %q", sel.Item.Title)
	}
	if sel.ItemIndex != 1 {
		t.Errorf("expected ItemIndex=1, got %d", sel.ItemIndex)
	}
}

func TestSelectHighestSeverityEntry_SkipsClaimedItems(t *testing.T) {
	entries := []nextWorkEntry{
		{
			SourceEpic: "test",
			QueueIndex: 0,
			Items: []nextWorkItem{
				{Title: "Claimed high", Severity: "high", ClaimStatus: "in_progress"},
				{Title: "Available medium", Severity: "medium"},
			},
		},
	}

	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected a selection, got nil")
	}
	if sel.Item.Title != "Available medium" {
		t.Errorf("expected 'Available medium', got %q", sel.Item.Title)
	}
}

func TestReadUnconsumedItems_SkipsConsumedItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "test",
		Timestamp:  "2026-03-01T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Consumed", Severity: "high", Consumed: true},
			{Title: "Available", Severity: "medium"},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0644)

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Available" {
		t.Errorf("expected 'Available', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_SkipsClaimedItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "test",
		Timestamp:  "2026-03-01T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Claimed", Severity: "high", ClaimStatus: "in_progress"},
			{Title: "Available", Severity: "medium"},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0644)

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Available" {
		t.Errorf("expected 'Available', got %q", items[0].Title)
	}
}

func TestReadQueueEntries_SkipsAllItemsConsumedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "test",
		Timestamp:  "2026-03-01T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "A", Severity: "high", Consumed: true},
			{Title: "B", Severity: "medium", Consumed: true},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0644)

	entries, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (all items consumed), got %d", len(entries))
	}
}

func TestRecomputeEntryLifecycle_EmptyItems(t *testing.T) {
	entry := &nextWorkEntry{
		SourceEpic:  "ag-test",
		Consumed:    true,
		ClaimStatus: "consumed",
	}
	recomputeEntryLifecycle(entry)
	// With no items, the function returns early without modifying the entry.
	if entry.ClaimStatus != "consumed" {
		t.Errorf("expected claim_status to remain 'consumed', got %q", entry.ClaimStatus)
	}
	if !entry.Consumed {
		t.Error("expected Consumed to remain true for empty-items entry")
	}
}

func TestRecomputeEntryLifecycle_FailedAtPropagated(t *testing.T) {
	failedTime := "2026-03-01T10:00:00Z"
	entry := &nextWorkEntry{
		SourceEpic: "ag-test",
		Items: []nextWorkItem{
			{Title: "item1", Consumed: true, ClaimStatus: "consumed"},
			{Title: "item2", Consumed: false, ClaimStatus: "available", FailedAt: &failedTime},
		},
	}
	recomputeEntryLifecycle(entry)
	if entry.FailedAt == nil || *entry.FailedAt != failedTime {
		t.Errorf("expected FailedAt=%q, got %v", failedTime, entry.FailedAt)
	}
	if entry.Consumed {
		t.Error("expected Consumed=false when not all items consumed")
	}
	if entry.ClaimStatus != "available" {
		t.Errorf("expected claim_status='available', got %q", entry.ClaimStatus)
	}
}

func TestCancelableSleep_NormalExpiry(t *testing.T) {
	err := cancelableSleep(context.Background(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCancelableSleep_ImmediateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cancelableSleep(ctx, time.Second)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCancelableSleep_CancelDuringSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := cancelableSleep(ctx, time.Second)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestApplyCycleDelay_KillSwitchBeforeSleep(t *testing.T) {
	tmpDir := t.TempDir()
	killPath := filepath.Join(tmpDir, "KILL")
	if err := os.WriteFile(killPath, []byte("stop"), 0644); err != nil {
		t.Fatalf("create kill switch: %v", err)
	}

	cfg := rpiLoopSupervisorConfig{
		KillSwitchPath: killPath,
		CycleDelay:     5 * time.Second,
	}

	start := time.Now()
	stop, err := applyCycleDelay(context.Background(), 2, cfg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !stop {
		t.Fatal("expected stop=true when kill switch is set")
	}
	if elapsed > time.Second {
		t.Fatalf("kill switch did not short-circuit sleep; elapsed %s", elapsed)
	}
}

func TestCompactNextWorkFile_RemovesOldConsumed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")

	oldTime := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	recentTime := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)

	entries := []nextWorkEntry{
		{
			SourceEpic: "epic-1", Consumed: true, ClaimStatus: "consumed",
			ConsumedAt: &oldTime,
			Items: []nextWorkItem{
				{Title: "old-item", Consumed: true, ClaimStatus: "consumed", ConsumedAt: &oldTime},
			},
		},
		{
			SourceEpic: "epic-2", Consumed: true, ClaimStatus: "consumed",
			ConsumedAt: &recentTime,
			Items: []nextWorkItem{
				{Title: "recent-item", Consumed: true, ClaimStatus: "consumed", ConsumedAt: &recentTime},
			},
		},
		{
			SourceEpic: "epic-3", Consumed: false,
			Items: []nextWorkItem{
				{Title: "open-item"},
			},
		},
	}
	writeJSONL(t, path, entries)

	n, err := compactNextWorkFile(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 compacted entry, got %d", n)
	}

	remaining := readJSONLEntries(t, path)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining entries, got %d", len(remaining))
	}
	if remaining[0].SourceEpic != "epic-2" {
		t.Errorf("expected first remaining entry epic-2, got %s", remaining[0].SourceEpic)
	}
	if remaining[1].SourceEpic != "epic-3" {
		t.Errorf("expected second remaining entry epic-3, got %s", remaining[1].SourceEpic)
	}
}

func TestCompactNextWorkFile_KeepsAllRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")

	recentTime := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)

	entries := []nextWorkEntry{
		{
			SourceEpic: "epic-1", Consumed: true, ClaimStatus: "consumed",
			ConsumedAt: &recentTime,
			Items: []nextWorkItem{
				{Title: "item-1", Consumed: true, ClaimStatus: "consumed", ConsumedAt: &recentTime},
			},
		},
		{
			SourceEpic: "epic-2", Consumed: true, ClaimStatus: "consumed",
			ConsumedAt: &recentTime,
			Items: []nextWorkItem{
				{Title: "item-2", Consumed: true, ClaimStatus: "consumed", ConsumedAt: &recentTime},
			},
		},
	}
	writeJSONL(t, path, entries)

	n, err := compactNextWorkFile(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 compacted entries, got %d", n)
	}

	remaining := readJSONLEntries(t, path)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 entries to remain, got %d", len(remaining))
	}
}

func TestCompactNextWorkFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	n, err := compactNextWorkFile(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 compacted entries, got %d", n)
	}
}

func TestMaybeCompactQueue_RunsAtInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")

	oldTime := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)

	entries := []nextWorkEntry{
		{
			SourceEpic: "epic-1", Consumed: true, ClaimStatus: "consumed",
			ConsumedAt: &oldTime,
			Items: []nextWorkItem{
				{Title: "old-item", Consumed: true, ClaimStatus: "consumed", ConsumedAt: &oldTime},
			},
		},
	}
	writeJSONL(t, path, entries)

	// cycle=5, interval=10 — should NOT run compaction.
	maybeCompactQueue(path, 5, 10, 24*time.Hour)
	remaining := readJSONLEntries(t, path)
	if len(remaining) != 1 {
		t.Fatalf("expected compaction to NOT run at cycle 5, but got %d entries", len(remaining))
	}

	// cycle=10, interval=10 — should run compaction.
	maybeCompactQueue(path, 10, 10, 24*time.Hour)
	remaining = readJSONLEntries(t, path)
	if len(remaining) != 0 {
		t.Fatalf("expected compaction to run at cycle 10, but got %d entries", len(remaining))
	}
}

// ---------------------------------------------------------------------------
// entryConsumedTime
// ---------------------------------------------------------------------------

func TestEntryConsumedTime_EntryLevel(t *testing.T) {
	ts := "2026-04-01T10:00:00Z"
	entry := &nextWorkEntry{ConsumedAt: &ts}
	got := entryConsumedTime(entry)
	want, _ := time.Parse(time.RFC3339, ts)
	if !got.Equal(want) {
		t.Errorf("entryConsumedTime = %v, want %v", got, want)
	}
}

func TestEntryConsumedTime_ItemLevel(t *testing.T) {
	ts1 := "2026-04-01T10:00:00Z"
	ts2 := "2026-04-02T10:00:00Z"
	entry := &nextWorkEntry{
		Items: []nextWorkItem{
			{ConsumedAt: &ts1},
			{ConsumedAt: &ts2},
		},
	}
	got := entryConsumedTime(entry)
	want, _ := time.Parse(time.RFC3339, ts2)
	if !got.Equal(want) {
		t.Errorf("entryConsumedTime = %v, want %v (should be latest)", got, want)
	}
}

func TestEntryConsumedTime_NoTimestamp(t *testing.T) {
	entry := &nextWorkEntry{}
	got := entryConsumedTime(entry)
	if !got.IsZero() {
		t.Errorf("entryConsumedTime = %v, want zero", got)
	}
}

func TestEntryConsumedTime_InvalidFormat(t *testing.T) {
	ts := "not-a-date"
	entry := &nextWorkEntry{ConsumedAt: &ts}
	got := entryConsumedTime(entry)
	if !got.IsZero() {
		t.Errorf("entryConsumedTime with invalid format = %v, want zero", got)
	}
}
