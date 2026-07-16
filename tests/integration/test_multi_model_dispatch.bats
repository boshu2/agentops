#!/usr/bin/env bats
# Conformance: multi-model judgment via fake runner (no real codex/ntm).
# Covers council, idea-genie duel, cross-model validate evidence, and
# diversity_unsatisfied degradation.

REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
RUNNER="$REPO_ROOT/skills/agent-native/scripts/fake_model_runner.py"
VALIDATE_CHALLENGE="$REPO_ROOT/skills/idea-genie/scripts/validate-challenge.sh"

setup() {
  FIX="$(mktemp -d)"
  export FIX
}

teardown() {
  rm -rf "$FIX"
}

@test "council two-model round records identities and distinct context ids" {
  run python3 "$RUNNER" council \
    --models "fable-a,fable-b" \
    --available "fable-a,fable-b" \
    --output "$FIX/council.json"
  [ "$status" -eq 0 ]
  python3 - "$FIX/council.json" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
assert r["schema_version"] == "council-report.v1"
assert len(r["judges"]) == 2
ids = [j["context_id"] for j in r["judges"]]
assert len(set(ids)) == 2
models = [j["model_identity"] for j in r["judges"]]
assert models == ["fable-a", "fable-b"]
assert r["agreement"]["cross_model"] is True
assert r["diversity_unsatisfied"] == []
for j in r["judges"]:
    assert "methodology" in j
print("council ok")
PY
}

@test "duel two-model sealed packet validates and records model_identity" {
  run python3 "$RUNNER" duel \
    --models "model-x,model-y" \
    --available "model-x,model-y" \
    --output "$FIX/idea-challenge.json"
  [ "$status" -eq 0 ]
  run bash "$VALIDATE_CHALLENGE" "$FIX/idea-challenge.json"
  [ "$status" -eq 0 ]
  python3 - "$FIX/idea-challenge.json" <<'PY'
import json, sys
p = json.load(open(sys.argv[1]))
assert p["sealed_generation"] is True
assert len(p["perspectives"]) == 2
assert {x["model_identity"] for x in p["perspectives"]} == {"model-x", "model-y"}
ctx = [x["context_id"] for x in p["perspectives"]]
assert len(set(ctx)) == 2
print("duel ok")
PY
}

@test "cross-model validate evidence records distinct models and context ids" {
  run python3 "$RUNNER" validate-cross \
    --author-model "author-model" \
    --validator-model "validator-model" \
    --author-context-id "author-ctx-1" \
    --validator-context-id "validator-ctx-1" \
    --available "author-model,validator-model" \
    --output "$FIX/validate-evidence.json"
  [ "$status" -eq 0 ]
  python3 - "$FIX/validate-evidence.json" <<'PY'
import json, sys
e = json.load(open(sys.argv[1]))
assert e["author_model_identity"] == "author-model"
assert e["validator_model_identity"] == "validator-model"
assert e["author_context_id"] != e["validator_context_id"]
assert e["diversity_unsatisfied"] == []
assert "author_model=" in e["freshness_attestation"]["notes"]
print("validate-cross ok")
PY
}

@test "unavailable model discloses diversity_unsatisfied and degrades" {
  run python3 "$RUNNER" council \
    --models "live-model,missing-model" \
    --available "live-model" \
    --output "$FIX/council-degrade.json"
  [ "$status" -eq 0 ]
  python3 - "$FIX/council-degrade.json" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
assert "missing-model" in r["diversity_unsatisfied"]
assert r["agreement"]["single_model_only"] is True
print("council degrade ok")
PY

  run python3 "$RUNNER" duel \
    --models "live-model,missing-model" \
    --available "live-model" \
    --output "$FIX/duel-degrade.json"
  [ "$status" -eq 0 ]
  [ -f "$FIX/duel-degrade.diversity.json" ]
  run bash "$VALIDATE_CHALLENGE" "$FIX/duel-degrade.json"
  [ "$status" -eq 0 ]

  run python3 "$RUNNER" validate-cross \
    --author-model "live-model" \
    --validator-model "missing-model" \
    --available "live-model" \
    --output "$FIX/validate-degrade.json"
  [ "$status" -eq 0 ]
  python3 - "$FIX/validate-degrade.json" <<'PY'
import json, sys
e = json.load(open(sys.argv[1]))
assert e["diversity_unsatisfied"] == ["missing-model"]
assert e["validator_model_identity"] == e["author_model_identity"] == "live-model"
print("validate degrade ok")
PY
}

@test "no claude -p or --print in model-dispatch or fake runner" {
  run grep -E 'claude -p|claude --print' \
    "$REPO_ROOT/skills/agent-native/references/model-dispatch.md" \
    "$REPO_ROOT/skills/agent-native/scripts/fake_model_runner.py"
  # grep finds the prohibition prose ("Never invoke claude -p") — that is fine;
  # fail only if an invocation recipe appears (pipe to claude -p, etc.).
  run grep -E '(^|[^"])claude -p|(^|[^"])claude --print' \
    "$REPO_ROOT/skills/agent-native/scripts/fake_model_runner.py"
  [ "$status" -ne 0 ]
  run grep -E 'codex exec|command -v ntm|command -v codex' \
    "$REPO_ROOT/skills/agent-native/references/model-dispatch.md"
  [ "$status" -eq 0 ]
}
