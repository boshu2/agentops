#!/usr/bin/env bats
#
# Behavioral spec for scripts/evidence-orphans.sh — the evidence-orphan
# receipt (not a registered gate: exit 0 is "ran to completion", never "no
# orphans").
#
# WHY: a probe scorecard and a fixture-set both bind an exact evaluator file
# by path AND sha256 digest (scripts/lib/probe-fixture-metadata.py's
# verify_evaluator_files: harness, preamble, metadata_helper, dispatch_helper,
# network_proxy). Editing one of those bound files silently orphans every
# scorecard/fixture-set that recorded the OLD digest. This receipt names the
# exposure at edit time instead of at the next unrelated probe run.
#
# Fixture note: capture-contract.json files under evals/skill-probes/** carry
# NO evaluator binding in this repository (checked against every instance on
# 2026-09-05) — only docs/evals/scorecards/**/*.json (`evaluator` +
# `capture_evaluator`) and evals/skill-probes/**/fixture-set.json
# (`capture_evaluator`) do. These fixtures build the real bound shape.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    SCRIPT="$REPO_ROOT/scripts/evidence-orphans.sh"
    export SCRIPT

    FIX="$BATS_TEST_TMPDIR/fixture-repo"
    mkdir -p "$FIX/docs/evals/scorecards" "$FIX/evals/skill-probes/probe-a/fixtures-low" "$FIX/scripts/lib"
    export FIX

    printf 'echo harness\n' > "$FIX/scripts/harness.sh"
    printf 'echo preamble\n' > "$FIX/scripts/lib/preamble.sh"
    HARNESS_SHA="sha256:$(shasum -a 256 "$FIX/scripts/harness.sh" | awk '{print $1}')"
    PREAMBLE_SHA="sha256:$(shasum -a 256 "$FIX/scripts/lib/preamble.sh" | awk '{print $1}')"
    export HARNESS_SHA PREAMBLE_SHA
}

write_scorecard() {
    local path="$1" harness_sha="$2" preamble_sha="$3"
    cat > "$path" <<JSON
{
  "evaluator": {
    "harness": {"path": "scripts/harness.sh", "sha256": "$harness_sha"},
    "preamble": {"path": "scripts/lib/preamble.sh", "sha256": "$preamble_sha"}
  },
  "capture_evaluator": {
    "harness": {"path": "scripts/harness.sh", "sha256": "$harness_sha"},
    "preamble": {"path": "scripts/lib/preamble.sh", "sha256": "$preamble_sha"}
  }
}
JSON
}

@test "a scorecard binding a changed path reports it orphaned" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"artifact": "docs/evals/scorecards/sc.json"'* ]]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *'"count": 1'* ]]
}

@test "an unrelated changed path with matching digests reports count 0" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/some-other-file.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"count": 0'* ]]
    [[ "$output" == *'"orphaned": []'* ]]
}

@test "a digest mismatch is flagged independent of the changed-path list" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "sha256:0000000000000000000000000000000000000000000000000000000000000" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *"\"recorded_sha256\": \"sha256:0000000000000000000000000000000000000000000000000000000000000\""* ]]
    [[ "$output" == *"\"current_sha256\": \"$HARNESS_SHA\""* ]]
}

@test "a fixture-set.json capture_evaluator binding is scanned" {
    cat > "$FIX/evals/skill-probes/probe-a/fixtures-low/fixture-set.json" <<JSON
{"capture_evaluator": {"harness": {"path": "scripts/harness.sh", "sha256": "$HARNESS_SHA"}}}
JSON

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"artifact": "evals/skill-probes/probe-a/fixtures-low/fixture-set.json"'* ]]
}

@test "malformed JSON in a scanned artifact fails closed" {
    printf '{not valid json' > "$FIX/docs/evals/scorecards/broken.json"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"malformed JSON"* ]]
}

@test "--text mode prints an OK line when nothing is orphaned" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" --text scripts/unrelated-path.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK: no evidence binding orphaned"* ]]
}

@test "--text mode prints one line per orphan and a trailing count" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" --text scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"docs/evals/scorecards/sc.json: binds scripts/harness.sh"* ]]
    [[ "$output" == *"count=1"* ]]
}

@test "no scorecards or fixture-sets present reports an empty receipt" {
    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/anything.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"count": 0'* ]]
}

@test "run against the real repository completes and exits 0" {
    run bash "$SCRIPT" scripts/probe-skill.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"changed":'* ]]
    [[ "$output" == *'"count":'* ]]
}
