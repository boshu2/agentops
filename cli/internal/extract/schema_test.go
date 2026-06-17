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

	// Spot-check structural shape: top-level object requires entities+relations,
	// item objects only require the Required fields (not over-required).
	m := doc.(map[string]any)
	req, _ := m["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("top-level required: want [entities relations], got %v", req)
	}
	props := m["properties"].(map[string]any)
	entItems := props["entities"].(map[string]any)["items"].(map[string]any)
	entReq, _ := entItems["required"].([]any)
	if len(entReq) != 2 { // id + label are required; tags + weight are not
		t.Errorf("entity item required: want 2 (id,label), got %v", entReq)
	}
	relItems := props["relations"].(map[string]any)["items"].(map[string]any)
	relReq, _ := relItems["required"].([]any)
	if len(relReq) != 3 { // from, relation, to; note is optional
		t.Errorf("relation item required: want 3 (from,relation,to), got %v", relReq)
	}
}

func TestCompileSchema_NilTemplate(t *testing.T) {
	if _, err := CompileSchema(nil); err == nil {
		t.Fatal("CompileSchema(nil) should error")
	}
}
