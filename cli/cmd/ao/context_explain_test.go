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
	if !containsSuppressionClass(result.Suppressed, "promoted-packets") {
		t.Fatalf("expected promoted-packets suppression, got %+v", result.Suppressed)
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

func containsSuppressionClass(items []contextExplainSuppression, class string) bool {
	for _, item := range items {
		if item.Class == class {
			return true
		}
	}
	return false
}
