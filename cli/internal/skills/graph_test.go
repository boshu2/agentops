package skills

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildGraphRejectsDuplicateNodes(t *testing.T) {
	g := BuildGraph([]CatalogEntry{{Name: "root", GraphRoot: true}, {Name: "root"}})
	if len(g.Diagnostics.DuplicateNodes) != 1 || g.Diagnostics.DuplicateNodes[0] != "root" {
		t.Fatalf("duplicate diagnostics: %#v", g.Diagnostics)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("duplicate nodes should fail graph validation")
	}
}

func TestBuildGraphRejectsDanglingTargets(t *testing.T) {
	g := BuildGraph([]CatalogEntry{{Name: "root", GraphRoot: true, Dependencies: []string{"missing"}}})
	if len(g.Diagnostics.DanglingEdges) != 1 || g.Diagnostics.DanglingEdges[0].To != "missing" {
		t.Fatalf("dangling diagnostics: %#v", g.Diagnostics)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("dangling target should fail graph validation")
	}
}

func TestBuildGraphRejectsDependencyCycles(t *testing.T) {
	g := BuildGraph([]CatalogEntry{
		{Name: "a", GraphRoot: true, Dependencies: []string{"b"}},
		{Name: "b", Dependencies: []string{"a"}},
	})
	if len(g.Diagnostics.DependencyCycles) == 0 {
		t.Fatalf("cycle missing: %#v", g.Diagnostics)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("dependency cycle should fail graph validation")
	}
}

func TestBuildGraphRejectsUnreachableNonRoot(t *testing.T) {
	g := BuildGraph([]CatalogEntry{
		{Name: "root", GraphRoot: true, Dependencies: []string{"leaf"}},
		{Name: "leaf"},
		{Name: "orphan", UserInvocable: true},
	})
	if got := strings.Join(g.Diagnostics.UnreachableNonRoots, ","); got != "orphan" {
		t.Fatalf("unreachable = %q, want orphan", got)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("unreachable non-root should fail graph validation")
	}
}

func TestBuildGraphPreservesExplicitRoot(t *testing.T) {
	g := BuildGraph([]CatalogEntry{{Name: "root", GraphRoot: true}})
	if len(g.Diagnostics.ZeroInboundRoots) != 1 || g.Diagnostics.ZeroInboundRoots[0] != "root" {
		t.Fatalf("explicit root diagnostics: %#v", g.Diagnostics)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("explicit root should be valid: %v", err)
	}
}

func TestBuildGraphRejectsDanglingContextTarget(t *testing.T) {
	g := BuildGraph([]CatalogEntry{{Name: "root", GraphRoot: true, ContextRel: []ContextRel{{Kind: "customer-of", With: "missing"}}}})
	if len(g.Diagnostics.DanglingEdges) != 1 || g.Diagnostics.DanglingEdges[0].Kind != "context:customer-of" {
		t.Fatalf("dangling context diagnostics: %#v", g.Diagnostics)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("dangling context target should fail graph validation")
	}
}

func TestGraphJSONCarriesTypedEdgesAndDiagnostics(t *testing.T) {
	entries := []CatalogEntry{
		{Name: "root", GraphRoot: true, Dependencies: []string{"leaf"}, ContextRel: []ContextRel{{Kind: "customer-of", With: "leaf"}}},
		{Name: "leaf"},
	}
	raw, err := GraphJSON(entries)
	if err != nil {
		t.Fatalf("GraphJSON: %v", err)
	}
	var got SkillGraph
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Edges) != 2 || got.Edges[0].Kind == got.Edges[1].Kind {
		t.Fatalf("typed edges missing: %#v", got.Edges)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("valid graph: %v", err)
	}
}

func TestMermaidUsesDependenciesNotArtifactConsumes(t *testing.T) {
	entries := []CatalogEntry{
		{Name: "root", GraphRoot: true, Dependencies: []string{"leaf"}, Consumes: []string{"artifact-name"}},
		{Name: "leaf"},
	}
	got := Mermaid(entries)
	if !strings.Contains(got, "s_root --> s_leaf") {
		t.Fatalf("dependency edge missing:\n%s", got)
	}
	if strings.Contains(got, "artifact-name") {
		t.Fatalf("artifact consumes leaked into dependency graph:\n%s", got)
	}
}
