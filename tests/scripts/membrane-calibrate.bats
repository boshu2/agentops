#!/usr/bin/env bats
# Stubbed-reviewer tests for scripts/membrane-calibrate.sh — the standing membrane
# calibration harness (age-e508.2). No live codex/agy: the reviewer is a STUB that
# prints a canned VERDICT, so the full producer→oracle→classify→evidence→trend
# pipeline is exercised deterministically over the FROZEN weak-producer trap
# corpus. These pin the calibration CONTRACT (dated evidence file, verbatim
# per-trap outcomes, aggregate rates, honest trend, plain REGRESSION on a drop).

setup() {
	REPO="$BATS_TEST_DIRNAME/../.."
	HARNESS="$REPO/scripts/membrane-calibrate.sh"
	OUT="$(mktemp -d)"
	command -v go >/dev/null 2>&1 || skip "go not on PATH (the frozen corpus oracle needs it)"
	# A stub reviewer that always REFUTEs: catches every false-done (catch_rate=1.0)
	# and also wrongly refutes both controls (false_refute_rate=1.0).
	REFUTE_STUB='printf "VERDICT: REFUTE\nWHY: stub refute\n"'
	# A stub reviewer that always ACKs: misses every false-done (catch_rate=0.0)
	# and never false-refutes a control (false_refute_rate=0.0).
	ACK_STUB='printf "VERDICT: ACK\nWHY: stub ack\n"'
}

# run_harness <label> <stub-cmd> <now-stamp> [extra args...]
run_harness() {
	local label="$1" stub="$2" now="$3"
	shift 3
	run bash "$HARNESS" --membrane-cmd "$stub" --membrane-label "$label" \
		--out-dir "$OUT" --now "$now" "$@"
}

@test "one invocation writes a dated evidence file with per-trap verbatim + aggregate rates" {
	run_harness stub-refute "$REFUTE_STUB" 2026-01-01
	[ "$status" -eq 0 ]
	ev="$OUT/membrane-calibration-stub-refute-2026-01-01.md"
	[ -f "$ev" ]
	for t in fd-no-mutate fd-buried-req fd-regression cleaner-median hard-utf8-truncate; do
		grep -q "$t" "$ev"
	done
	grep -qi "catch_rate" "$ev"
	grep -qi "false_refute_rate" "$ev"
	# per-trap verbatim: the stub's own WHY text is reproduced (not summarized)
	grep -q "stub refute" "$ev"
	sc="$OUT/membrane-calibration-stub-refute-2026-01-01.scorecard.json"
	[ -f "$sc" ]
	python3 -c "import json;json.load(open('$sc'))"
}

@test "the all-REFUTE stub yields catch_rate 1.0 and false_refute_rate 1.0" {
	run_harness stub-refute "$REFUTE_STUB" 2026-01-02
	[ "$status" -eq 0 ]
	sc="$OUT/membrane-calibration-stub-refute-2026-01-02.scorecard.json"
	run python3 -c "import json;d=json.load(open('$sc'));print(d['rates']['catch_rate'],d['rates']['false_refute_rate'])"
	[ "$output" = "1.0 1.0" ]
}

@test "the all-ACK stub yields catch_rate 0.0 and false_refute_rate 0.0" {
	run_harness stub-ack "$ACK_STUB" 2026-01-03
	[ "$status" -eq 0 ]
	sc="$OUT/membrane-calibration-stub-ack-2026-01-03.scorecard.json"
	run python3 -c "import json;d=json.load(open('$sc'));print(d['rates']['catch_rate'],d['rates']['false_refute_rate'])"
	[ "$output" = "0.0 0.0" ]
}

@test "reproducible: two runs of the same stub over the same corpus give identical per-task classes" {
	run_harness stub-refute "$REFUTE_STUB" 2026-02-01
	[ "$status" -eq 0 ]
	a="$(python3 -c "import json;d=json.load(open('$OUT/membrane-calibration-stub-refute-2026-02-01.scorecard.json'));print(sorted((x['task'],x['class']) for x in d['per_task']))")"
	run_harness stub-refute "$REFUTE_STUB" 2026-02-02
	[ "$status" -eq 0 ]
	b="$(python3 -c "import json;d=json.load(open('$OUT/membrane-calibration-stub-refute-2026-02-02.scorecard.json'));print(sorted((x['task'],x['class']) for x in d['per_task']))")"
	[ "$a" = "$b" ]
}

@test "a second run with a dropped catch-rate over the SAME corpus is flagged REGRESSION with a trend diff" {
	run_harness gemini-fallback "$REFUTE_STUB" 2026-03-01
	[ "$status" -eq 0 ]
	run_harness gemini-fallback "$ACK_STUB" 2026-03-02
	[ "$status" -eq 0 ]
	ev2="$OUT/membrane-calibration-gemini-fallback-2026-03-02.md"
	grep -q "REGRESSION" "$ev2"
	grep -qi "Trend" "$ev2"
	grep -qi "prior" "$ev2"
}

@test "the first run for an adapter is labeled BASELINE (no prior to diff)" {
	run_harness fresh-adapter "$REFUTE_STUB" 2026-04-01
	[ "$status" -eq 0 ]
	grep -q "BASELINE" "$OUT/membrane-calibration-fresh-adapter-2026-04-01.md"
}

@test "the honesty header names ADR-0011 (calibration, NOT corpus-compounding evidence)" {
	run_harness stub-refute "$REFUTE_STUB" 2026-05-01
	[ "$status" -eq 0 ]
	grep -q "ADR-0011" "$OUT/membrane-calibration-stub-refute-2026-05-01.md"
}

@test "per-adapter calibration: two adapters keep separate trend histories" {
	run_harness adapter-a "$REFUTE_STUB" 2026-06-01
	[ "$status" -eq 0 ]
	# adapter-b's FIRST run must be BASELINE, not compared against adapter-a.
	run_harness adapter-b "$ACK_STUB" 2026-06-02
	[ "$status" -eq 0 ]
	grep -q "BASELINE" "$OUT/membrane-calibration-adapter-b-2026-06-02.md"
}

@test "--help documents the token/time budget" {
	run bash "$HARNESS" --help
	[ "$status" -eq 0 ]
	echo "$output" | grep -qi "budget"
}

@test "same-date/same-label re-run never overwrites the first evidence (append-only integrity, ebec-kin refute-fix)" {
	run_harness dup "$REFUTE_STUB" 2026-06-01
	[ "$status" -eq 0 ]
	first="$OUT/membrane-calibration-dup-2026-06-01.md"
	[ -f "$first" ]
	grep -q "stub refute" "$first"
	# Second run: SAME label + SAME --now, DIFFERENT reviewer. Must NOT clobber the first.
	run_harness dup "$ACK_STUB" 2026-06-01
	second="$OUT/membrane-calibration-dup-2026-06-01-2.md"
	[ -f "$second" ]                         # disambiguated file created
	grep -q "stub ack" "$second"
	[ -f "$first" ]                          # first survives
	grep -q "stub refute" "$first"           # ...with its ORIGINAL content
	run grep -q "stub ack" "$first"; [ "$status" -ne 0 ]   # first NOT overwritten by the ack run
	# Append-only history: two rows pointing at two DISTINCT evidence files (no corrupt pointer).
	hist="$OUT/membrane-calibration-history.jsonl"
	grep -qF "membrane-calibration-dup-2026-06-01.md" "$hist"
	grep -qF "membrane-calibration-dup-2026-06-01-2.md" "$hist"
}
