package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestHandoffDryRunSatisfiesSchema proves the `ao session handoff --dry-run`
// generator agrees with schemas/handoff.v1.schema.json: the id matches the
// schema id pattern, every schema-required property is present, and the
// artifact emits no key the schema (additionalProperties:false) forbids. This
// locks the reconciliation that removed the stale type/consumed/rpi machinery
// and dropped the fractional-second id (the three verbatim schema errors).
func TestHandoffDryRunSatisfiesSchema(t *testing.T) {
	// Resolve the schema path from the package directory before t.Chdir moves
	// the working directory to a temp dir.
	schemaPath, err := filepath.Abs(filepath.Join("..", "..", "..", "schemas", "handoff.v1.schema.json"))
	if err != nil {
		t.Fatalf("resolve schema path: %v", err)
	}

	dir := t.TempDir()
	t.Chdir(dir)

	handoffGoal = "prove one behavior"
	handoffContinuation = ""
	handoffCollect = false
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

	var artifact map[string]any
	if err := json.Unmarshal(out.Bytes(), &artifact); err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\n%s", err, out.String())
	}

	// Load the schema the artifact must satisfy.
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Pattern string `json:"pattern"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	// Error 1+2 fixed: every required property is present.
	for _, req := range schema.Required {
		if _, ok := artifact[req]; !ok {
			t.Errorf("artifact missing schema-required property %q", req)
		}
	}

	// additionalProperties:false — every emitted key is a declared property.
	for key := range artifact {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("artifact emits key %q not declared in schema (additionalProperties:false)", key)
		}
	}

	// The stale consumption/phase machinery must never reappear.
	for _, forbidden := range []string{"type", "consumed", "consumed_at", "consumed_by", "rpi"} {
		if _, ok := artifact[forbidden]; ok {
			t.Errorf("artifact leaked retired lifecycle field %q", forbidden)
		}
	}

	// Error 3 fixed: id matches the schema pattern exactly (second-granular,
	// no fractional seconds).
	idPattern := schema.Properties["id"].Pattern
	if idPattern == "" {
		t.Fatal("schema declares no id pattern")
	}
	id, _ := artifact["id"].(string)
	if !regexp.MustCompile(idPattern).MatchString(id) {
		t.Errorf("id %q does not match schema pattern %q", id, idPattern)
	}
}
