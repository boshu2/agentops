// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/skills"
)

// newCmd returns a cobra command with captured stdout/stderr.
func newCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&errb)
	return c, &out, &errb
}

func TestSkillsList_JSONShapeFromLiveCatalog(t *testing.T) {
	prev := skillsListJSON
	defer func() { skillsListJSON = prev }()
	skillsListJSON = true
	resetListFilters()

	c, out, errb := newCmd()
	if err := runSkillsList(c, nil); err != nil {
		t.Fatalf("runSkillsList: %v", err)
	}
	if errb.Len() != 0 {
		t.Errorf("expected no stderr in JSON mode, got %q", errb.String())
	}
	var got []skills.CatalogEntry
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a CatalogEntry array: %v\n%s", err, out.String())
	}
	if len(got) == 0 {
		t.Fatal("live catalog yielded zero skills; expected the full inventory")
	}
	// sorted by name ascending
	for i := 1; i < len(got); i++ {
		if got[i-1].Name > got[i].Name {
			t.Errorf("list not sorted at %d: %q > %q", i, got[i-1].Name, got[i].Name)
		}
	}
	// every entry carries a name and a non-empty hexagonal_role
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
	prev := skillsListJSON
	defer func() { skillsListJSON = prev }()
	skillsListJSON = true

	resetListFilters()
	skillsListRole = "domain"
	defer func() { skillsListRole = "" }()

	c, out, _ := newCmd()
	if err := runSkillsList(c, nil); err != nil {
		t.Fatalf("runSkillsList: %v", err)
	}
	var got []skills.CatalogEntry
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not a JSON array: %v", err)
	}
	for _, e := range got {
		if !strings.EqualFold(e.HexagonalRole, "domain") {
			t.Errorf("role=domain returned %q with role %q", e.Name, e.HexagonalRole)
		}
	}
}

func TestSkillsList_RejectsBadUserInvocable(t *testing.T) {
	resetListFilters()
	skillsListUserInvocable = "maybe"
	defer func() { skillsListUserInvocable = "" }()

	c, _, _ := newCmd()
	err := runSkillsList(c, nil)
	if err == nil {
		t.Fatal("expected error for --user-invocable=maybe, got nil")
	}
	if !strings.Contains(err.Error(), "user-invocable") {
		t.Errorf("error should mention the offending flag, got %q", err.Error())
	}
}

func TestSkillsConsumers_JSONArrayOfNames(t *testing.T) {
	prev := skillsConsumersJSON
	defer func() { skillsConsumersJSON = prev }()
	skillsConsumersJSON = true

	c, out, _ := newCmd()
	if err := runSkillsConsumers(c, []string{"rpi"}); err != nil {
		t.Fatalf("runSkillsConsumers: %v", err)
	}
	var got []string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON string array: %v\n%s", err, out.String())
	}
	// names are sorted and never equal to the query target itself
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
	prev := skillsProducersJSON
	defer func() { skillsProducersJSON = prev }()
	skillsProducersJSON = true

	c, out, errb := newCmd()
	if err := runSkillsProducers(c, []string{"definitely-not-a-real-port-xyz"}); err != nil {
		t.Fatalf("runSkillsProducers: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Errorf("empty producers JSON: want %q, got %q", "[]", got)
	}
	if errb.Len() != 0 {
		t.Errorf("expected no stderr in JSON mode, got %q", errb.String())
	}
}

func TestSkillsGraph_MermaidHeaderAndNodes(t *testing.T) {
	prev := skillsGraphFormat
	defer func() { skillsGraphFormat = prev }()
	skillsGraphFormat = "mermaid"

	c, out, _ := newCmd()
	if err := runSkillsGraph(c, nil); err != nil {
		t.Fatalf("runSkillsGraph: %v", err)
	}
	s := out.String()
	if !strings.HasPrefix(s, "graph LR\n") {
		t.Errorf("mermaid output should start with 'graph LR', got:\n%s", s[:min(40, len(s))])
	}
	if !strings.Contains(s, "s_rpi[rpi]") {
		t.Errorf("expected a node for the rpi skill, got:\n%s", s)
	}
}

func TestSkillsGraph_RejectsUnknownFormat(t *testing.T) {
	prev := skillsGraphFormat
	defer func() { skillsGraphFormat = prev }()
	skillsGraphFormat = "dot"

	c, _, _ := newCmd()
	err := runSkillsGraph(c, nil)
	if err == nil {
		t.Fatal("expected error for --format=dot, got nil")
	}
	if !strings.Contains(err.Error(), "mermaid") {
		t.Errorf("error should name the supported format, got %q", err.Error())
	}
}

func TestSkillsGraph_JSONCarriesTypedTopologyDiagnostics(t *testing.T) {
	prev := skillsGraphFormat
	defer func() { skillsGraphFormat = prev }()
	skillsGraphFormat = "json"

	c, out, _ := newCmd()
	if err := runSkillsGraph(c, nil); err != nil {
		t.Fatalf("runSkillsGraph: %v", err)
	}
	var got skills.SkillGraph
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a SkillGraph: %v\n%s", err, out.String())
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

// resetListFilters clears all package-level list flag state so tests don't
// leak filter values into one another.
func resetListFilters() {
	skillsListRole = ""
	skillsListProduces = ""
	skillsListConsumes = ""
	skillsListPractice = ""
	skillsListUserInvocable = ""
}
