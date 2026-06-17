package extract

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// provenanceTemplatePath is the real on-disk extraction template under test.
const provenanceTemplatePath = "templates/agentops_provenance.yaml"

// TestTemplate_AgentopsProvenanceValid loads the first real extraction template
// through the age-rsh loader/validator and asserts it passes with no error and
// binds to the expected typed shape (L2: a real file, the production reader).
func TestTemplate_AgentopsProvenanceValid(t *testing.T) {
	tmpl, err := Load(provenanceTemplatePath)
	if err != nil {
		t.Fatalf("Load(%s) should validate, got error: %v", provenanceTemplatePath, err)
	}
	if tmpl.Name != "agentops_provenance" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "agentops_provenance")
	}
	if tmpl.Type != "graph" {
		t.Errorf("Type = %q, want %q", tmpl.Type, "graph")
	}
	// The canonical relation identifier must be the closed-form '{from}|{relation}|{to}'.
	if tmpl.Identifiers.RelationID != canonicalRelationID {
		t.Errorf("RelationID = %q, want %q", tmpl.Identifiers.RelationID, canonicalRelationID)
	}
	if len(tmpl.Output.Entities) == 0 {
		t.Error("expected at least one entity field")
	}
	if len(tmpl.Output.Relations) == 0 {
		t.Error("expected at least one relation field")
	}
	// The HOW (guideline) is required and must be present.
	if tmpl.Guideline == "" {
		t.Error("guideline (the HOW) must be non-empty")
	}
}

// provOVerbRe matches a PROV-O-shaped verb token ("wasX...") so the verb-closure
// test can scan the template text for every verb it mentions, including any an
// author might have leaked in by mistake.
var provOVerbRe = regexp.MustCompile(`\bwas[A-Z][A-Za-z]+\b`)

// TestTemplate_ProvOVerbsClosed asserts that every PROV-O verb mentioned in the
// agentops_provenance template is a member of the closed provenancegraph.Relations
// enum. The extraction-template schema cannot enumerate the allowed verbs (the
// relation verb is a free string field), so the closure invariant is enforced
// here: the template names the vocabulary in its relation-field description and
// guideline, and not one of those tokens may fall outside the enforced set.
func TestTemplate_ProvOVerbsClosed(t *testing.T) {
	data, err := os.ReadFile(filepath.Clean(provenanceTemplatePath))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	allowed := make(map[string]bool, len(provenancegraph.Relations))
	for _, r := range provenancegraph.Relations {
		allowed[r] = true
	}
	verbs := provOVerbRe.FindAllString(string(data), -1)
	if len(verbs) == 0 {
		t.Fatal("template names no PROV-O verbs; expected the closed vocabulary to be enumerated")
	}
	seen := map[string]bool{}
	for _, v := range verbs {
		if !allowed[v] {
			t.Errorf("relation verb %q is NOT a member of provenancegraph.Relations %v", v, provenancegraph.Relations)
		}
		seen[v] = true
	}
	// The template must reference at least one real PROV-O verb (sanity: the
	// closure check above is only meaningful over a non-empty set of valid verbs).
	var anyValid bool
	for v := range seen {
		if allowed[v] {
			anyValid = true
			break
		}
	}
	if !anyValid {
		t.Error("template references no valid provenancegraph.Relations verb")
	}
}
