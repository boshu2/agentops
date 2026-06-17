package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// validTemplateYAML loads the real on-disk fixture so every mutation test below
// starts from a shape production actually emits (go.md fixture-fidelity rule).
func validTemplateYAML(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "agentops_provenance.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// mutate decodes the fixture into a generic map, applies fn, and re-encodes it,
// so each negative case differs from the valid fixture by exactly one field.
func mutate(t *testing.T, fn func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal(validTemplateYAML(t), &m); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	fn(m)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(m); err != nil {
		t.Fatalf("re-encode fixture: %v", err)
	}
	_ = enc.Close()
	return buf.Bytes()
}

func TestTemplate(t *testing.T) {
	// Scenario: a well-formed template loads and validates.
	t.Run("well_formed_loads", func(t *testing.T) {
		tmpl, err := Parse(validTemplateYAML(t))
		if err != nil {
			t.Fatalf("expected valid template, got error: %v", err)
		}
		if tmpl.Name != "agentops_provenance" {
			t.Errorf("Name = %q, want %q", tmpl.Name, "agentops_provenance")
		}
		if tmpl.Type != "graph" {
			t.Errorf("Type = %q, want %q", tmpl.Type, "graph")
		}
		if !allowedTypes[tmpl.Type] {
			t.Errorf("Type %q not in allowed {graph, temporal_graph, model, set}", tmpl.Type)
		}
		if tmpl.Guideline == "" {
			t.Error("Guideline must be non-empty")
		}
		if tmpl.Identifiers.RelationID != canonicalRelationID {
			t.Errorf("RelationID = %q, want %q", tmpl.Identifiers.RelationID, canonicalRelationID)
		}
		if len(tmpl.Output.Entities) != 3 {
			t.Errorf("Output.Entities len = %d, want 3", len(tmpl.Output.Entities))
		}
		if len(tmpl.Output.Relations) != 3 {
			t.Errorf("Output.Relations len = %d, want 3", len(tmpl.Output.Relations))
		}
	})

	// Scenario: an unknown template type ("hypergraph") is rejected.
	t.Run("unknown_type_rejected", func(t *testing.T) {
		data := mutate(t, func(m map[string]any) { m["type"] = "hypergraph" })
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for type=hypergraph, got nil")
		}
		if !strings.Contains(err.Error(), "hypergraph") {
			t.Errorf("error should name the disallowed type; got: %v", err)
		}
	})

	// Scenario: a malformed relation identifier is rejected. We bypass the
	// schema (which only checks minLength) and exercise Validate directly so the
	// well-formedness check is the thing under test.
	t.Run("malformed_identifier_rejected", func(t *testing.T) {
		tmpl, err := Parse(validTemplateYAML(t))
		if err != nil {
			t.Fatalf("setup: valid template should parse: %v", err)
		}
		tmpl.Identifiers.RelationID = "{from}-{relation}-{to}" // dashes, not pipes
		err = tmpl.Validate()
		if err == nil {
			t.Fatal("expected error for malformed relation identifier, got nil")
		}
		if !strings.Contains(err.Error(), "relation identifier") {
			t.Errorf("error should identify the malformed identifier; got: %v", err)
		}
	})

	t.Run("malformed_identifier_wrong_token_rejected", func(t *testing.T) {
		tmpl, err := Parse(validTemplateYAML(t))
		if err != nil {
			t.Fatalf("setup: valid template should parse: %v", err)
		}
		tmpl.Identifiers.RelationID = "{from}|{rel}|{to}" // {rel} not {relation}
		err = tmpl.Validate()
		if err == nil {
			t.Fatal("expected error for {rel} placeholder, got nil")
		}
		if !strings.Contains(err.Error(), "{relation}") {
			t.Errorf("error should name the expected {relation} token; got: %v", err)
		}
	})

	// Scenario: a template missing the guideline (HOW) is rejected.
	t.Run("missing_guideline_rejected", func(t *testing.T) {
		data := mutate(t, func(m map[string]any) { delete(m, "guideline") })
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for missing guideline, got nil")
		}
		if !strings.Contains(err.Error(), "guideline") {
			t.Errorf("error should require the guideline; got: %v", err)
		}
	})

	// Schema-level: additionalProperties:false rejects unknown top-level keys.
	t.Run("unknown_field_rejected", func(t *testing.T) {
		data := mutate(t, func(m map[string]any) { m["surprise"] = "boom" })
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for unknown top-level field, got nil")
		}
	})
}

// TestTemplate_LoadRoundTrip is the L2 check: a real on-disk template file
// round-trips through Load and yields the expected typed values.
func TestTemplate_LoadRoundTrip(t *testing.T) {
	path := filepath.Join("testdata", "agentops_provenance.yaml")
	tmpl, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s) error: %v", path, err)
	}
	if tmpl.Language != "en" {
		t.Errorf("Language = %q, want %q", tmpl.Language, "en")
	}
	if tmpl.Name != "agentops_provenance" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "agentops_provenance")
	}
	if tmpl.Type != "graph" {
		t.Errorf("Type = %q, want %q", tmpl.Type, "graph")
	}
	if tmpl.Identifiers.EntityID != "id" {
		t.Errorf("EntityID = %q, want %q", tmpl.Identifiers.EntityID, "id")
	}
	if tmpl.Identifiers.RelationID != canonicalRelationID {
		t.Errorf("RelationID = %q, want %q", tmpl.Identifiers.RelationID, canonicalRelationID)
	}
	// Spot-check a typed field survived the round-trip.
	var foundNodeType bool
	for _, f := range tmpl.Output.Entities {
		if f.Name == "node_type" {
			foundNodeType = true
			if f.Type != "string" {
				t.Errorf("node_type field Type = %q, want %q", f.Type, "string")
			}
			if !f.Required {
				t.Error("node_type field should be required")
			}
		}
	}
	if !foundNodeType {
		t.Error("expected a node_type entity field in the round-tripped template")
	}
}

// TestTemplate_SchemaParity asserts the package-embedded schema stays
// byte-identical with the canonical repo-root schema, mirroring the codex
// schema parity guard.
func TestTemplate_SchemaParity(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "extraction-template.v1.schema.json"))
	if err != nil {
		t.Fatalf("read canonical schema: %v", err)
	}
	if !bytes.Equal(canonical, schemaJSON) {
		t.Error("embedded extraction-template schema drifted from schemas/extraction-template.v1.schema.json; re-copy it")
	}
}
