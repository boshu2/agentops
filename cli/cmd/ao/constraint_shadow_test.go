package main

import (
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

func TestConstraintActivate_ShadowWithoutPrecisionEvidenceFails(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)
	entry := shadowConstraintEntry("c-unmeasured", search.ConstraintEvidence{
		PositiveRefs:        []string{"positive-a"},
		NegativeControlRefs: []string{"negative-a"},
	})
	if err := saveConstraintIndex(&constraintIndex{SchemaVersion: 1, Constraints: []constraintEntry{entry}}); err != nil {
		t.Fatal(err)
	}

	err := constraintActivateCmd.RunE(constraintActivateCmd, []string{entry.ID})
	if err == nil || !strings.Contains(err.Error(), "precision evidence") {
		t.Fatalf("activation error = %v, want missing precision evidence", err)
	}
	reloaded, _ := loadConstraintIndex()
	if got := findConstraint(reloaded, entry.ID).Status; got != "shadow" {
		t.Fatalf("failed activation changed status to %q, want shadow", got)
	}
}

func TestConstraintActivate_MeasuredShadowBecomesBlocking(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)
	evidence := search.ConstraintEvidence{
		PositiveRefs:         []string{"positive-a", "positive-b"},
		NegativeControlRefs:  []string{"negative-a", "negative-b"},
		PrecisionEvidenceRef: ".agents/evidence/constraints/c-ready.json",
		ShadowSamples:        40,
		TruePositives:        39,
		FalsePositives:       1,
	}
	entry := shadowConstraintEntry("c-ready", evidence)
	if err := saveConstraintIndex(&constraintIndex{SchemaVersion: 1, Constraints: []constraintEntry{entry}}); err != nil {
		t.Fatal(err)
	}

	if _, err := captureStdout(t, func() error {
		return constraintActivateCmd.RunE(constraintActivateCmd, []string{entry.ID})
	}); err != nil {
		t.Fatalf("activate measured shadow: %v", err)
	}
	reloaded, _ := loadConstraintIndex()
	got := findConstraint(reloaded, entry.ID)
	if got.Status != "active" || got.EnforcementMode != "block" {
		t.Fatalf("activated status/mode = %q/%q, want active/block", got.Status, got.EnforcementMode)
	}
}

func shadowConstraintEntry(id string, evidence search.ConstraintEvidence) constraintEntry {
	return constraintEntry{
		ID:              id,
		Title:           id,
		Status:          "shadow",
		EnforcementMode: "warn",
		CompiledAt:      time.Now().UTC().Format(time.RFC3339),
		Evidence:        evidence,
	}
}

func activationReadyConstraintEntry(id string) constraintEntry {
	return activationReadyConstraintEntryAt(id, time.Now().UTC().Format(time.RFC3339))
}

func activationReadyConstraintEntryAt(id, compiledAt string) constraintEntry {
	entry := shadowConstraintEntry(id, search.ConstraintEvidence{
		PositiveRefs:         []string{"positive-a", "positive-b"},
		NegativeControlRefs:  []string{"negative-a", "negative-b"},
		PrecisionEvidenceRef: ".agents/evidence/constraints/" + id + ".json",
		ShadowSamples:        40,
		TruePositives:        39,
		FalsePositives:       1,
	})
	entry.CompiledAt = compiledAt
	return entry
}
