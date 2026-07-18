package eval

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileEvalSchemaForTest loads a checked-in eval contract schema
// (cwd = cli/internal/eval, so three levels up).
func compileEvalSchemaForTest(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "..", "..", "schemas", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse schema %s: %v", name, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(name, doc); err != nil {
		t.Fatalf("add schema resource %s: %v", name, err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatalf("compile schema %s: %v", name, err)
	}
	return schema
}

// TestRunRecord_ValidatesAgainstEvalRunSchema (LOAD-BEARING drift guard):
// a run record produced by the production writer must validate against
// schemas/eval-run.v1.schema.json. Without this, the Go writer and the
// declared contract can fork silently — the same failure class the
// verdict.v2 golden corpus closed for the Validate contract.
func TestRunRecord_ValidatesAgainstEvalRunSchema(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "alpha\nneedle\nomega\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "fixture.schema-drift",
  "name": "Schema drift fixture",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "contains",
      "title": "fixture contains needle",
      "kind": "artifact_check",
      "objective": "Verify static fixtures are scored offline.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "needle"}
      ]
    }
  ]
}`)

	outPath := filepath.Join(dir, "run.json")
	run, err := RunSuite(RunOptions{
		SuitePath:  suitePath,
		RunID:      "run-schema-drift",
		OutputPath: outPath,
		Now:        fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if run == nil {
		t.Fatal("RunSuite returned nil record")
	}

	// Validate the PERSISTED bytes, not a re-marshal: the on-disk artifact is
	// the contract surface consumers read.
	payload, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read persisted run record: %v", err)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("persisted run record is not JSON: %v", err)
	}
	schema := compileEvalSchemaForTest(t, "eval-run.v1.schema.json")
	if err := schema.Validate(value); err != nil {
		t.Fatalf("persisted run record does not satisfy eval-run.v1.schema.json:\n%v", err)
	}
}

// TestSuiteFixture_ValidatesAgainstEvalSuiteSchema pins the suite contract the
// same way: the fixture accepted by the production loader must satisfy
// schemas/eval-suite.v1.schema.json.
func TestSuiteFixture_ValidatesAgainstEvalSuiteSchema(t *testing.T) {
	var value any
	if err := json.Unmarshal([]byte(`{
  "schema_version": 1,
  "id": "fixture.schema-drift",
  "name": "Schema drift fixture",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "contains",
      "title": "fixture contains needle",
      "kind": "artifact_check",
      "objective": "Verify static fixtures are scored offline.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "needle"}
      ]
    }
  ]
}`), &value); err != nil {
		t.Fatal(err)
	}
	schema := compileEvalSchemaForTest(t, "eval-suite.v1.schema.json")
	if err := schema.Validate(value); err != nil {
		t.Fatalf("minimal suite fixture does not satisfy eval-suite.v1.schema.json:\n%v", err)
	}
}
