#!/usr/bin/env bats
#
# Behavioral spec for scripts/evidence-orphans.sh — the evidence-orphan
# receipt (not a registered gate: exit 0 is "ran to completion", never "no
# orphans"; exit 2 is "the scan did not run to completion").
#
# WHY: a probe scorecard, a fixture-set, and a capture contract bind exact
# files by path AND sha256 digest (scripts/lib/probe-fixture-metadata.py's
# verify_evaluator_files: harness, preamble, metadata_helper, dispatch_helper,
# network_proxy; plus the canonical_skill snapshot the capture was taken
# against). Editing one of those bound files silently orphans every artifact
# that recorded the OLD digest. This receipt names the exposure at edit time
# instead of at the next unrelated probe run.
#
# Fixture note: capture-contract.json files under evals/skill-probes/** carry
# NO evaluator binding in this repository (checked against every instance on
# 2026-09-05) — but they DO carry a canonical_skill snapshot, which is why the
# scanner reads them for that key alone. These fixtures build the real bound
# shapes.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    SCRIPT="$REPO_ROOT/scripts/evidence-orphans.sh"
    export SCRIPT

    FIX="$BATS_TEST_TMPDIR/fixture-repo"
    mkdir -p "$FIX/docs/evals/scorecards" "$FIX/evals/skill-probes/probe-a/fixtures-low" \
        "$FIX/scripts/lib" "$FIX/skills/demo"
    export FIX

    printf 'echo harness\n' > "$FIX/scripts/harness.sh"
    printf 'echo preamble\n' > "$FIX/scripts/lib/preamble.sh"
    printf -- '---\nname: demo\ndescription: a demo skill\n---\n\nbody\n' > "$FIX/skills/demo/SKILL.md"
    HARNESS_SHA="sha256:$(shasum -a 256 "$FIX/scripts/harness.sh" | awk '{print $1}')"
    PREAMBLE_SHA="sha256:$(shasum -a 256 "$FIX/scripts/lib/preamble.sh" | awk '{print $1}')"
    SKILL_SHA="sha256:$(shasum -a 256 "$FIX/skills/demo/SKILL.md" | awk '{print $1}')"
    export HARNESS_SHA PREAMBLE_SHA SKILL_SHA
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

write_fixture_set() {
    local skill_sha="$1"
    cat > "$FIX/evals/skill-probes/probe-a/fixtures-low/fixture-set.json" <<JSON
{
  "capture_evaluator": {"harness": {"path": "scripts/harness.sh", "sha256": "$HARNESS_SHA"}},
  "canonical_skill": {"name": "demo", "path": "skills/demo/SKILL.md", "sha256": "$skill_sha"}
}
JSON
}

# --- causes ----------------------------------------------------------------

@test "a binding on a changed path is orphaned with cause changed_path" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"artifact": "docs/evals/scorecards/sc.json"'* ]]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *'"cause": "changed_path"'* ]]
}

@test "a digest mismatch is orphaned with cause digest_drift regardless of the changed list" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "sha256:$(printf '0%.0s' $(seq 64))" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *'"cause": "digest_drift"'* ]]
    [[ "$output" == *"\"current_sha256\": \"$HARNESS_SHA\""* ]]
}

@test "a changed path whose digest also drifted reports cause both" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "sha256:$(printf '0%.0s' $(seq 64))" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"cause": "both"'* ]]
    [[ "$output" != *'"cause": "changed_path"'* ]]
}

@test "a bound file that no longer exists is orphaned, not silently unknown" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"
    rm "$FIX/scripts/harness.sh"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *'"cause": "digest_drift"'* ]]
    [[ "$output" == *'"current_sha256": null'* ]]
}

@test "a bound path that is now a symlink is orphaned rather than followed" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"
    rm "$FIX/scripts/harness.sh"
    ln -s lib/preamble.sh "$FIX/scripts/harness.sh"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
    [[ "$output" == *'"current_sha256": null'* ]]
}

@test "matching digests and an unrelated changed path report zero of both counts" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/some-other-file.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binding_count": 0'* ]]
    [[ "$output" == *'"artifact_count": 0'* ]]
    [[ "$output" == *'"orphaned": []'* ]]
}

# --- counts ----------------------------------------------------------------

@test "binding and artifact counts are reported separately" {
    # One harness edit orphans two bindings across two artifacts; a single
    # number could not say which of the two moved.
    write_scorecard "$FIX/docs/evals/scorecards/one.json" "$HARNESS_SHA" "$PREAMBLE_SHA"
    write_scorecard "$FIX/docs/evals/scorecards/two.json" "$HARNESS_SHA" "$PREAMBLE_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh scripts/lib/preamble.sh
    [ "$status" -eq 0 ]
    # Two artifacts, and each artifact's two bound paths are distinct bindings.
    [[ "$output" == *'"binding_count": 4'* ]]
    [[ "$output" == *'"artifact_count": 2'* ]]
}

# --- skill-snapshot orphaning ---------------------------------------------

@test "a fixture-set whose canonical skill still matches is not orphaned by it" {
    write_fixture_set "$SKILL_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" != *'"cause": "skill_changed"'* ]]
    [[ "$output" == *'"binding_count": 0'* ]]
}

@test "a fixture-set bound to a rewritten SKILL.md reports cause skill_changed" {
    write_fixture_set "$SKILL_SHA"
    printf -- '---\nname: demo\ndescription: a rewritten demo skill\n---\n\nnew body\n' > "$FIX/skills/demo/SKILL.md"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binds": "skills/demo/SKILL.md"'* ]]
    [[ "$output" == *'"cause": "skill_changed"'* ]]
}

@test "a capture-contract canonical skill snapshot is scanned too" {
    cat > "$FIX/evals/skill-probes/probe-a/fixtures-low/capture-contract.json" <<JSON
{"canonical_skill": {"name": "demo", "path": "skills/demo/SKILL.md", "sha256": "sha256:$(printf '0%.0s' $(seq 64))"}}
JSON

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'capture-contract.json'* ]]
    [[ "$output" == *'"cause": "skill_changed"'* ]]
}

@test "a canonical skill whose SKILL.md is gone is orphaned, not a crash" {
    write_fixture_set "$SKILL_SHA"
    rm -r "$FIX/skills/demo"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"cause": "skill_changed"'* ]]
    [[ "$output" == *'"current_sha256": null'* ]]
}

@test "a fixture-set capture_evaluator binding is still scanned alongside the skill" {
    write_fixture_set "$SKILL_SHA"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"artifact": "evals/skill-probes/probe-a/fixtures-low/fixture-set.json"'* ]]
    [[ "$output" == *'"binds": "scripts/harness.sh"'* ]]
}

# --- fail-closed -----------------------------------------------------------

@test "malformed JSON in a scanned artifact fails closed" {
    printf '{not valid json' > "$FIX/docs/evals/scorecards/broken.json"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"malformed JSON"* ]]
}

@test "a binding whose path is not a string fails closed" {
    cat > "$FIX/docs/evals/scorecards/sc.json" <<'JSON'
{"evaluator": {"harness": {"path": 7, "sha256": "sha256:aa"}}}
JSON

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"path is not a string"* ]]
}

@test "a binding whose sha256 is missing fails closed" {
    cat > "$FIX/docs/evals/scorecards/sc.json" <<'JSON'
{"evaluator": {"harness": {"path": "scripts/harness.sh"}}}
JSON

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"sha256 is not a string"* ]]
}

@test "an evaluator block that is not an object fails closed" {
    printf '{"evaluator": ["harness"]}\n' > "$FIX/docs/evals/scorecards/sc.json"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"not an object"* ]]
}

@test "an explicitly null evaluator block is a declared absence, not a malformed shape" {
    printf '{"evaluator": null, "capture_evaluator": null}\n' > "$FIX/docs/evals/scorecards/sc.json"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binding_count": 0'* ]]
}

@test "a malformed canonical_skill record fails closed" {
    cat > "$FIX/evals/skill-probes/probe-a/fixtures-low/fixture-set.json" <<'JSON'
{"canonical_skill": {"name": "demo", "path": "skills/demo/SKILL.md"}}
JSON

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/unrelated.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"sha256 is not a string"* ]]
}

@test "an unknown option is refused rather than silently ignored" {
    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" --text scripts/harness.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown option --text"* ]]
}

@test "the retired --text formatter is gone, not merely unreachable" {
    # --text had no consumer. Removing the flag while leaving the formatter is
    # how a retired surface comes back; assert the code is gone.
    run bash -c "grep -c 'out_format\|OK: no evidence binding orphaned' '$SCRIPT' || true"
    [ "$output" = "0" ]
}

@test "an unreadable scanned directory fails closed instead of reporting no orphans" {
    write_scorecard "$FIX/docs/evals/scorecards/sc.json" "$HARNESS_SHA" "$PREAMBLE_SHA"
    mkdir -p "$FIX/docs/evals/scorecards/locked"
    chmod 000 "$FIX/docs/evals/scorecards/locked"

    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/harness.sh
    chmod 755 "$FIX/docs/evals/scorecards/locked"
    [ "$status" -eq 2 ]
    [[ "$output" == *"cannot walk"* ]]
    [[ "$output" == *"did not run to completion"* ]]
}

# --- shape and real-repository smoke --------------------------------------

@test "no scorecards or fixture-sets present reports an empty receipt" {
    run env EVIDENCE_ORPHANS_ROOT="$FIX" bash "$SCRIPT" scripts/anything.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"binding_count": 0'* ]]
    [[ "$output" == *'"artifact_count": 0'* ]]
}

@test "run against the real repository completes and exits 0" {
    run bash "$SCRIPT" scripts/probe-skill.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *'"changed":'* ]]
    [[ "$output" == *'"binding_count":'* ]]
    [[ "$output" == *'"artifact_count":'* ]]
    # Every entry carries a cause from the declared vocabulary.
    python3 - <<PY
import json
data = json.loads('''$output''')
allowed = {"changed_path", "digest_drift", "both", "skill_changed"}
bad = [o for o in data["orphaned"] if o.get("cause") not in allowed]
assert not bad, bad
assert data["artifact_count"] <= data["binding_count"]
PY
}
