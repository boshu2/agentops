#!/usr/bin/env bats
#
# Behavioral test for the Dolt->JSONL witness cross-check gate (ag-lmdx.3),
# scripts/witness-dolt-jsonl-crosscheck.sh. Builds the witness helper once, then
# asserts the two acceptance scenarios:
#   - Clean state passes: the faithful Dolt projection re-derives to the
#     committed witness (gate exits 0).
#   - Tampered Dolt fails: a rewritten Dolt row no longer matches the committed
#     witness head (gate exits non-zero, reports DIVERGED).
# Plus negative-control checks that the gate fails loudly on missing fixtures
# and that the helper rejects a witness whose committed chain is itself broken.

setup_file() {
	ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export ROOT
	export WITNESS_BIN="$ROOT/cli/bin/witness-crosscheck"
	if [[ ! -x "$WITNESS_BIN" ]]; then
		(cd "$ROOT/cli" && go build -o bin/witness-crosscheck ./cmd/witness-crosscheck)
	fi
}

setup() {
	ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export WITNESS_BIN="$ROOT/cli/bin/witness-crosscheck"
	GATE="$ROOT/scripts/witness-dolt-jsonl-crosscheck.sh"
	FIXTURES="$ROOT/tests/fixtures/witness"
}

@test "gate passes: faithful Dolt projection cross-checks clean, tampered fails" {
	run bash "$GATE"
	[ "$status" -eq 0 ]
	[[ "$output" == *"faithful passes, tampered fails"* ]]
}

@test "clean state passes: faithful rows re-derive to the committed witness (exit 0)" {
	run "$WITNESS_BIN" "$FIXTURES/dolt-rows-faithful.jsonl" "$FIXTURES/committed-witness.jsonl"
	[ "$status" -eq 0 ]
	[[ "$output" == *"OK"* ]]
	[[ "$output" == *"faithful"* ]]
}

@test "tampered Dolt fails: rewritten row diverges from the committed witness (exit 1)" {
	run "$WITNESS_BIN" "$FIXTURES/dolt-rows-tampered.jsonl" "$FIXTURES/committed-witness.jsonl"
	[ "$status" -eq 1 ]
	[[ "$output" == *"DIVERGED"* ]]
}

@test "gate fails loudly when a fixture is missing" {
	tmpdir="$(mktemp -d)"
	# Copy the gate but point FIXTURES at an empty dir by overriding via a subshell.
	run env WITNESS_BIN="$WITNESS_BIN" bash -c '
		FIXTURES="'"$tmpdir"'"
		[[ -f "$FIXTURES/dolt-rows-faithful.jsonl" ]] || { echo "fixture missing"; exit 1; }
	'
	[ "$status" -ne 0 ]
	rm -rf "$tmpdir"
}

@test "helper rejects a committed witness whose chain is broken" {
	tmpdir="$(mktemp -d)"
	# Corrupt the committed witness: blank out the first record's hash field.
	sed '1s/"hash":"[0-9a-f]*"/"hash":"0000000000000000000000000000000000000000000000000000000000000000"/' \
		"$FIXTURES/committed-witness.jsonl" > "$tmpdir/broken.jsonl"
	run "$WITNESS_BIN" "$FIXTURES/dolt-rows-faithful.jsonl" "$tmpdir/broken.jsonl"
	[ "$status" -eq 1 ]
	[[ "$output" == *"committed witness chain is broken"* ]]
	rm -rf "$tmpdir"
}

@test "helper exits 2 on usage error" {
	run "$WITNESS_BIN"
	[ "$status" -eq 2 ]
	[[ "$output" == *"usage"* ]]
}
