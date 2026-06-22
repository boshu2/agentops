#!/usr/bin/env bash
# Deterministic oracle for rfd-nested-schema. Emits one JSON line {"score","total","pass"}.
# The decisive (hidden) check: OpenAI strict mode is RECURSIVE — the NESTED
# `address` object must ALSO be strict (additionalProperties:false AND required
# lists every nested key). A top-level-only implementation fails here.
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

ORACLE="schema/oracle_zz_test.go"
cat > "$ORACLE" <<'EOF'
package schema

import (
	"encoding/json"
	"testing"
)

func oracleParse(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(CompileUserSchema(), &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	return m
}

// keySet returns the string keys of a JSON object node's named member as a set.
func keySet(node map[string]any, member string) map[string]bool {
	set := map[string]bool{}
	switch v := node[member].(type) {
	case map[string]any: // properties: object keyed by name
		for k := range v {
			set[k] = true
		}
	case []any: // required: array of names
		for _, k := range v {
			if s, ok := k.(string); ok {
				set[s] = true
			}
		}
	}
	return set
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// assertStrictObject enforces the FULL OpenAI strict-mode invariant on one object
// node: additionalProperties==false, `properties` has EXACTLY wantKeys, and
// `required` equals the property keyset (every property required, none dangling).
func assertStrictObject(t *testing.T, label string, node map[string]any, wantKeys ...string) {
	t.Helper()
	if node["additionalProperties"] != false {
		t.Fatalf("%s.additionalProperties = %v, want false", label, node["additionalProperties"])
	}
	props := keySet(node, "properties")
	want := map[string]bool{}
	for _, k := range wantKeys {
		want[k] = true
	}
	if !sameSet(props, want) {
		t.Fatalf("%s.properties keys = %v, want exactly %v", label, props, wantKeys)
	}
	// strict mode: required must equal the property keyset (not a subset, and not
	// referencing keys absent from properties).
	if req := keySet(node, "required"); !sameSet(req, props) {
		t.Fatalf("%s.required = %v, want it to equal properties keys %v", label, req, props)
	}
}

func TestOracle_TopStrict(t *testing.T) {
	assertStrictObject(t, "top", oracleParse(t), "name", "address")
}

func TestOracle_NestedAddressStrict(t *testing.T) {
	m := oracleParse(t)
	props, _ := m["properties"].(map[string]any)
	addr, ok := props["address"].(map[string]any)
	if !ok {
		t.Fatalf("properties.address missing or not an object")
	}
	if addr["type"] != "object" {
		t.Fatalf("address.type = %v, want object", addr["type"])
	}
	// RECURSIVE strict-mode invariant: the nested object must be fully strict too.
	assertStrictObject(t, "address", addr, "street", "city")
}
EOF

score=0
total=2
go test -run 'TestOracle_TopStrict' ./schema/ >/dev/null 2>&1 && score=$((score + 1))
go test -run 'TestOracle_NestedAddressStrict' ./schema/ >/dev/null 2>&1 && score=$((score + 1))

rm -f "$ORACLE"

pass=false
[ "$score" -eq "$total" ] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
