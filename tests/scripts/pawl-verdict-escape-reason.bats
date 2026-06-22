#!/usr/bin/env bats
# EM.2.1 (age-membrane-memory-arch-tz2s.2.1): a REFUTED verdict is a candidate
# ESCAPE. `pawl-verdict.sh write` must REQUIRE a non-empty --reason on any REFUTED
# (what was missed) — the conscious write is where classification belongs — while
# leaving CONFIRMED unaffected. The domain is guaranteed downstream by the Go
# writer's UNCLASSIFIED sentinel, so it is NOT hard-required at the script (a
# missing domain must never exit-1 the fail-open auto-logger and drop the catch).
#
# `ao` is STUBBED via PATH (hermetic); `jq` is real.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"; ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  cat >"$TMP/bin/ao" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$PATH"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"
  printf 'real fresh-context review ran\n' > "$TMP/evidence.txt"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "REFUTED write WITHOUT --reason is refused (exit 2) — a candidate escape must be classified" {
  run bash "$SCRIPT" write age-esc-noreason 100 \
    --disposition REFUTED --head "$SHA" \
    --author-context author-ctx \
    --refuter "claude:REFUTED:fresh-reviewer-ctx:$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 2 ]
  [[ "$output" == *"--reason is REQUIRED"* ]]
  [ ! -f "$TMP/verdicts/age-esc-noreason.json" ]  # nothing written on refusal
}

@test "REFUTED write WITH --reason is accepted (exit 0)" {
  run bash "$SCRIPT" write age-esc-reason 101 \
    --disposition REFUTED --head "$SHA" \
    --author-context author-ctx \
    --refuter "claude:REFUTED:fresh-reviewer-ctx:$TMP/evidence.txt" \
    --reason "missed: nil-deref on the empty-base path" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  [ -f "$TMP/verdicts/age-esc-reason.json" ]
}

@test "CONFIRMED write WITHOUT --reason is unaffected (exit 0) — REQUIRE is REFUTED-only" {
  run bash "$SCRIPT" write age-conf-noreason 102 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter "claude:CONFIRMED:fresh-reviewer-ctx:$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  [ -f "$TMP/verdicts/age-conf-noreason.json" ]
}
