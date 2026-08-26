#!/usr/bin/env bats
#
# Behavioral test for the skill.probe-headroom gate,
# scripts/check-skill-probe-headroom.sh, and its helper cli/cmd/probe-headroom.
#
# The acceptance this pins is the RED-fixture flip: two committed scorecard
# pairs that carry the SAME probe verdict (INERT) must classify DIFFERENTLY —
# the saturated pair flagged, the INERT-with-headroom pair not. A detector that
# reads the verdict, or one that flags every INERT, fails here.

setup_file() {
	ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export ROOT
	export PROBE_HEADROOM_BIN="$ROOT/cli/bin/probe-headroom"
	if [[ ! -x "$PROBE_HEADROOM_BIN" ]]; then
		(cd "$ROOT/cli" && go build -o bin/probe-headroom ./cmd/probe-headroom)
	fi
}

setup() {
	ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export PROBE_HEADROOM_BIN="$ROOT/cli/bin/probe-headroom"
	GATE="$ROOT/scripts/check-skill-probe-headroom.sh"
	FIXTURES="$ROOT/tests/fixtures/skill-probes"
}

@test "gate passes: the saturated pair is flagged and the headroom pair is not" {
	run bash "$GATE" --no-scan
	[ "$status" -eq 0 ]
	[[ "$output" == *"saturated pair: flagged"* ]]
	[[ "$output" == *"headroom pair:  not flagged"* ]]
}

@test "both committed fixture pairs carry the same INERT verdict" {
	# If this drifts, the gate stops proving anything: a detector could pass by
	# reading the verdict instead of the control arm's absolute rate.
	run grep -l '"verdict": "INERT"' \
		"$FIXTURES/saturated/fixture-saturated-quiz-low.json" \
		"$FIXTURES/saturated/fixture-saturated-quiz-xhigh.json" \
		"$FIXTURES/headroom/fixture-headroom-quiz-low.json" \
		"$FIXTURES/headroom/fixture-headroom-quiz-xhigh.json"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 4 ]
}

@test "saturated pair: control aces two effort levels, helper exits 3" {
	run "$PROBE_HEADROOM_BIN" "$FIXTURES/saturated/fixture-saturated-quiz-low.json" \
		"$FIXTURES/saturated/fixture-saturated-quiz-xhigh.json"
	[ "$status" -eq 3 ]
	[[ "$output" == *"PROBE_HEADROOM: SATURATED"* ]]
	[[ "$output" == *"2 effort levels"* ]]
}

@test "INERT-with-headroom pair: control leaves room, helper exits 0" {
	run "$PROBE_HEADROOM_BIN" "$FIXTURES/headroom/fixture-headroom-quiz-low.json" \
		"$FIXTURES/headroom/fixture-headroom-quiz-xhigh.json"
	[ "$status" -eq 0 ]
	[[ "$output" == *"PROBE_HEADROOM: SEPARATED"* ]]
}

@test "one saturated effort level alone is not enough to flag" {
	run "$PROBE_HEADROOM_BIN" "$FIXTURES/saturated/fixture-saturated-quiz-low.json"
	[ "$status" -eq 0 ]
	[[ "$output" == *"PROBE_HEADROOM: SEPARATED"* ]]
}

@test "gate fails loudly when a fixture pair is missing" {
	tmpdir="$(mktemp -d)"
	mkdir -p "$tmpdir/saturated"
	run env PROBE_HEADROOM_FIXTURES="$tmpdir" bash "$GATE" --no-scan
	[ "$status" -eq 2 ]
	[[ "$output" == *"fixture pair missing"* ]]
	rm -rf "$tmpdir"
}

@test "gate fails when the detector stops discriminating" {
	# Negative control: swap the headroom pair's bytes for saturated ones. The
	# gate must now FAIL, proving its assertion is load-bearing rather than
	# decorative.
	tmpdir="$(mktemp -d)"
	mkdir -p "$tmpdir/saturated" "$tmpdir/headroom"
	cp "$FIXTURES/saturated"/*.json "$tmpdir/saturated/"
	cp "$FIXTURES/saturated"/*.json "$tmpdir/headroom/"
	run env PROBE_HEADROOM_FIXTURES="$tmpdir" bash "$GATE" --no-scan
	[ "$status" -eq 1 ]
	[[ "$output" == *"WAS flagged"* ]]
	rm -rf "$tmpdir"
}

@test "advisory sweep never fails on a saturated historical group" {
	run bash "$GATE"
	[ "$status" -eq 0 ]
	[[ "$output" == *"PROBE_HEADROOM_SCAN:"* ]]
	[[ "$output" == *"SATURATED"* ]]
}

@test "helper rejects a foreign schema instead of classifying it" {
	tmpdir="$(mktemp -d)"
	printf '{"schema":"verdict.v2","probe":"x"}\n' > "$tmpdir/foreign.json"
	run "$PROBE_HEADROOM_BIN" "$tmpdir/foreign.json"
	[ "$status" -eq 2 ]
	[[ "$output" == *"agentops-skill-probe."* ]]
	rm -rf "$tmpdir"
}

@test "helper exits 2 on usage error" {
	run "$PROBE_HEADROOM_BIN"
	[ "$status" -eq 2 ]
	[[ "$output" == *"usage"* ]]
}
