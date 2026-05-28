// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextExplainCmdJSONOutput(t *testing.T) {
	resetCommandState(t)
	dir := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "learnings"),
		filepath.Join(".agents", "patterns"),
		filepath.Join(".agents", "rpi"),
		filepath.Join(".agents", "packets", "promoted"),
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, ".agents", "findings", "f-explain-001.md"), []byte(`---
id: "f-explain-001"
title: "Explain ranked startup context"
status: "active"
severity: "high"
applicable_when: ["startup","task"]
scope_tags: ["context","startup"]
---
# Finding

Show why startup context was selected.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "planning-rules", "f-explain-001.md"), []byte(`---
id: "f-explain-001"
---
# Planning Rule

- Ask: Did the payload explain why context was selected?
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "pre-mortem-checks", "f-explain-001.md"), []byte(`---
id: "f-explain-001"
---
# Pre-Mortem Check

- Ask: Did the payload explain why packet families were suppressed?
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "learnings", "startup-learning.md"), []byte(`---
id: startup-learning
type: learning
date: 2026-04-01
source: test
maturity: provisional
utility: 0.9
---

# Explain startup context

Explain why ranked startup context was selected.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "patterns", "startup-pattern.md"), []byte(`# Startup payloads

Keep startup payloads small and explainable.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(`{"source_epic":"ag-amg","timestamp":"2026-04-01T22:00:00Z","items":[{"title":"Add explainability diagnostics","type":"feature","severity":"high","source":"research","description":"Explain why runtime context was selected","target_repo":"`+detectRepoName(dir)+`","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "packets", "promoted", "packet-a.md"), []byte("# Packet"), 0o644); err != nil {
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
	oldOutput := output
	output = "table"
	t.Cleanup(func() { output = oldOutput })
	contextExplainFlags.task = ""
	contextExplainFlags.phase = "task"
	contextExplainFlags.limit = defaultStigmergicPacketLimit
	cmd.SetArgs([]string{"-o", "json", "context", "explain", "--task", "explain startup context", "--phase", "startup"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, buf.String())
	}

	var result contextExplainResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}
	if result.Phase != "startup" {
		t.Fatalf("Phase = %q, want startup", result.Phase)
	}
	if len(result.Selected) == 0 {
		t.Fatal("expected selected items")
	}
	if len(result.Health) == 0 {
		t.Fatal("expected health diagnostics")
	}
	if !containsSelectionClass(result.Selected, "planning-rule") {
		t.Fatalf("expected planning-rule selection, got %+v", result.Selected)
	}
	if !containsSelectionClassWithReason(result.Selected, "next-work", "Selected from the backlog by repo affinity, severity, and query overlap.") {
		t.Fatalf("expected generic next-work selection reason, got %+v", result.Selected)
	}
	if !containsSuppressionClass(result.Suppressed, "promoted-packets") {
		t.Fatalf("expected promoted-packets suppression, got %+v", result.Suppressed)
	}
}

func TestContextExplainReportsProofBackedNextWorkSuppression(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "rpi"),
		filepath.Join(".agents", "releases", "evidence-only-closures"),
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeCompletedLoopRegistryRun(t, dir, "run-complete", "ag-complete", "Complete via run")
	writeEvidenceOnlyClosurePacket(t, dir, "ag-proof.2")
	writeCompletedLoopRegistryRun(t, dir, "run-exec", "ag-exec", "Complete via packet")

	queue := `{"source_epic":"ag-complete","timestamp":"2026-04-01T22:00:00Z","items":[{"title":"Already done by run","type":"task","severity":"high","source":"council-finding","description":"closed elsewhere","target_repo":"` + detectRepoName(dir) + `","proof_ref":{"kind":"completed_run","run_id":"run-complete"},"consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
{"source_epic":"ag-proof","timestamp":"2026-04-01T22:00:01Z","items":[{"title":"Already done by closure","type":"task","severity":"high","source":"council-finding","description":"closed elsewhere","target_repo":"` + detectRepoName(dir) + `","proof_ref":{"kind":"evidence_only_closure","target_id":"ag-proof.2"},"consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
{"source_epic":"ag-exec","timestamp":"2026-04-01T22:00:02Z","items":[{"title":"Already done by packet","type":"task","severity":"high","source":"council-finding","description":"closed elsewhere","target_repo":"` + detectRepoName(dir) + `","proof_ref":{"kind":"execution_packet","run_id":"run-exec"},"consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
{"source_epic":"ag-open","timestamp":"2026-04-01T22:00:03Z","items":[{"title":"Still open","type":"task","severity":"high","source":"council-finding","description":"needs work","target_repo":"` + detectRepoName(dir) + `","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(dir, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
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

	bundle := collectRankedContextBundle(dir, "proof-backed next work", defaultStigmergicPacketLimit)
	result := buildContextExplainResult(dir, detectRepoName(dir), "proof-backed next work", "task", bundle)

	if !containsSelectionClassWithReason(result.Selected, "next-work", "Selected from the backlog by repo affinity, severity, and query overlap.") {
		t.Fatalf("expected generic selected next-work item, got %+v", result.Selected)
	}
	for _, want := range []string{"completed-run proof", "evidence-only-closure proof", "execution-packet proof"} {
		if !containsSuppressionReason(result.Suppressed, want) {
			t.Fatalf("expected suppression reason containing %q, got %+v", want, result.Suppressed)
		}
	}
}

func TestContextExplainCmdHumanOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "rpi"), 0o755); err != nil {
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
	oldOutput := output
	output = "table"
	t.Cleanup(func() { output = oldOutput })
	contextExplainFlags.task = ""
	contextExplainFlags.phase = "task"
	contextExplainFlags.limit = defaultStigmergicPacketLimit
	cmd.SetArgs([]string{"context", "explain", "--task", "empty payload"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, buf.String())
	}

	output := buf.String()
	for _, section := range []string{"## Context Explain", "## Packet Health", "## Selected", "## Suppressed"} {
		if !strings.Contains(output, section) {
			t.Fatalf("output missing %q:\n%s", section, output)
		}
	}
}

func containsSelectionClass(items []contextExplainSelection, class string) bool {
	for _, item := range items {
		if item.Class == class {
			return true
		}
	}
	return false
}

func containsSelectionClassWithReason(items []contextExplainSelection, class, reason string) bool {
	for _, item := range items {
		if item.Class == class && strings.Contains(item.Reason, reason) {
			return true
		}
	}
	return false
}

func containsSuppressionClass(items []contextExplainSuppression, class string) bool {
	for _, item := range items {
		if item.Class == class {
			return true
		}
	}
	return false
}

func containsSuppressionReason(items []contextExplainSuppression, substring string) bool {
	for _, item := range items {
		if strings.Contains(item.Reason, substring) {
			return true
		}
	}
	return false
}
