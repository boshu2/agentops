package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestLegacyCaptureToInjectAndFeedbackE2E(t *testing.T) {
	fixture := setupLegacyCaptureKnowledgeLoopFixture(t)
	p, pendingPath := migrateAndIngestLegacyCapture(t, fixture)
	recordLegacyCapturePromotionCitation(t, fixture.tmp, pendingPath)
	artifactPath := autoPromoteLegacyCaptureArtifact(t, p)

	assertLegacyCaptureLearningRetrievable(t, fixture.tmp)
	processLegacyCaptureFeedback(t, fixture.tmp, artifactPath)
	assertLegacyCaptureUtilityUpdated(t, artifactPath)
}

type legacyCaptureKnowledgeLoopFixture struct {
	tmp        string
	sourceDir  string
	pendingDir string
}

func setupLegacyCaptureKnowledgeLoopFixture(t *testing.T) legacyCaptureKnowledgeLoopFixture {
	t.Helper()

	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, ".agents", "knowledge")
	pendingDir := filepath.Join(sourceDir, "pending")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}

	legacyCapture := `---
type: process
source: manual
confidence: high
date: 2026-01-01
---

# Prefer command -v for tool detection

Use command -v before assuming a binary is missing from PATH.
`
	legacyPath := filepath.Join(sourceDir, "2026-01-01-legacy.md")
	if err := os.WriteFile(legacyPath, []byte(legacyCapture), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	return legacyCaptureKnowledgeLoopFixture{
		tmp:        tmp,
		sourceDir:  sourceDir,
		pendingDir: pendingDir,
	}
}

func migrateAndIngestLegacyCapture(t *testing.T, fixture legacyCaptureKnowledgeLoopFixture) (*pool.Pool, string) {
	t.Helper()

	migrateRes, err := migrateLegacyKnowledgeFiles(fixture.sourceDir, fixture.pendingDir)
	if err != nil {
		t.Fatalf("migrate legacy captures: %v", err)
	}
	if migrateRes.Moved != 1 || len(migrateRes.Moves) != 1 {
		t.Fatalf("unexpected migrate result: %+v", migrateRes)
	}

	ingestRes, err := ingestPendingFilesToPool(fixture.tmp, []string{migrateRes.Moves[0].To})
	if err != nil {
		t.Fatalf("ingest pending: %v", err)
	}
	if ingestRes.Added != 1 {
		t.Fatalf("expected one candidate ingested, got %+v", ingestRes)
	}

	p := pool.NewPool(fixture.tmp)
	entries, err := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one pending candidate, got %d", len(entries))
	}

	return p, entries[0].FilePath
}

func recordLegacyCapturePromotionCitation(t *testing.T, tmp, pendingPath string) {
	t.Helper()

	// Promotion gates require citation evidence.
	if err := ratchet.RecordCitation(tmp, types.CitationEvent{
		ArtifactPath: pendingPath,
		SessionID:    "session-capture-promotion",
		CitedAt:      time.Now(),
		CitationType: "retrieved",
		Query:        "tool detection",
	}); err != nil {
		t.Fatalf("record promotion citation: %v", err)
	}
}

func autoPromoteLegacyCaptureArtifact(t *testing.T, p *pool.Pool) string {
	t.Helper()

	autoRes, err := autoPromoteAndPromoteToArtifacts(p, time.Hour, true)
	if err != nil {
		t.Fatalf("auto promote: %v", err)
	}
	if autoRes.Promoted != 1 || len(autoRes.Artifacts) != 1 {
		t.Fatalf("unexpected auto-promote result: %+v", autoRes)
	}

	return autoRes.Artifacts[0]
}

func assertLegacyCaptureLearningRetrievable(t *testing.T, tmp string) {
	t.Helper()

	learnings, err := collectLearnings(tmp, "tool", 10, "", 0)
	if err != nil {
		t.Fatalf("collect learnings: %v", err)
	}
	if len(learnings) == 0 {
		t.Fatal("expected promoted learning to be retrievable")
	}
}

func processLegacyCaptureFeedback(t *testing.T, tmp, artifactPath string) {
	t.Helper()

	citation := types.CitationEvent{
		ArtifactPath: artifactPath,
		SessionID:    "session-capture-feedback",
		CitedAt:      time.Now(),
		CitationType: "applied",
		Query:        "tool detection",
	}

	events, updatedCount, failedCount := processUniqueCitations(
		tmp,
		"session-capture-feedback",
		"",
		[]types.CitationEvent{citation},
		1.0,
		types.DefaultAlpha,
	)
	if updatedCount != 1 || failedCount != 0 || len(events) != 1 {
		t.Fatalf("unexpected feedback processing outcome: updated=%d failed=%d events=%d", updatedCount, failedCount, len(events))
	}
	if events[0].UtilityAfter <= events[0].UtilityBefore {
		t.Fatalf("utility should increase after positive feedback: before=%.3f after=%.3f", events[0].UtilityBefore, events[0].UtilityAfter)
	}
}

func assertLegacyCaptureUtilityUpdated(t *testing.T, artifactPath string) {
	t.Helper()

	updatedLearning, err := parseLearningFile(artifactPath)
	if err != nil {
		t.Fatalf("parse updated artifact: %v", err)
	}
	if updatedLearning.Utility <= types.InitialUtility {
		t.Fatalf("expected utility to be above baseline %.2f, got %.3f", types.InitialUtility, updatedLearning.Utility)
	}
}
