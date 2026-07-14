#!/usr/bin/env bats
# em-loop-donetest.bats — terminal acceptance for the membrane self-improvement loop
# (epic age-membrane-memory-arch-tz2s; EM.3 fixture + EM.TEST e2e, .2.3/.2.6). Runs
# the real done-test harness, which builds + runs the real (shipped) ao binary over
# an ISOLATED temp project and asserts the loop CLOSES: escape -> mechanical
# constraint -> active -> blocks re-introduction. Never touches the real repo.
#
# Integration by nature. It SKIPS only on a THIN ENVIRONMENT (go / python3 absent).
# It NEVER skips on a build failure: the harness owns the `go build` and fails LOUDLY
# (`|| fail build`, exit 1), so a broken build is a real RED here, not a masked skip
# (the false-green a cross-family review caught in the first cut of this wrapper).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  HARNESS="$REPO_ROOT/scripts/em-loop-donetest.sh"
  # Skip ONLY on a thin environment (a tool genuinely absent). Everything about the
  # code's OWN artifacts (the harness file, its mode, the build) is a hard pass/fail,
  # never a skip — a skip on those would mask a committed regression as green.
  command -v go >/dev/null 2>&1 || skip "go not available (thin env — cannot build the shipped ao binary)"
  command -v python3 >/dev/null 2>&1 || skip "python3 not available (thin env)"
}

@test "EM-loop done-test: the membrane self-improvement loop closes e2e on the shipped binary" {
  # A MISSING harness is a real failure (not a skip). Run via `bash` so a lost +x
  # mode bit doesn't matter — the harness builds ao from the committed tree and
  # fails LOUDLY on any build/step failure, so a non-zero status is real, never skipped.
  [ -f "$HARNESS" ] || { echo "harness missing: $HARNESS"; false; }
  run bash "$HARNESS"
  [ "$status" -eq 0 ]
  [[ "$output" == *"EM LOOP CLOSED"* ]]
  # the load-bearing transitions are each asserted by the harness:
  [[ "$output" == *"COMPILE: escape -> draft constraint"* ]]   # the cut wire fires
  [[ "$output" == *"BLOCK"* ]]                                  # re-introduction is gate-blocked (the "blocks" half)
  [[ "$output" == *"activate guard"* ]]                         # a draft gates nothing (no false-green)
  [[ "$output" == *"LOAD: canonical Premortem"* ]]             # derived check retrievable by domain (the "auto-loads+cites" half, EM.4; ao lookup was archived in #906, so the harness reads the canonical premortem-checks dir directly)
  [[ "$output" == *"TRAVEL: published constraint enforces"* ]]  # learning travels to a clean CI checkout (EM.2.9)
}
