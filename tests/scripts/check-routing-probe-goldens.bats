#!/usr/bin/env bats
# check-routing-probe-goldens.bats — acceptance for the retrieval-eval grader.
#
# The contract under test is discrimination: the grader must PASS a pack that
# satisfies its golden and FAIL, by named finding, every way a pack can miss it.
# Most cases are hermetic — a stub routing surface emits a canned pack, so what
# is graded is the GRADER, not today's skill catalog. One case grades the real
# repository so the committed goldens cannot rot unnoticed.
#
# Fixture tree per test: $FIX/{scripts,scripts/lib,schemas,goldens,bin,skills}
# with the real gate + preamble + contract copied in, a stub `ao`, and goldens
# written by helper. Fully offline; never touches the real repo.

bats_require_minimum_version 1.5.0

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GATE_SRC="$REPO_ROOT/scripts/check-routing-probe-goldens.sh"
    SCHEMA_SRC="$REPO_ROOT/schemas/pack-quality-expectations.v1.schema.json"

    FIX="$BATS_TEST_TMPDIR/repo"
    GOLDENS="$FIX/goldens"
    mkdir -p "$FIX/scripts/lib" "$FIX/schemas" "$GOLDENS" "$FIX/bin" "$FIX/skills"
    cp "$GATE_SRC" "$FIX/scripts/check-routing-probe-goldens.sh"
    cp "$REPO_ROOT/scripts/lib/preamble.sh" "$FIX/scripts/lib/preamble.sh"
    cp "$SCHEMA_SRC" "$FIX/schemas/pack-quality-expectations.v1.schema.json"

    GATE="$FIX/scripts/check-routing-probe-goldens.sh"

    # Stub routing surface: deterministic, ignores the query, emits the pack the
    # test declared. Keeps every hermetic case independent of the live catalog.
    cat > "$FIX/bin/ao" <<EOF
#!/usr/bin/env bash
cat "$FIX/pack.json"
EOF
    chmod +x "$FIX/bin/ao"

    export ROUTING_GOLDENS_DIR="$GOLDENS"
    export ROUTING_GOLDENS_SCHEMA="$FIX/schemas/pack-quality-expectations.v1.schema.json"
    export ROUTING_GOLDENS_AO_BIN="$FIX/bin/ao"
}

# make_pack SPEC... — SPEC is "name:description_word_count:provenance(1|0)".
# provenance 1 writes a real SKILL.md the pack can cite; 0 cites a dead path.
make_pack() {
    python3 - "$FIX" "$@" <<'PY'
import json, os, sys

fixture_root = sys.argv[1]
pack = []
for spec in sys.argv[2:]:
    name, words, resolvable = spec.split(":")
    directory = os.path.join(fixture_root, "skills", name)
    path = os.path.join(directory, "SKILL.md")
    if resolvable == "1":
        os.makedirs(directory, exist_ok=True)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("fixture skill\n")
    else:
        path = os.path.join(directory, "never-written.md")
    pack.append({
        "name": name,
        "description": " ".join(["word"] * int(words)),
        "path": path,
        "score": 0.5,
    })
with open(os.path.join(fixture_root, "pack.json"), "w", encoding="utf-8") as handle:
    json.dump(pack, handle)
PY
}

# make_golden ID [JSON_OVERRIDES] — writes a contract-valid golden, then applies
# the caller's overrides so each test states only the field it is about.
make_golden() {
    python3 - "$GOLDENS/$1.json" "$1" "${2:-{\}}" <<'PY'
import json, sys

path, fixture_id, overrides = sys.argv[1:4]
golden = {
    "schema_version": 1,
    "id": fixture_id,
    "retrieval_surface": "ao-skills-find",
    "task": "fixture task",
    "query": "fixture query",
    "top_k": 3,
    "expected_selected_ids": ["alpha"],
    "critical_omitted_ids": [],
    "forbidden_leaks": [],
    "min_provenance_density": 1.0,
    "max_tokens": 100,
}
golden.update(json.loads(overrides))
with open(path, "w", encoding="utf-8") as handle:
    json.dump(golden, handle)
PY
}

json_field() {
    python3 -c '
import json, sys
value = json.loads(sys.argv[1])
for part in sys.argv[2].split("."):
    value = value[part]
print(value)
' "$1" "$2"
}

@test "a pack that satisfies its golden passes" {
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden pass-case

    run bash "$GATE"

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"rank1=alpha"* ]]
    [[ "$output" == *"1/1 goldens matched"* ]]
}

@test "an expected id missing from the pack is a MISS" {
    make_pack "beta:5:1" "gamma:5:1" "delta:5:1"
    make_golden miss-case

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"MISS: expected 'alpha' in the top-3 pack"* ]]
}

@test "a critical-omitted id present in the pack is a LEAK" {
    make_pack "alpha:5:1" "decoy:5:1" "gamma:5:1"
    make_golden leak-case '{"critical_omitted_ids": ["decoy"]}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"LEAK: 'decoy' must be omitted but ranked 2"* ]]
}

@test "a forbidden pattern matching rank 1 is a MISROUTE" {
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden misroute-case '{"forbidden_leaks": ["^alph"]}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"MISROUTE: rank-1 'alpha' matches forbidden pattern ^alph"* ]]
}

@test "a forbidden pattern matching a lower rank is not a MISROUTE" {
    # Rank 1 is the routing decision; a decoy that merely appears in the pack is
    # governed by critical_omitted_ids, not by forbidden_leaks.
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden lower-rank-case '{"forbidden_leaks": ["^beta$"]}'

    run bash "$GATE"

    [ "$status" -eq 0 ]
}

@test "a pack citing a dead path fails the provenance floor" {
    make_pack "alpha:5:1" "beta:5:0" "gamma:5:1"
    make_golden provenance-case

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"PROVENANCE: 0.67 of the pack cites a resolvable path"* ]]
}

@test "a pack over its declared token ceiling fails" {
    make_pack "alpha:40:1" "beta:40:1" "gamma:40:1"
    make_golden tokens-case '{"max_tokens": 100}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"TOKENS: pack is 120 tokens, over the declared ceiling of 100"* ]]
}

@test "only top_k results are graded" {
    # The surface may return more than the golden asked for; expectations are
    # declared over exactly top_k, so a hit beyond the cut is still a MISS.
    make_pack "beta:5:1" "gamma:5:1" "delta:5:1" "alpha:5:1"
    make_golden cutoff-case '{"top_k": 3}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"MISS: expected 'alpha'"* ]]

    make_golden cutoff-case '{"top_k": 4}'
    run bash "$GATE"
    [ "$status" -eq 0 ]
}

@test "a golden that violates the contract fails as SCHEMA and is not graded" {
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden schema-case '{"expected_selected_ids": [], "surprise": true}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"SCHEMA"* ]]
    [[ "$output" == *"needs at least 1 item"* ]]
    [[ "$output" == *"additional property 'surprise' not allowed"* ]]
    # Not graded means no routing finding was invented for an invalid fixture.
    [[ "$output" != *"MISS:"* ]]
}

@test "two goldens sharing an id fail as SCHEMA" {
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden first-file '{"id": "shared-id"}'
    make_golden second-file '{"id": "shared-id"}'

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"duplicate fixture id 'shared-id'"* ]]
}

@test "an empty goldens directory fails — zero fixtures is not a pass" {
    make_pack "alpha:5:1"

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"zero fixtures is zero measurement"* ]]
}

@test "a missing goldens directory fails" {
    make_pack "alpha:5:1"
    rm -rf "$GOLDENS"

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"goldens directory not found"* ]]
}

@test "an unreadable contract fails instead of grading against nothing" {
    make_pack "alpha:5:1" "beta:5:1" "gamma:5:1"
    make_golden contract-case
    printf 'not json\n' > "$ROUTING_GOLDENS_SCHEMA"

    run bash "$GATE"

    [ "$status" -eq 1 ]
    [[ "$output" == *"contract missing or not valid JSON"* ]]
}

@test "--json reports the denominator and every finding" {
    make_pack "beta:5:1" "gamma:5:1" "delta:5:1"
    make_golden good-one '{"expected_selected_ids": ["beta"]}'
    make_golden bad-one '{"expected_selected_ids": ["alpha"]}'

    run --separate-stderr bash "$GATE" --json

    [ "$status" -eq 1 ]
    [ "$(json_field "$output" total)" = "2" ]
    [ "$(json_field "$output" passed)" = "1" ]
    [ "$(json_field "$output" failed)" = "1" ]
}

@test "the repository's committed goldens grade with exactly the known finding" {
    # HONEST PIN, not a target. rq-04-independent-verdict is red on purpose:
    # `ao skills find` does not surface `validate` for the phrasings a caller
    # actually uses to ask for a verdict (evals/routing-probes/README.md,
    # "Standing finding"). This case asserts the CURRENT truth so the state
    # cannot drift silently. When the gap is closed — or a new one opens — this
    # test goes red and the pin below is updated to the new honest state.
    command -v go >/dev/null 2>&1 || skip "go toolchain unavailable; the nightly advisory job carries the real-corpus grade"
    unset ROUTING_GOLDENS_DIR ROUTING_GOLDENS_SCHEMA ROUTING_GOLDENS_AO_BIN

    run --separate-stderr bash "$REPO_ROOT/scripts/check-routing-probe-goldens.sh" --json

    [ "$status" -eq 1 ]
    [ "$(json_field "$output" total)" = "6" ]
    [ "$(json_field "$output" failed)" = "1" ]
    [[ "$output" == *"rq-04-independent-verdict"* ]]
}
