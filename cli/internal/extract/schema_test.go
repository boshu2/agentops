package extract

import (
	"encoding/json"
	"testing"
)

// sampleTemplate builds a template whose Output has entities and relations with
// a mix of required and optional fields, used by the schema tests.
func sampleTemplate() *Template {
	return &Template{
		Language: "en",
		Name:     "agentops_provenance",
		Type:     "graph",
		Output: Output{
			Entities: []Field{
				{Name: "id", Type: "string", Description: "stable id", Required: true},
				{Name: "label", Type: "string", Required: true},
				{Name: "tags", Type: "array", Required: false},
				{Name: "weight", Type: "number", Required: false},
			},
			Relations: []Field{
				{Name: "from", Type: "string", Required: true},
				{Name: "relation", Type: "string", Required: true},
				{Name: "to", Type: "string", Required: true},
				{Name: "note", Type: "string", Required: false},
			},
		},
		Guideline:   "Extract provenance entities and relations.",
		Identifiers: Identifiers{EntityID: "{id}", RelationID: "{from}|{relation}|{to}"},
	}
}

// walkAdditionalProps recursively asserts that every JSON Schema object node
// (a map carrying "type":"object" OR a "properties" map) sets
// "additionalProperties": false.
func walkAdditionalProps(t *testing.T, node any, path string) {
	t.Helper()
	switch n := node.(type) {
	case map[string]any:
		_, hasProps := n["properties"]
		isObject := n["type"] == "object"
		if isObject || hasProps {
			ap, ok := n["additionalProperties"]
			if !ok {
				t.Errorf("object node at %s is missing additionalProperties (codex 400 footgun)", path)
			} else if ap != false {
				t.Errorf("object node at %s has additionalProperties=%v, want false", path, ap)
			}
		}
		for k, v := range n {
			walkAdditionalProps(t, v, path+"/"+k)
		}
	case []any:
		for i, v := range n {
			walkAdditionalProps(t, v, path+"["+itoa(i)+"]")
		}
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}

func TestSchema_AdditionalPropertiesFalse(t *testing.T) {
	raw, err := CompileSchema(sampleTemplate())
	if err != nil {
		t.Fatalf("CompileSchema: %v", err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal compiled schema: %v", err)
	}
	walkAdditionalProps(t, doc, "$")

	// Structural shape, codex strict-mode contract: EVERY object's required ==
	// every key in properties (this is what codex --output-schema enforces; the
	// old "only required:true fields" shape 400'd live — age-nzx).
	m := doc.(map[string]any)
	req, _ := m["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("top-level required: want [entities relations], got %v", req)
	}
	props := m["properties"].(map[string]any)
	entItems := props["entities"].(map[string]any)["items"].(map[string]any)
	entReq, _ := entItems["required"].([]any)
	if len(entReq) != 4 { // id,label,tags,weight — ALL four, not just required:true
		t.Errorf("entity item required: want 4 (all props), got %v", entReq)
	}
	relItems := props["relations"].(map[string]any)["items"].(map[string]any)
	relReq, _ := relItems["required"].([]any)
	if len(relReq) != 4 { // from,relation,to,note — ALL four
		t.Errorf("relation item required: want 4 (all props), got %v", relReq)
	}
}

// TestSchema_StrictValidatorPasses runs the single-source-of-truth validator
// recursively over the compiled REAL agentops_provenance template. This is the
// regression that the additionalProperties-only walk missed: it asserts every
// object's required lists every property, which is exactly the live 400 cause.
func TestSchema_StrictValidatorPasses(t *testing.T) {
	tmpl, err := LoadProvenanceTemplate()
	if err != nil {
		t.Fatalf("LoadProvenanceTemplate: %v", err)
	}
	raw, err := CompileSchema(tmpl)
	if err != nil {
		t.Fatalf("CompileSchema: %v", err)
	}
	if err := ValidateCodexStrictSchema(raw); err != nil {
		t.Fatalf("compiled agentops_provenance schema is not codex-strict-valid: %v\nschema=%s", err, raw)
	}
	// And the synthetic mixed-required sample must also pass.
	rawSample, err := CompileSchema(sampleTemplate())
	if err != nil {
		t.Fatalf("CompileSchema(sample): %v", err)
	}
	if err := ValidateCodexStrictSchema(rawSample); err != nil {
		t.Fatalf("compiled sample schema is not codex-strict-valid: %v", err)
	}
}

// TestSchema_ValidatorRejectsMissingRequired seeds the negative test from the
// EXACT live failure: an object whose required omits a property key must be
// REJECTED. This is the literal shape the old CompileSchema emitted.
func TestSchema_ValidatorRejectsMissingRequired(t *testing.T) {
	// "weight" is in properties but missing from required — the age-nzx 400.
	bad := []byte(`{
		"type":"object","additionalProperties":false,
		"properties":{"id":{"type":"string"},"weight":{"type":"number"}},
		"required":["id"]
	}`)
	if err := ValidateCodexStrictSchema(bad); err == nil {
		t.Fatal("ValidateCodexStrictSchema accepted a schema missing a property from required (the age-nzx 400 shape)")
	}

	// Missing additionalProperties is also rejected.
	noAddl := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)
	if err := ValidateCodexStrictSchema(noAddl); err == nil {
		t.Fatal("ValidateCodexStrictSchema accepted a schema missing additionalProperties")
	}
}

// TestSchema_OptionalNullable asserts an optional template field compiles to a
// property that IS in required AND whose type permits null.
func TestSchema_OptionalNullable(t *testing.T) {
	raw, err := CompileSchema(sampleTemplate())
	if err != nil {
		t.Fatalf("CompileSchema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	props := doc["properties"].(map[string]any)
	entItems := props["entities"].(map[string]any)["items"].(map[string]any)
	entProps := entItems["properties"].(map[string]any)
	entReq, _ := entItems["required"].([]any)

	// "weight" is required:false in the sample -> must be in required AND nullable.
	inRequired := false
	for _, r := range entReq {
		if r == "weight" {
			inRequired = true
		}
	}
	if !inRequired {
		t.Errorf("optional field 'weight' missing from required, got %v", entReq)
	}
	weightType := entProps["weight"].(map[string]any)["type"]
	typeArr, ok := weightType.([]any)
	if !ok {
		t.Fatalf("optional field 'weight' type should be a union array, got %T %v", weightType, weightType)
	}
	hasNull := false
	for _, x := range typeArr {
		if x == "null" {
			hasNull = true
		}
	}
	if !hasNull {
		t.Errorf("optional field 'weight' type does not permit null: %v", typeArr)
	}

	// A required field ("id") keeps a single-string type.
	idType := entProps["id"].(map[string]any)["type"]
	if _, isStr := idType.(string); !isStr {
		t.Errorf("required field 'id' type should be a single string, got %T %v", idType, idType)
	}
}

func TestCompileSchema_NilTemplate(t *testing.T) {
	if _, err := CompileSchema(nil); err == nil {
		t.Fatal("CompileSchema(nil) should error")
	}
}
