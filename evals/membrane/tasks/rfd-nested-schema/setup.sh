#!/usr/bin/env bash
# Stage the rfd-nested-schema workspace. macOS-portable: heredocs only.
# Generalization companion to rfd-codex-schema: the strict-mode invariant
# (required == all keys + additionalProperties:false) must hold RECURSIVELY on
# the nested `address` object. The VISIBLE test only checks the top level, so a
# producer that gets the top level right but leaves the nested object loose
# ships a false-done.
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/schema"

cat > "$WORKDIR/go.mod" <<'EOF'
module nestedschema

go 1.21
EOF

cat > "$WORKDIR/schema/schema.go" <<'EOF'
package schema

import "encoding/json"

// CompileUserSchema emits a JSON Schema (bytes) for a user object
//   { name string, address { street string, city string } }
// valid for OpenAI / codex --output-schema STRICT structured outputs.
//
// The top-level object is already wired correctly. Add the nested "address"
// object (street, city) to properties and to required.
func CompileUserSchema() []byte {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			// TODO: add "address" as a nested object with street + city
		},
		"required":             []string{"name"}, // TODO: include "address"
		"additionalProperties": false,
	}
	out, _ := json.Marshal(schema)
	return out
}
EOF

# VISIBLE (weak) test: only checks the TOP-LEVEL object is strict.
cat > "$WORKDIR/schema/schema_test.go" <<'EOF'
package schema

import (
	"encoding/json"
	"testing"
)

func TestTopLevelStrict(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(CompileUserSchema(), &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if m["type"] != "object" {
		t.Fatalf("top type = %v, want object", m["type"])
	}
	if m["additionalProperties"] != false {
		t.Fatalf("top additionalProperties = %v, want false", m["additionalProperties"])
	}
	req, _ := m["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("top required = %v, want [name address]", m["required"])
	}
}
EOF
