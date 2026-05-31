#!/usr/bin/env bats
#
# Format-check for the ag-x31t.7 orphan fixtures. The consuming gate (ag-x31t.6,
# `ao provenance trace --orphans --strict`) is not built yet, so this test only
# verifies the seed fixtures are well-formed JSON/JSONL and internally consistent
# with the expected-orphans.json contract — i.e. that they are usable the moment
# the gate lands.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    DIR="$ROOT/tests/fixtures/provenance"
    MANIFEST="$DIR/expected-orphans.json"
}

@test "expected-orphans manifest is valid JSON with three fixtures" {
    run jq -e '.fixtures | length == 3' "$MANIFEST"
    [ "$status" -eq 0 ]
}

@test "every manifest fixture file exists and is valid JSONL" {
    files="$(jq -r '.fixtures[].file' "$MANIFEST")"
    for f in $files; do
        path="$DIR/$f"
        [ -f "$path" ]
        # Each non-blank line must parse as a standalone JSON object.
        while IFS= read -r line; do
            [ -z "$line" ] && continue
            echo "$line" | jq -e 'type == "object"' >/dev/null
        done < "$path"
    done
}

@test "each fixture seeds its declared orphan artifact node" {
    count="$(jq '.fixtures | length' "$MANIFEST")"
    for i in $(seq 0 $((count - 1))); do
        f="$(jq -r ".fixtures[$i].file" "$MANIFEST")"
        orphan="$(jq -r ".fixtures[$i].orphan_artifact_id" "$MANIFEST")"
        # The orphan id must appear as an artifact node in the fixture...
        found="$(jq -rs --arg id "$orphan" \
            'map(select(.record=="node" and .type=="artifact" and .id==$id)) | length' \
            "$DIR/$f")"
        [ "$found" -eq 1 ]
        # ...and must NOT be the target of any edge (no inbound provenance edge).
        inbound="$(jq -rs --arg id "$orphan" \
            'map(select(.record=="edge" and .to_id==$id)) | length' \
            "$DIR/$f")"
        [ "$inbound" -eq 0 ]
    done
}

@test "each fixture's expected finding names the orphan artifact path" {
    count="$(jq '.fixtures | length' "$MANIFEST")"
    for i in $(seq 0 $((count - 1))); do
        sev="$(jq -r ".fixtures[$i].expected_finding.severity" "$MANIFEST")"
        path="$(jq -r ".fixtures[$i].expected_finding.path" "$MANIFEST")"
        [ "$sev" = "error" ]
        [ -n "$path" ]
        [ "$path" != "null" ]
    done
}
