#!/usr/bin/env bash
# Deterministic oracle for hard-deep-merge. A shallow merge REPLACES a nested map
# instead of merging it, dropping keys present only in dst. Pin that nested keys
# from BOTH sides survive.
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"
ORACLE="config/oracle_zz_test.go"
cat > "$ORACLE" <<'GO'
package config

import "testing"

func TestOracle_DeepMerge(t *testing.T) {
	dst := map[string]any{
		"svc": map[string]any{"host": "a", "port": 80},
		"top": 1,
	}
	src := map[string]any{
		"svc": map[string]any{"port": 443, "tls": true},
		"new": 2,
	}
	got := Merge(dst, src)
	svc, ok := got["svc"].(map[string]any)
	if !ok {
		t.Fatalf("svc is not a map: %T", got["svc"])
	}
	// host came ONLY from dst — a shallow merge would have dropped it.
	if svc["host"] != "a" {
		t.Fatalf("deep merge dropped dst-only nested key svc.host: %v", svc)
	}
	if svc["port"] != 443 || svc["tls"] != true {
		t.Fatalf("src nested keys not applied: %v", svc)
	}
	if got["top"] != 1 || got["new"] != 2 {
		t.Fatalf("top-level merge wrong: %v", got)
	}
}
GO
score=0; total=1
if go test -run 'TestOracle_DeepMerge' ./config/ >/dev/null 2>&1; then score=1; fi
rm -f "$ORACLE"
pass=false; [ "$score" -eq "$total" ] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
