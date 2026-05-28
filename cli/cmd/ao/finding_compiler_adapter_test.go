// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 113 corpus_writer_adapter_test.go.

func TestProductionFindingCompiler_DefaultsToAllThreeKinds(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:   "soc-test",
		Body: "rationale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3 (defaults)", len(out))
	}
	gotKinds := []ports.CompiledOutputKind{out[0].Kind, out[1].Kind, out[2].Kind}
	wantKinds := []ports.CompiledOutputKind{
		ports.CompiledOutputPlanningRule,
		ports.CompiledOutputPreMortemCheck,
		ports.CompiledOutputConstraint,
	}
	for i, k := range wantKinds {
		if gotKinds[i] != k {
			t.Fatalf("kinds[%d] = %s, want %s", i, gotKinds[i], k)
		}
	}
}

func TestProductionFindingCompiler_HonorsCompilerTargets(t *testing.T) {
	c := newProductionFindingCompiler()
	out, err := c.Compile(context.Background(), ports.FindingArtifact{
		ID:          "soc-test",
		Frontmatter: map[string]string{"compiler_targets": "plan, constraint"},
		Body:        "rationale",
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
		ID:          "soc-test",
		Frontmatter: map[string]string{"compiler_targets": "plan,plan,constraint,plan"},
		Body:        "x",
	})
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (dedup)", len(out))
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
	out, _ := c.Compile(context.Background(), ports.FindingArtifact{ID: "soc-y5vh"})
	pathByKind := map[ports.CompiledOutputKind]string{}
	for _, o := range out {
		pathByKind[o.Kind] = o.Path
	}
	want := map[ports.CompiledOutputKind]string{
		ports.CompiledOutputPlanningRule:   ".agents/planning-rules/soc-y5vh.md",
		ports.CompiledOutputPreMortemCheck: ".agents/pre-mortem-checks/soc-y5vh.md",
		ports.CompiledOutputConstraint:     ".agents/constraints/soc-y5vh.md",
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
