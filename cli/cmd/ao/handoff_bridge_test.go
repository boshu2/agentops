// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBridgeHandoffToLearnings_WithDecisions(t *testing.T) {
	tmp := t.TempDir()
	artifact := &handoffArtifact{
		ID:   "handoff-test-001",
		Type: "manual",
		Goal: "test goal",
		DecisionsMade: []string{
			"Use PostgreSQL for persistence",
			"Deploy to us-east-1",
			"Pin Go version to 1.22",
		},
	}

	if err := bridgeHandoffToLearnings(tmp, artifact); err != nil {
		t.Fatalf("bridgeHandoffToLearnings returned error: %v", err)
	}

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("failed to read learnings dir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 files, got %d", len(entries))
	}

	for i, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			t.Errorf("file %d: expected .md extension, got %s", i, name)
		}
		data, err := os.ReadFile(filepath.Join(learningsDir, name))
		if err != nil {
			t.Fatalf("failed to read file %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Errorf("file %s is empty", name)
		}
	}
}

func TestBridgeHandoffToLearnings_NoDecisions(t *testing.T) {
	tmp := t.TempDir()
	artifact := &handoffArtifact{
		ID:            "handoff-test-002",
		Type:          "manual",
		Goal:          "test goal",
		DecisionsMade: []string{},
	}

	if err := bridgeHandoffToLearnings(tmp, artifact); err != nil {
		t.Fatalf("bridgeHandoffToLearnings returned error: %v", err)
	}

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("failed to read learnings dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 files for empty decisions, got %d", len(entries))
	}
}

func TestBridgeHandoffToLearnings_FrontmatterValid(t *testing.T) {
	tmp := t.TempDir()
	artifact := &handoffArtifact{
		ID:            "handoff-test-003",
		Type:          "rpi",
		Goal:          "validate auth module",
		DecisionsMade: []string{"Use JWT tokens"},
	}

	if err := bridgeHandoffToLearnings(tmp, artifact); err != nil {
		t.Fatalf("bridgeHandoffToLearnings returned error: %v", err)
	}

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("failed to read learnings dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(learningsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	content := string(data)

	requiredFields := []string{
		"type: learning",
		"source: handoff-bridge",
		"date: ",
		"confidence: medium",
		"session_type: rpi",
		"maturity: provisional",
	}
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("frontmatter missing field: %q", field)
		}
	}

	if !strings.HasPrefix(content, "---\n") {
		t.Error("frontmatter should start with ---")
	}
	if !strings.Contains(content, "\n---\n") {
		t.Error("frontmatter should have closing ---")
	}

	if !strings.Contains(content, "# Decision: Use JWT tokens") {
		t.Error("missing decision heading")
	}
	if !strings.Contains(content, "handoff-test-003") {
		t.Error("missing artifact ID reference")
	}
	if !strings.Contains(content, "validate auth module") {
		t.Error("missing goal reference")
	}
}

func TestBridgeHandoffToLearnings_SessionType(t *testing.T) {
	tests := []struct {
		name        string
		sessionType string
	}{
		{"manual type", "manual"},
		{"rpi type", "rpi"},
		{"auto type", "auto"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			artifact := &handoffArtifact{
				ID:            "handoff-type-test",
				Type:          tc.sessionType,
				Goal:          "type test",
				DecisionsMade: []string{"some decision"},
			}

			if err := bridgeHandoffToLearnings(tmp, artifact); err != nil {
				t.Fatalf("bridgeHandoffToLearnings returned error: %v", err)
			}

			learningsDir := filepath.Join(tmp, ".agents", "learnings")
			entries, err := os.ReadDir(learningsDir)
			if err != nil {
				t.Fatalf("failed to read learnings dir: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 file, got %d", len(entries))
			}

			data, err := os.ReadFile(filepath.Join(learningsDir, entries[0].Name()))
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			expected := "session_type: " + tc.sessionType
			if !strings.Contains(string(data), expected) {
				t.Errorf("expected %q in content, got:\n%s", expected, string(data))
			}
		})
	}
}

func TestBridgeHandoffToLearnings_PreservesResearchSources(t *testing.T) {
	tmp := t.TempDir()
	artifact := &handoffArtifact{
		ID:            "handoff-test-004",
		Type:          "manual",
		Goal:          "Close the gap documented in .agents/research/2026-03-22-flywheel-gap.md",
		Summary:       "Use the research artifact directly instead of generic provenance labels",
		DecisionsMade: []string{"Adopt task-scoped startup retrieval"},
	}

	if err := bridgeHandoffToLearnings(tmp, artifact); err != nil {
		t.Fatalf("bridgeHandoffToLearnings returned error: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(tmp, ".agents", "learnings"))
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", "learnings", entries[0].Name()))
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "research_sources:") {
		t.Fatalf("expected research_sources frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, ".agents/research/2026-03-22-flywheel-gap.md") {
		t.Fatalf("expected exact research path, got:\n%s", content)
	}
}
