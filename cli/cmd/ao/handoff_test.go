package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestHandoffDryRunSatisfiesSchema proves the `ao session handoff --dry-run`
// generator agrees with schemas/handoff.v1.schema.json using a real JSON Schema
// validator (santhosh-tekuri/jsonschema/v6 — the same dependency the eval and
// provenance drift guards use, with the same compile-then-Validate pattern and
// its default format-annotation behaviour). This checks types, the
// schema_version const, the id pattern, enums, and nested additionalProperties,
// not mere key presence. It locks the reconciliation that dropped the three
// verbatim schema errors while keeping the deprecated read-compat fields
// accepted.
func TestHandoffDryRunSatisfiesSchema(t *testing.T) {
	schema := compileHandoffSchema(t)

	dir := t.TempDir()
	t.Chdir(dir)

	handoffGoal = "prove one behavior"
	handoffContinuation = "caller will choose whether to revise"
	handoffCollect = true // exercise the nested state block too
	handoffDryRun = true
	t.Cleanup(func() {
		handoffGoal, handoffContinuation = "", ""
		handoffCollect, handoffDryRun = false, false
	})

	var out bytes.Buffer
	cmd := *handoffCmd
	cmd.SetOut(&out)
	if err := runHandoff(&cmd, []string{"candidate failed validation"}); err != nil {
		t.Fatal(err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\n%s", err, out.String())
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("dry-run output does not satisfy handoff.v1.schema.json:\n%v\n%s", err, out.String())
	}

	// Write-discipline: the schema now ACCEPTS the deprecated lifecycle fields
	// for read-compat, but the current generator must never EMIT them.
	var keys map[string]any
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		t.Fatalf("re-parse dry-run output: %v", err)
	}
	for _, forbidden := range []string{"type", "consumed", "consumed_at", "consumed_by", "rpi"} {
		if _, ok := keys[forbidden]; ok {
			t.Errorf("generator emitted deprecated lifecycle field %q (must stay read-compat-only)", forbidden)
		}
	}
}

// TestHandoffSchemaAcceptsLegacyArtifact proves the read-compatibility promise:
// an artifact written by an earlier generator (carrying type/consumed and a
// fractional id) still validates against handoff.v1, so the schema change is
// not a silent incompatible break.
func TestHandoffSchemaAcceptsLegacyArtifact(t *testing.T) {
	schema := compileHandoffSchema(t)

	legacy := []byte(`{
	  "schema_version": 1,
	  "id": "handoff-20260101T090000.123456789Z",
	  "created_at": "2026-01-01T09:00:00.123456789Z",
	  "type": "manual",
	  "consumed": false,
	  "consumed_at": null,
	  "consumed_by": null,
	  "goal": "legacy goal"
	}`)
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(legacy))
	if err != nil {
		t.Fatalf("legacy fixture is not JSON: %v", err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("legacy v1 artifact must still validate (read-compat):\n%v", err)
	}
}

func compileHandoffSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	// cwd during `go test` is the package dir cli/cmd/ao, so three levels up.
	name := filepath.Join("..", "..", "..", "schemas", "handoff.v1.schema.json")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read handoff schema: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse handoff schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("handoff.v1.schema.json", doc); err != nil {
		t.Fatalf("add handoff schema resource: %v", err)
	}
	schema, err := c.Compile("handoff.v1.schema.json")
	if err != nil {
		t.Fatalf("compile handoff schema: %v", err)
	}
	return schema
}
