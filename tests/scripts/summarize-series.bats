#!/usr/bin/env bats
# Regression tests for evals/membrane/harvest-2026-06-22/summarize-series.py — the
# I-MR control-chart analysis over the escape-rate series E5 reads. The SPC math is
# subtle and a silent regression would corrupt the governed-metric analysis, so each
# behavior is pinned: moving-range sigma (robust to a single outlier), I-MR limits,
# OOC detection over CLEAN rows only, the degenerate (sigma=0) guard, the n<8 caveat,
# and the degraded-row exclusion. Driven by fixture JSONL series (no model calls).

setup() {
  SUMM="$BATS_TEST_DIRNAME/../../evals/membrane/harvest-2026-06-22/summarize-series.py"
  FIX="$(mktemp -d)"
}
teardown() { rm -rf "$FIX"; }

# Write a row whose fields are MUTUALLY CONSISTENT (the real persisted shape):
# membrane_miss_rate == n_missed/n_false_dones and catch_rate == n_caught/n_false_dones,
# n_caught+n_missed == n_false_dones. fd=100 so any 2-decimal rate is exact. A fixture
# whose rate disagreed with its counts would be invalid input and could mask a
# counts-vs-rate cross-check bug.
row() { # $1=run $2=miss_rate $3=degraded
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
run, miss, deg = sys.argv[1], float(sys.argv[2]), int(sys.argv[3])
fd = 100
missed = round(miss * fd)
caught = fd - missed
print(json.dumps({"run_id": run, "membrane_miss_rate": missed / fd, "catch_rate": caught / fd,
                  "n_false_dones": fd, "n_caught": caught, "n_missed": missed,
                  "degraded": deg}))
PY
}

@test "moving-range I-MR flags a single outlier the in-sample stddev would swallow" {
  # 8 tight points ~0.23 + one 0.95 spike. In-sample 3-sigma would balloon and miss
  # it; the MR-based sigma keeps the band tight and flags it.
  : > "$FIX/s.jsonl"
  for r in 0.25 0.20 0.30 0.22 0.28 0.24 0.26 0.21 0.95; do row "p$r" "$r" 0 >> "$FIX/s.jsonl"; done
  run python3 "$SUMM" "$FIX/s.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OUT-OF-CONTROL"* ]]
  [[ "$output" == *"0.95"* ]]
  # I-MR upper limit must be well below 1.0 (the in-sample method gave UCL>1.0).
  [[ "$output" == *"UCL=0.702"* ]]
}

@test "two identical clean points => degenerate limits, no false OOC flag" {
  row a 0.20 0 > "$FIX/s.jsonl"; row b 0.20 0 >> "$FIX/s.jsonl"
  run python3 "$SUMM" "$FIX/s.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *"DEGENERATE"* ]]
  [[ "$output" != *"OUT-OF-CONTROL"* ]]
}

@test "degraded rows are excluded from the metric AND the control-limit math" {
  # Clean 0.20 + 0.30 (mean 0.25) + a degraded 0.95 spike. The DISCRIMINATOR is the
  # centerline: clean-only CL=0.250; if the degraded spike were wrongly included the
  # CL would be (0.20+0.30+0.95)/3 = 0.483. (Asserting only "no OOC" would NOT
  # discriminate — including the spike inflates its own band so it's never flagged
  # either way.) The pooled rate is likewise clean-only (50/200=0.25 vs 145/300 if
  # included).
  row a 0.20 0 > "$FIX/s.jsonl"; row b 0.30 0 >> "$FIX/s.jsonl"; row spike 0.95 2 >> "$FIX/s.jsonl"
  run python3 "$SUMM" "$FIX/s.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 clean"* ]]
  [[ "$output" == *"1 degraded"* ]]
  [[ "$output" == *"CL=0.250"* ]]   # centerline from CLEAN only — the real discriminator
  [[ "$output" == *"pooled over CLEAN runs: 50 miss(es) / 200 false-done(s) = 0.250"* ]]
  [[ "$output" != *"OUT-OF-CONTROL"* ]]
}

@test "n<8 prints the provisional caveat, never an 'in statistical control' claim" {
  for r in 0.20 0.30 0.25; do row "p$r" "$r" 0 >> "$FIX/s.jsonl"; done
  run python3 "$SUMM" "$FIX/s.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *"< 8"* ]]
  [[ "$output" != *"process in statistical control"* ]]
}
