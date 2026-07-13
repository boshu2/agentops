package ports

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

// Sibling pattern: inmemory_corpus_writer_test.go (cycle 79). Same
// shape — table-driven where helpful, L1-style assertions for
// behavior + port contract, no external collaborators.

// mechanicalFM is a finding frontmatter carrying valid mechanical detector
// metadata (the only shape that compiles to a constraint), optionally constrained
// to specific compiler_targets.
func mechanicalFM(targets string) map[string]string {
	fm := map[string]string{
		"detectability":         "mechanical",
		"detector_kind":         "regex",
		"detector_pattern":      `panic\(`,
		"constraint_path_globs": "cli/**",
		"compiled_at":           "2026-06-21T00:00:00Z",
	}
	if targets != "" {
		fm["compiler_targets"] = targets
	}
	return fm
}

func replayReadyDetectorEvidence() *DetectorEvidence {
	return &DetectorEvidence{
		PositiveFixtures: []DetectorFixture{{Ref: "positive/panic.go", Content: "panic(\"boom\")"}},
		NegativeControls: []DetectorFixture{{Ref: "negative/return.go", Content: "return errors.New(\"boom\")"}},
	}
}

// An ADVISORY finding (no detector metadata) defaults to plan + pre-mortem only;
// the constraint is skipped, not emitted as a dead artifact (EM-ENF property:
// constraint only with valid mechanical detector metadata).
func TestInMemoryFindingCompiler_AdvisoryDefaultEmitsTwoTargets(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	got, err := c.Compile(context.Background(), FindingArtifact{ID: "fnd-xyz", Body: "body content"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("advisory default emitted %d outputs, want 2 (plan + pre-mortem)", len(got))
	}
	if contains(collectKinds(got), CompiledOutputConstraint) {
		t.Fatalf("advisory finding must emit no constraint; got %v", collectKinds(got))
	}
}

// A MECHANICAL finding (valid detector metadata) defaults to all three targets.
func TestInMemoryFindingCompiler_MechanicalDefaultEmitsThreeTargets(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	got, err := c.Compile(context.Background(), FindingArtifact{
		ID:               "fnd-xyz",
		Frontmatter:      mechanicalFM(""),
		Body:             "body content",
		DetectorEvidence: replayReadyDetectorEvidence(),
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("mechanical default emitted %d outputs, want 3", len(got))
	}
	kinds := collectKinds(got)
	for _, want := range []CompiledOutputKind{
		CompiledOutputPlanningRule,
		CompiledOutputPremortemCheck,
		CompiledOutputConstraint,
	} {
		if !contains(kinds, want) {
			t.Fatalf("missing kind %q in mechanical default emit; got kinds: %v", want, kinds)
		}
	}
}

func TestInMemoryFindingCompiler_CompileEmptyIDIsRejected(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	_, err := c.Compile(context.Background(), FindingArtifact{ID: "", Body: "x"})
	if err == nil {
		t.Fatal("Compile with empty ID returned nil error, want structural rejection")
	}
}

func TestInMemoryFindingCompiler_CompilePathsAreNamespacedByID(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	got, err := c.Compile(context.Background(), FindingArtifact{ID: "fnd-pathing", Frontmatter: mechanicalFM(""), Body: "x", DetectorEvidence: replayReadyDetectorEvidence()})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got {
		if !strings.Contains(item.Path, "fnd-pathing") {
			t.Fatalf("path %q missing artifact ID 'fnd-pathing'", item.Path)
		}
		// Each kind must have a distinct path.
		switch item.Kind {
		case CompiledOutputPlanningRule:
			if !strings.Contains(item.Path, "planning-rules") {
				t.Fatalf("plan kind path %q missing planning-rules/ segment", item.Path)
			}
		case CompiledOutputPremortemCheck:
			if !strings.Contains(item.Path, "premortem-checks") {
				t.Fatalf("premortem kind path %q missing premortem-checks/ segment", item.Path)
			}
		case CompiledOutputConstraint:
			if !strings.HasSuffix(item.Path, ".sh") {
				t.Fatalf("constraint kind path %q must end in .sh", item.Path)
			}
		}
	}
}

func TestInMemoryFindingCompiler_CompileTargetsHonorFrontmatter(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	cases := []struct {
		name      string
		targets   string
		wantKinds []CompiledOutputKind
	}{
		{
			name:    "plan only",
			targets: "plan",
			wantKinds: []CompiledOutputKind{
				CompiledOutputPlanningRule,
			},
		},
		{
			name:    "two targets comma separated",
			targets: "plan, premortem",
			wantKinds: []CompiledOutputKind{
				CompiledOutputPlanningRule,
				CompiledOutputPremortemCheck,
			},
		},
		{
			name:    "unknown target is silently dropped",
			targets: "plan, unknown-kind, constraint",
			wantKinds: []CompiledOutputKind{
				CompiledOutputPlanningRule,
				CompiledOutputConstraint,
			},
		},
		{
			name:      "all-unknown emits empty slice",
			targets:   "alpha, beta",
			wantKinds: []CompiledOutputKind{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Compile(context.Background(), FindingArtifact{
				ID:               "fnd-targeted",
				Frontmatter:      mechanicalFM(tc.targets),
				DetectorEvidence: replayReadyDetectorEvidence(),
			})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got == nil {
				t.Fatal("Compile returned nil slice; port contract requires non-nil")
			}
			gotKinds := collectKinds(got)
			sort.Slice(gotKinds, func(i, j int) bool { return gotKinds[i] < gotKinds[j] })
			wantSorted := append([]CompiledOutputKind{}, tc.wantKinds...)
			sort.Slice(wantSorted, func(i, j int) bool { return wantSorted[i] < wantSorted[j] })
			if !equalKinds(gotKinds, wantSorted) {
				t.Fatalf("got kinds %v, want %v", gotKinds, wantSorted)
			}
		})
	}
}

func TestInMemoryFindingCompiler_CompileHonorsContextCancellation(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := c.Compile(ctx, FindingArtifact{ID: "x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Fatalf("on error, returned slice should be nil; got %+v", got)
	}
}

func TestInMemoryFindingCompiler_CompileOutputPathsAreUnique(t *testing.T) {
	c := NewInMemoryFindingCompiler()
	// Pass the same target twice — adapter must deduplicate.
	got, err := c.Compile(context.Background(), FindingArtifact{
		ID:               "fnd-dedup",
		Frontmatter:      mechanicalFM("plan, plan, constraint"),
		DetectorEvidence: replayReadyDetectorEvidence(),
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, item := range got {
		if _, dup := seen[item.Path]; dup {
			t.Fatalf("duplicate Path %q in Compile output (contract violation)", item.Path)
		}
		seen[item.Path] = struct{}{}
	}
	if len(got) != 2 {
		t.Fatalf("dedup expected 2 outputs (plan + constraint), got %d", len(got))
	}
}

// --- test helpers ---

func collectKinds(out []CompiledOutput) []CompiledOutputKind {
	kinds := make([]CompiledOutputKind, 0, len(out))
	for _, o := range out {
		kinds = append(kinds, o.Kind)
	}
	return kinds
}

func contains(kinds []CompiledOutputKind, want CompiledOutputKind) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func equalKinds(a, b []CompiledOutputKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
