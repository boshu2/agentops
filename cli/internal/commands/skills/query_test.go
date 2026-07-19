// practices: [design-by-contract, code-complete]
package skills

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/skills"
)

func TestSkillsList_JSONShapeFromLiveCatalog(t *testing.T) {
	stdout, stderr, err := execSkills(t, "list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if stderr != "" {
		t.Errorf("expected no stderr in JSON mode, got %q", stderr)
	}
	var got []skills.CatalogEntry
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not a CatalogEntry array: %v\n%s", err, stdout)
	}
	if len(got) == 0 {
		t.Fatal("live catalog yielded zero skills; expected the full inventory")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Name > got[i].Name {
			t.Errorf("list not sorted at %d: %q > %q", i, got[i-1].Name, got[i].Name)
		}
	}
	for _, e := range got {
		if e.Name == "" {
			t.Errorf("entry with empty name: %+v", e)
		}
		if e.HexagonalRole == "" {
			t.Errorf("skill %q has empty hexagonal_role", e.Name)
		}
	}
}

func TestSkillsList_RoleFilterIsSubsetAndAllMatch(t *testing.T) {
	stdout, _, err := execSkills(t, "list", "--json", "--role", "domain")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got []skills.CatalogEntry
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not a JSON array: %v", err)
	}
	for _, e := range got {
		if !strings.EqualFold(e.HexagonalRole, "domain") {
			t.Errorf("role=domain returned %q with role %q", e.Name, e.HexagonalRole)
		}
	}
}

func TestSkillsList_RejectsBadUserInvocable(t *testing.T) {
	_, _, err := execSkills(t, "list", "--user-invocable", "maybe")
	if err == nil {
		t.Fatal("expected error for --user-invocable=maybe, got nil")
	}
	if !strings.Contains(err.Error(), "user-invocable") {
		t.Errorf("error should mention the offending flag, got %q", err.Error())
	}
}

func TestSkillsConsumers_JSONArrayOfNames(t *testing.T) {
	stdout, _, err := execSkills(t, "consumers", "--json", "rpi")
	if err != nil {
		t.Fatalf("consumers: %v", err)
	}
	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not a JSON string array: %v\n%s", err, stdout)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("consumers not sorted at %d: %q > %q", i, got[i-1], got[i])
		}
	}
	for _, n := range got {
		if n == "rpi" {
			t.Errorf("a skill should not be listed as its own consumer: %q", n)
		}
	}
}

func TestSkillsProducers_EmptyMatchIsEmptyJSONArray(t *testing.T) {
	stdout, stderr, err := execSkills(t, "producers", "--json", "definitely-not-a-real-port-xyz")
	if err != nil {
		t.Fatalf("producers: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Errorf("empty producers JSON: want %q, got %q", "[]", got)
	}
	if stderr != "" {
		t.Errorf("expected no stderr in JSON mode, got %q", stderr)
	}
}

func TestSkillsGraph_MermaidHeaderAndNodes(t *testing.T) {
	stdout, _, err := execSkills(t, "graph", "--format", "mermaid")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.HasPrefix(stdout, "graph LR\n") {
		t.Errorf("mermaid output should start with 'graph LR', got:\n%s", stdout[:min(40, len(stdout))])
	}
	if !strings.Contains(stdout, "s_rpi[rpi]") {
		t.Errorf("expected a node for the rpi skill, got:\n%s", stdout)
	}
}

func TestSkillsGraph_RejectsUnknownFormat(t *testing.T) {
	_, _, err := execSkills(t, "graph", "--format", "dot")
	if err == nil {
		t.Fatal("expected error for --format=dot, got nil")
	}
	if !strings.Contains(err.Error(), "mermaid") {
		t.Errorf("error should name the supported format, got %q", err.Error())
	}
}

func TestSkillsGraph_JSONCarriesTypedTopologyDiagnostics(t *testing.T) {
	stdout, _, err := execSkills(t, "graph", "--format", "json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var got skills.SkillGraph
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not a SkillGraph: %v\n%s", err, stdout)
	}
	if got.SchemaVersion != "skill-graph.v1" {
		t.Fatalf("schema_version = %q, want skill-graph.v1", got.SchemaVersion)
	}
	if len(got.Nodes) == 0 || len(got.Edges) == 0 {
		t.Fatalf("graph must carry nodes and typed edges: %+v", got)
	}
	if got.Diagnostics.EntryPoints == nil || got.Diagnostics.DanglingEdges == nil {
		t.Fatalf("graph diagnostics must encode empty arrays, not null: %+v", got.Diagnostics)
	}
	if len(got.Diagnostics.DanglingEdges) != 0 || len(got.Diagnostics.DependencyCycles) != 0 {
		t.Fatalf("live graph contains an invalid hard edge; diagnostics = %+v", got.Diagnostics)
	}
}
