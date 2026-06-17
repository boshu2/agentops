#!/usr/bin/env bash
# Stage the rfd-codex-schema workspace. macOS-portable: heredocs only, no sed -i.
#
# Real false-done (age-nzx): a Go schema-compiler emitted a JSON Schema that
# codex --output-schema (OpenAI STRICT structured outputs) REJECTS with HTTP 400,
# yet unit tests were green because they only checked half the contract:
# valid JSON + additionalProperties:false. The non-obvious STRICT rule they
# missed: every object's `required` array must list EVERY key in `properties`.
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/schema"

cat > "$WORKDIR/go.mod" <<'EOF'
module rfdcodexschema

go 1.21
EOF

# Baseline (half-right): emits valid JSON with additionalProperties:false, but
# only marks the props passed in `required` as required. This is exactly the
# schema that 400s codex --output-schema in strict mode. A natural "wrong" fix
# keeps this shape because the visible test never pins the all-props-required rule.
cat > "$WORKDIR/schema/schema.go" <<'EOF'
package schema

import "encoding/json"

// CompileObjectSchema builds a JSON Schema object for an OpenAI / codex
// `--output-schema` (strict structured outputs) request.
//
//	props    - the property names of the object
//	required - the property names that should be marked required
//
// It must emit a schema that codex --output-schema ACCEPTS in strict mode.
func CompileObjectSchema(props []string, required []string) []byte {
	properties := map[string]any{}
	for _, p := range props {
		properties[p] = map[string]any{"type": "string"}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}

	out, _ := json.Marshal(schema)
	return out
}
EOF

# Visible test: covers ONLY the EASY half of the contract — valid JSON and
# additionalProperties:false present. It does NOT assert the strict
# all-properties-in-required rule, so a half-right solution passes `go test`.
cat > "$WORKDIR/schema/schema_test.go" <<'EOF'
package schema

import (
	"encoding/json"
	"testing"
)

func TestCompileObjectSchema_ValidJSON(t *testing.T) {
	out := CompileObjectSchema([]string{"name", "age"}, []string{"name"})
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("emitted schema is not valid JSON: %v", err)
	}
	if m["type"] != "object" {
		t.Fatalf("top-level type = %v, want object", m["type"])
	}
}

func TestCompileObjectSchema_AdditionalPropertiesFalse(t *testing.T) {
	out := CompileObjectSchema([]string{"name", "age"}, []string{"name"})
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	ap, ok := m["additionalProperties"]
	if !ok {
		t.Fatalf("missing additionalProperties")
	}
	if ap != false {
		t.Fatalf("additionalProperties = %v, want false", ap)
	}
}
EOF
