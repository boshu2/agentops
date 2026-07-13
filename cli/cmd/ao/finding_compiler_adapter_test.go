// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
)

// Sibling pattern: cycle 113 corpus_writer_adapter_test.go.

// mechanicalFrontmatter is a finding carrying valid mechanical detector metadata
// — the only shape that compiles to a (structured) constraint.
func mechanicalFrontmatter(extra map[string]string) map[string]string {
	fm := map[string]string{
		"detectability":         "mechanical",
		"detector_kind":         "regex",
		"detector_pattern":      "panic\\(",
		"constraint_path_globs": "cli/**",
		"compiled_at":           "2026-06-21T00:00:00Z",
	}
	for k, v := range extra {
		fm[k] = v
	}
	return fm
}

// An ADVISORY finding (no detector metadata) defaults to plan + pre-mortem only:
// the constraint target is skipped rather than emitting a dead artifact the gate
// ignores. (EM-ENF: constraint only when detector metadata is present and valid.)
func TestProductionFindingCompiler_AdvisoryDefaultsToTwoKinds(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:   "soc-test",
		Body: "rationale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (advisory: plan+pre-mortem, no constraint)", len(out))
	}
	for _, o := range out {
		if o.Kind == ports.CompiledOutputConstraint {
			t.Fatalf("advisory finding must not emit a constraint")
		}
	}
}

// A MECHANICAL finding with empty compiler_targets emits all three, and the
// constraint is the structured index.json entry.
func TestProductionFindingCompiler_MechanicalDefaultsIncludeConstraint(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:               "soc-test",
		Frontmatter:      mechanicalFrontmatter(nil),
		Body:             "rationale",
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3 (mechanical defaults)", len(out))
	}
	if out[2].Kind != ports.CompiledOutputConstraint || out[2].Path != ".agents/constraints/index.json" {
		t.Fatalf("constraint target = %s @ %q, want constraint @ index.json", out[2].Kind, out[2].Path)
	}
}

func TestProductionFindingCompiler_HonorsCompilerTargets(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:               "soc-test",
		Frontmatter:      mechanicalFrontmatter(map[string]string{"compiler_targets": "plan, constraint"}),
		Body:             "rationale",
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (per compiler_targets)", len(out))
	}
	if out[0].Kind != ports.CompiledOutputPlanningRule {
		t.Fatalf("out[0].Kind = %s, want plan", out[0].Kind)
	}
	if out[1].Kind != ports.CompiledOutputConstraint {
		t.Fatalf("out[1].Kind = %s, want constraint", out[1].Kind)
	}
}

func TestProductionFindingCompiler_DeduplicatesTargets(t *testing.T) {
	c := newProductionFindingCompiler()
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{
		ID:               "soc-test",
		Frontmatter:      mechanicalFrontmatter(map[string]string{"compiler_targets": "plan,plan,constraint,plan"}),
		Body:             "x",
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
	})
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (dedup)", len(out))
	}
}

// The structured constraint entry starts warn-only in shadow with detector
// fields matching the landed gate exactly (kind=regex / pattern / path_globs).
func TestProductionFindingCompiler_MechanicalEmitsShadowEntry(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:               "f-test-1",
		Frontmatter:      mechanicalFrontmatter(map[string]string{"compiler_targets": "constraint", "detector_message": "no panic"}),
		Body:             "x",
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Kind != ports.CompiledOutputConstraint {
		t.Fatalf("want one constraint output, got %d", len(out))
	}
	var e search.ConstraintEntry
	if err := json.Unmarshal(out[0].Body, &e); err != nil {
		t.Fatalf("constraint body is not a ConstraintEntry: %v", err)
	}
	if e.ID != "f-test-1" || e.Status != "shadow" || e.EnforcementMode != "warn" {
		t.Fatalf("entry id/status/mode = %q/%q/%q, want f-test-1/shadow/warn", e.ID, e.Status, e.EnforcementMode)
	}
	if e.Detector.Kind != "regex" || e.Detector.Pattern != "panic\\(" || e.Detector.Message != "no panic" {
		t.Fatalf("detector = %+v, want regex/panic\\(/no panic", e.Detector)
	}
	if len(e.AppliesTo.PathGlobs) != 1 || e.AppliesTo.PathGlobs[0] != "cli/**" {
		t.Fatalf("path_globs = %v, want [cli/**]", e.AppliesTo.PathGlobs)
	}
}

// Incomplete/non-mechanical detector metadata => the constraint is skipped, NOT
// emitted as a broken entry (advisory-only is the safe default).
func TestProductionFindingCompiler_IncompleteDetectorSkipsConstraint(t *testing.T) {
	cases := map[string]map[string]string{
		"advisory (no detectability)":   {"compiler_targets": "constraint"},
		"mechanical but no pattern":     {"compiler_targets": "constraint", "detectability": "mechanical", "constraint_path_globs": "cli/**", "compiled_at": "x"},
		"mechanical but no globs":       {"compiler_targets": "constraint", "detectability": "mechanical", "detector_pattern": "x", "compiled_at": "x"},
		"mechanical but no compiled_at": {"compiler_targets": "constraint", "detectability": "mechanical", "detector_pattern": "x", "constraint_path_globs": "cli/**"},
		"unsupported detector kind":     {"compiler_targets": "constraint", "detectability": "mechanical", "detector_kind": "command", "detector_pattern": "x", "constraint_path_globs": "cli/**", "compiled_at": "x"},
	}
	c := newProductionFindingCompiler()
	for name, fm := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := c.Compile(context.Background(), ports.FindingArtifact{ID: "f", Frontmatter: fm, Body: "x"})
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: want 0 outputs (constraint skipped), got %d", name, len(out))
			}
		})
	}
}

func TestProductionFindingCompiler_UnknownTargetErrors(t *testing.T) {
	c := newProductionFindingCompiler()
	_, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:          "soc-test",
		Frontmatter: map[string]string{"compiler_targets": "plan,bogus"},
	})
	if err == nil {
		t.Fatal("expected error on unknown target, got nil")
	}
}

func TestProductionFindingCompiler_PathsFollowContract(t *testing.T) {
	c := newProductionFindingCompiler()
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{ID: "soc-y5vh", Frontmatter: mechanicalFrontmatter(nil), DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json")})
	pathByKind := map[ports.CompiledOutputKind]string{}
	for _, o := range out {
		pathByKind[o.Kind] = o.Path
	}
	want := map[ports.CompiledOutputKind]string{
		ports.CompiledOutputPlanningRule:   ".agents/planning-rules/soc-y5vh.md",
		ports.CompiledOutputPremortemCheck: ".agents/premortem-checks/soc-y5vh.md",
		// A constraint is a structured entry in the shared index, not a per-id file.
		ports.CompiledOutputConstraint: ".agents/constraints/index.json",
	}
	for kind, wantPath := range want {
		if pathByKind[kind] != wantPath {
			t.Fatalf("Path[%s] = %q, want %q", kind, pathByKind[kind], wantPath)
		}
	}
}

func TestProductionFindingCompiler_NoDuplicatePaths(t *testing.T) {
	c := newProductionFindingCompiler()
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{ID: "soc-x"})
	seen := make(map[string]struct{}, len(out))
	for _, o := range out {
		if _, dup := seen[o.Path]; dup {
			t.Fatalf("duplicate path: %s", o.Path)
		}
		seen[o.Path] = struct{}{}
	}
}

func TestProductionFindingCompiler_BodyIncludesKindHeader(t *testing.T) {
	c := newProductionFindingCompiler()
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{
		ID:   "soc-x",
		Body: "rationale text",
	})
	for _, o := range out {
		body := string(o.Body)
		if !strings.Contains(body, "(soc-x)") {
			t.Fatalf("missing ID in body for %s:\n%s", o.Kind, body)
		}
		if !strings.Contains(body, "rationale text") {
			t.Fatalf("missing original body for %s", o.Kind)
		}
	}
}

func TestProductionFindingCompiler_FrontmatterRenderedSorted(t *testing.T) {
	c := newProductionFindingCompiler()
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{
		ID:          "soc-x",
		Frontmatter: map[string]string{"tag": "evolve", "date": "2026-05-12"},
		Body:        "body",
	})
	body := string(out[0].Body)
	if !strings.HasPrefix(body, "---\ndate: 2026-05-12\ntag: evolve\n---\n") {
		t.Fatalf("frontmatter not rendered/sorted:\n%s", body)
	}
}

func TestProductionFindingCompiler_EmptyIDErrors(t *testing.T) {
	c := newProductionFindingCompiler()
	_, err := c.Compile(context.Background(), ports.FindingArtifact{Body: "x"})
	if err == nil {
		t.Fatal("expected error on empty ID, got nil")
	}
}

func TestProductionFindingCompiler_HonorsContextCancellation(t *testing.T) {
	c := newProductionFindingCompiler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Compile(ctx, ports.FindingArtifact{ID: "soc-x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
