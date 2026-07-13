package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
)

func TestProductionFindingCompiler_PassingReplayEmitsCanonicalPremortemAndWarnOnlyShadow(t *testing.T) {
	artifact := ports.FindingArtifact{
		ID:               "f-shadow",
		Frontmatter:      mechanicalFrontmatter(nil),
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
		Body:             "Avoid panic calls.",
	}
	outputs, err := newProductionFindingCompiler().Compile(context.Background(), artifact)
	if err != nil {
		t.Fatal(err)
	}
	var premortemPath string
	var constraint search.ConstraintEntry
	for _, output := range outputs {
		switch output.Kind {
		case ports.CompiledOutputPremortemCheck:
			premortemPath = output.Path
		case ports.CompiledOutputConstraint:
			if err := json.Unmarshal(output.Body, &constraint); err != nil {
				t.Fatal(err)
			}
		}
	}
	if premortemPath != ".agents/premortem-checks/f-shadow.md" {
		t.Fatalf("premortem path = %q, want canonical path", premortemPath)
	}
	if constraint.Status != "shadow" || constraint.EnforcementMode != "warn" {
		t.Fatalf("constraint status/mode = %q/%q, want shadow/warn", constraint.Status, constraint.EnforcementMode)
	}
	if len(constraint.Evidence.PositiveRefs) != 2 || len(constraint.Evidence.NegativeControlRefs) != 2 {
		t.Fatalf("constraint omitted replay evidence: %+v", constraint.Evidence)
	}
}

func loadDetectorEvidenceFixtureFromRepo(t *testing.T, name string) *ports.DetectorEvidence {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve fixture caller")
	}
	path := filepath.Join(filepath.Dir(here), "..", "..", "..", "tests", "fixtures", "constraint-shadow", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence ports.DetectorEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	return &evidence
}
