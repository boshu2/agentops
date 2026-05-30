package evalsubstrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// schemaRepoRoot climbs from this test file to the repo root.
// file = .../cli/internal/evalsubstrate/rubric_schema_test.go
func schemaRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// loadOutcomesRubricSchema loads the committed Outcomes-rubric JSON Schema.
func loadOutcomesRubricSchema(t *testing.T) map[string]any {
	t.Helper()
	p := filepath.Join(schemaRepoRoot(t), "schemas", "outcomes-rubric.v1.schema.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read schema %s: %v", p, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}

// jsonFieldNames returns the JSON tag names (sans ",omitempty") for an
// exported struct, so the schema's declared properties can be diffed against
// the real emitted payload shape.
func jsonFieldNames(t *testing.T, v any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func schemaPropNames(t *testing.T, props map[string]any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for k := range props {
		out[k] = true
	}
	return out
}

// TestOutcomesRubricSchemaMatchesStruct is the schema<->struct drift guard. If
// a field is added to Rubric or Criterion without updating the committed
// schema (or vice versa), this fails — keeping the executable projection and
// the published contract in lockstep. Mirrors verdictledger's
// TestSchemaDeclaresADRFields posture.
func TestOutcomesRubricSchemaMatchesStruct(t *testing.T) {
	schema := loadOutcomesRubricSchema(t)

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no properties object")
	}

	// Root properties must exactly match the Rubric struct's JSON fields.
	wantRoot := jsonFieldNames(t, Rubric{})
	gotRoot := schemaPropNames(t, props)
	if !reflect.DeepEqual(wantRoot, gotRoot) {
		t.Errorf("root property drift: struct fields %v != schema properties %v", keys(wantRoot), keys(gotRoot))
	}

	// Criterion properties must exactly match the Criterion struct's JSON fields.
	critSchema, ok := props["criteria"].(map[string]any)
	if !ok {
		t.Fatal("schema.properties.criteria missing")
	}
	items, ok := critSchema["items"].(map[string]any)
	if !ok {
		t.Fatal("schema.properties.criteria.items missing")
	}
	critProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatal("criteria.items.properties missing")
	}
	wantCrit := jsonFieldNames(t, Criterion{})
	gotCrit := schemaPropNames(t, critProps)
	if !reflect.DeepEqual(wantCrit, gotCrit) {
		t.Errorf("criterion property drift: struct fields %v != schema properties %v", keys(wantCrit), keys(gotCrit))
	}
}

// TestOutcomesRubricSchemaForbidsExtraProps verifies additionalProperties:false
// at every nesting level — the structural enforcement that a leak field
// (target/ground_truth/expected_output) makes the payload fail validation, not
// merely a code check. Managed Agents are not ZDR.
func TestOutcomesRubricSchemaForbidsExtraProps(t *testing.T) {
	schema := loadOutcomesRubricSchema(t)
	if v, ok := schema["additionalProperties"].(bool); !ok || v {
		t.Error("root additionalProperties must be false")
	}
	props := schema["properties"].(map[string]any)
	items := props["criteria"].(map[string]any)["items"].(map[string]any)
	if v, ok := items["additionalProperties"].(bool); !ok || v {
		t.Error("criteria.items additionalProperties must be false")
	}
}

// TestOutcomesRubricSchemaRequiredAndVersion pins the required set and the
// schema_version const to the real projection (evalsubstrate.SchemaVersion).
func TestOutcomesRubricSchemaRequiredAndVersion(t *testing.T) {
	schema := loadOutcomesRubricSchema(t)

	req := toStringSet(schema["required"])
	for _, f := range []string{"schema_version", "source_task_id", "judge_content_hash", "criteria"} {
		if !req[f] {
			t.Errorf("schema required must include %q", f)
		}
	}
	if req["instructions"] {
		t.Error("instructions must be optional (omitempty), not required")
	}

	props := schema["properties"].(map[string]any)
	sv := props["schema_version"].(map[string]any)
	cst, ok := sv["const"]
	if !ok {
		t.Fatal("schema_version must declare a const")
	}
	if int(cst.(float64)) != SchemaVersion {
		t.Errorf("schema_version const %v != evalsubstrate.SchemaVersion %d", cst, SchemaVersion)
	}
}

// TestProjectRubricEmissionWithinSchema asserts every JSON key a real
// ProjectRubric output emits is declared in the schema properties — the schema
// can never lag the actual emitted payload.
func TestProjectRubricEmissionWithinSchema(t *testing.T) {
	schema := loadOutcomesRubricSchema(t)
	props := schemaPropNames(t, schema["properties"].(map[string]any))

	r := ProjectRubric(
		Task{ID: "task-x", Description: "do the thing"},
		[]Criterion{{ID: "c1", Description: "d", Weight: 1.0}},
		"sha256:abc",
	)
	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal rubric: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(blob, &m); err != nil {
		t.Fatalf("unmarshal rubric: %v", err)
	}
	for k := range m {
		if !props[k] {
			t.Errorf("emitted key %q not declared in schema properties", k)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func toStringSet(v any) map[string]bool {
	out := map[string]bool{}
	arr, ok := v.([]any)
	if !ok {
		return out
	}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out[s] = true
		}
	}
	return out
}
