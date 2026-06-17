#!/usr/bin/env bash
# Deterministic oracle for rfd-codex-schema. Emits one JSON line {"score","total","pass"}.
#
# Pins the STRICT structured-outputs contract that actually 400s codex:
#   - emitted schema is valid JSON
#   - additionalProperties:false on the object
#   - EVERY key in `properties` appears in `required` (the non-obvious rule)
# A half-right schema (additionalProperties:false but only "obviously required"
# props in required) passes the visible test but FAILS here.
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

// TestOracle_StrictContract asserts the full STRICT structured-outputs contract
// for several property/required shapes. The decisive case is when `required`
// passed in is a STRICT SUBSET of `props`: a correct compiler must still mark
// ALL properties required, or codex --output-schema returns 400.
func TestOracle_StrictContract(t *testing.T) {
	cases := []struct {
		props    []string
		required []string
	}{
		{[]string{"name", "age"}, []string{"name"}},          // subset -> all must be required
		{[]string{"a", "b", "c"}, []string{}},                // none declared -> all must be required
		{[]string{"only"}, []string{"only"}},                 // already complete
		{[]string{"x", "y", "z"}, []string{"x", "y", "z"}},   // already complete
	}

	for _, c := range cases {
		out := CompileObjectSchema(c.props, c.required)

		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("props=%v: not valid JSON: %v", c.props, err)
		}

		if ap, ok := m["additionalProperties"]; !ok || ap != false {
			t.Fatalf("props=%v: additionalProperties must be false, got %v (present=%v)", c.props, ap, ok)
		}

		props, ok := m["properties"].(map[string]any)
		if !ok {
			t.Fatalf("props=%v: properties missing or not an object", c.props)
		}

		reqRaw, ok := m["required"].([]any)
		if !ok {
			t.Fatalf("props=%v: required missing or not an array", c.props)
		}
		reqSet := map[string]bool{}
		for _, r := range reqRaw {
			if s, ok := r.(string); ok {
				reqSet[s] = true
			}
		}

		// STRICT rule: every property must appear in required.
		for k := range props {
			if !reqSet[k] {
				t.Fatalf("props=%v: property %q missing from required (strict mode 400s)", c.props, k)
			}
		}
		// And required must not name a non-existent property.
		for r := range reqSet {
			if _, present := props[r]; !present {
				t.Fatalf("props=%v: required names unknown property %q", c.props, r)
			}
		}
	}
}
EOF

score=0
total=1

if go test -run 'TestOracle_StrictContract' ./schema/ >/dev/null 2>&1; then
  score=$((score + 1))
fi

rm -f "$ORACLE"

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
