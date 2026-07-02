#!/usr/bin/env bats
# pawl-review-timeout-scale.bats — age-iian (FOLDED into age-rk3r.1) regression lock.
# pawl-review.sh scales the review timeout UP for a large read-files diff
# (scale_review_timeout), but the kill-budget snapshot (REVIEW_TIMEOUT, consumed by the
# cold exec wrapper) was captured BEFORE the scaling ran — so the auto-scaled budget never
# reached the cold reviewer kill. This test drives the REAL pawl-review.sh over a large diff
# and proves the SCALED budget is what reaches the timeout wrapper: a STUB `timeout` records
# the budget it was invoked with. FAILS on the pre-fix flow (records the unscaled 300),
# PASSES with the re-snapshot (records the scaled value). The real codex is NEVER invoked.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  LIB="$REPO_ROOT/scripts/lib/codex-exec.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  # A LARGE committed change (~20KB) so diff_bytes >> the tiny inline cap => read-files +
  # timeout scaling. printf a big blob into a tracked file, then commit it.
  { for i in $(seq 1 400); do printf 'line %04d: the quick brown fox jumps over the lazy dog %04d\n' "$i" "$i"; done; } > big.txt
  git add big.txt
  git commit --quiet -m "feat(x): a large change (age-rev-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  export PAWL_NO_SERVICE=1        # force the cold path (never the warm tmux service)
  export PAWL_MAX_INLINE_BYTES=100  # tiny cap => the diff is "large" => read-files + scaling
  export PAWL_AUTOBIND=0
  # DO NOT set PAWL_REVIEW_TIMEOUT — an explicit pin disables scaling (scale_review_timeout).
  unset PAWL_REVIEW_TIMEOUT || true

  # A stub `timeout` that RECORDS the budget ($1) it was handed, then execs the rest — this
  # is the exec wrapper the scaled budget must reach.
  cat > "$BIN/timeout" <<FAKE
#!/usr/bin/env bash
printf '%s' "\$1" > "$TMP/recorded-budget"
shift
exec "\$@"
FAKE
  chmod +x "$BIN/timeout"
  # A codex stub that emits a genuine (marker-bearing) REFUTED review, so the flow exits at
  # the REFUTED branch WITHOUT reaching the verdict-write machinery (keeps the test light).
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat >/dev/null
echo codex
echo "Reviewed. tokens used: 5"
echo "VERDICT: REFUTED"
exit 0
FAKE
  chmod +x "$BIN/codex"
  # jq only needs to EXIST for the precondition (never executed on the REFUTED path); ao is a
  # no-op so emit_pawl_catch cannot reach a real ao.
  printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/jq"; chmod +x "$BIN/jq"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/ao"; chmod +x "$BIN/ao"
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

@test "age-iian: the SCALED review timeout reaches the cold exec wrapper (not the unscaled 300)" {
  command -v git >/dev/null 2>&1 || skip "git required"
  # The pawl-review timeout wrapper is `timeout <budget> codex ...`; our stub records <budget>.
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ -f "$TMP/recorded-budget" ]
  recorded="$(cat "$TMP/recorded-budget")"

  # Compute the EXPECTED scaled budget the SAME way pawl-review does: source it (the source
  # guard returns before the main flow) to get scale_review_timeout, replicate diff_bytes.
  diff="$(git -C "$REPO" show HEAD --no-ext-diff --no-textconv --no-color 2>/dev/null)"
  diff_bytes="$(printf '%s' "$diff" | wc -c | tr -d ' ')"
  # shellcheck source=/dev/null
  source "$SCRIPT" 2>/dev/null || true
  expected="$(scale_review_timeout "$diff_bytes" 100 300 "")"

  # Sanity: scaling actually happened (the diff is large enough to raise the budget past 300).
  [ "$expected" -gt 300 ]
  # The budget that reached the exec wrapper is the SCALED one, not the unscaled default.
  [ "$recorded" = "$expected" ]
  [ "$recorded" -ne 300 ]
}
