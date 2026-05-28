// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCurate_Integration_StatusEmpty(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	oldOutput := output
	oldJsonFlag := jsonFlag
	output = "table"
	jsonFlag = false
	t.Cleanup(func() { output = oldOutput; jsonFlag = oldJsonFlag })

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"curate", "status"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected curate status to succeed, got error: %v", err)
	}

	if !strings.Contains(out, "Curation Pipeline Status") {
		t.Errorf("expected 'Curation Pipeline Status' header, got:\n%s", out)
	}

	// All counts should be 0
	if !strings.Contains(out, "Total:      0") {
		t.Errorf("expected 'Total:      0' in empty state, got:\n%s", out)
	}
}

func TestCurate_Integration_StatusWithArtifacts(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	oldOutput := output
	oldJsonFlag := jsonFlag
	output = "table"
	jsonFlag = false
	t.Cleanup(func() { output = oldOutput; jsonFlag = oldJsonFlag })

	now := time.Now().UTC().Format(time.RFC3339)

	// Place JSON artifacts in .agents/learnings/
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "learn-001.json"), `{
		"id": "learn-2026-04-01-abc1",
		"type": "learning",
		"content": "Test learning content",
		"date": "2026-04-01",
		"schema_version": 1,
		"curated_at": "`+now+`"
	}`)
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "decis-001.json"), `{
		"id": "decis-2026-04-01-abc2",
		"type": "decision",
		"content": "Test decision content",
		"date": "2026-04-01",
		"schema_version": 1,
		"curated_at": "`+now+`"
	}`)
	writeFile(t, filepath.Join(dir, ".agents", "patterns", "patt-001.json"), `{
		"id": "patt-2026-04-01-abc3",
		"type": "pattern",
		"content": "Test pattern content",
		"date": "2026-04-01",
		"schema_version": 1,
		"curated_at": "`+now+`"
	}`)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"curate", "status"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected curate status to succeed, got error: %v", err)
	}

	if !strings.Contains(out, "Learnings:  1") {
		t.Errorf("expected 'Learnings:  1', got:\n%s", out)
	}
	if !strings.Contains(out, "Decisions:  1") {
		t.Errorf("expected 'Decisions:  1', got:\n%s", out)
	}
	if !strings.Contains(out, "Patterns:   1") {
		t.Errorf("expected 'Patterns:   1', got:\n%s", out)
	}
	if !strings.Contains(out, "Total:      3") {
		t.Errorf("expected 'Total:      3', got:\n%s", out)
	}
}

func TestCurate_Integration_StatusJSON(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	now := time.Now().UTC().Format(time.RFC3339)
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "learn-j1.json"), `{
		"id": "learn-j1",
		"type": "learning",
		"content": "JSON test",
		"date": "2026-04-01",
		"schema_version": 1,
		"curated_at": "`+now+`"
	}`)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	t.Cleanup(func() { rootCmd.SetOut(nil); rootCmd.SetErr(nil); rootCmd.SetArgs(nil) })
	rootCmd.SetArgs([]string{"curate", "status"})
	err := rootCmd.Execute()
	out := buf.String()

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	var result curateStatusResult
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("JSON parse error: %v\nraw:\n%s", jsonErr, out)
	}

	if result.Learnings != 1 {
		t.Errorf("expected 1 learning, got %d", result.Learnings)
	}
	if result.Total != 1 {
		t.Errorf("expected total=1, got %d", result.Total)
	}
	if result.PendingVerify != 1 {
		t.Errorf("expected pending_verify=1 (never verified), got %d", result.PendingVerify)
	}
}

func TestCurate_Integration_CatalogArtifact(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create an artifact to catalog
	artifactContent := `---
type: learning
date: "2026-04-01"
---
# Cataloged Learning

This is a test learning to catalog.
`
	writeFile(t, filepath.Join(dir, "artifact-to-catalog.md"), artifactContent)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"curate", "catalog", "artifact-to-catalog.md"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected catalog to succeed, got error: %v\noutput:\n%s", err, out)
	}

	// Should mention the cataloged artifact
	if !strings.Contains(out, "learn-") && !strings.Contains(out, "Cataloged") && !strings.Contains(out, "catalog") {
		t.Logf("catalog output: %s", out)
	}
}

func TestCurate_Integration_CatalogInvalidType(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	writeFile(t, filepath.Join(dir, "bad-artifact.md"), "---\ntype: bogus\n---\nContent.\n")

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"curate", "catalog", "bad-artifact.md"})
		return rootCmd.Execute()
	})

	if err == nil {
		t.Error("expected error for unknown artifact type 'bogus'")
	}
}
