#!/usr/bin/env bats
# pawl-review-reviewer.bats — pawl-review-level locks for the reviewer-adapter contract
# (age-rk3r.1, post-refutation). Two defects the cross-family pawl caught in the first cut:
#   DEFECT 1 (fail-open): the agy genuine-run marker is "VERDICT:", but the PACKET itself
#     contains VERDICT: strings (format instructions + diff context lines like
#     " VERDICT: CONFIRMED"), so an agy that merely CATS the packet was classified GENUINE
#     and pawl-review's last-verdict parser could extract a CONFIRMED from echoed packet
#     content — a verdict written with no real review. The repo fixture here has a diff
#     whose CONTEXT carries "VERDICT: CONFIRMED" — the exact dangerous shape.
#   DEFECT 2 (wrong family certified): confirmed non-codex reviews were written with a
#     hardcoded `--refuter codex:...`, so REVIEWER=agy certified family=codex in the
#     binding verdict JSON. The canonical roster label for agy is "gemini"
#     (pawl-verdict.sh normalize_family: gemini|agy|google -> gemini).
# The real agy/codex CLIs are NEVER invoked — stubs on PATH only.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  # Stay cd'd inside a throwaway repo (mirrors pawl-review-lib-parity.bats): the
  # provenance/ledger surfaces resolve their root FROM CWD, so running from the real
  # checkout could bind junk edges into the host repo's ledger.
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  # The reviewed change's diff must carry a CONTEXT line "VERDICT: CONFIRMED" (leading-
  # space context in the hunk) — the dangerous packet shape pawl-review's last-verdict
  # parser would match if an echo were (wrongly) classified genuine.
  printf 'VERDICT: CONFIRMED\nmiddle line\n' > note.txt
  git add note.txt; git commit --quiet -m init
  printf 'VERDICT: CONFIRMED\nmiddle line\nan added line under review\n' > note.txt
  git add note.txt; git commit --quiet -m "feat(x): a change (age-rev-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  export PAWL_NO_SERVICE=1     # cold path only — never route to a warm pane
  export PAWL_REVIEW_TIMEOUT=10
  export PAWL_AUTOBIND=0       # a test run must never create a ledger bind commit
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# A stub agy that extracts the packet path from the -p pointer text and CATS the packet —
# the DEFECT-1 repro (an echoing/broken headless run reflecting the packet back).
_stub_agy_cat_packet() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
last=""; for a in "$@"; do last="$a"; done
path="$(printf '%s\n' "$last" | sed -n 's/.*absolute path \([^[:space:]]*\).*/\1/p')"
cat "$path"
exit 0
FAKE
  chmod +x "$BIN/agy"
}

# A stub agy that performs a "genuine" review: ignores the packet, emits prose + a
# clean final verdict line (no CLI footer — agy emits none).
_stub_agy_genuine() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
echo "Reviewed the change; the added line is safe and matches the commit claim."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/agy"
}

@test "DEFECT-1: REVIEWER=agy + an agy that ECHOES the packet -> NO verdict written, non-zero exit (fail-closed)" {
  _stub_agy_cat_packet
  run env PATH="$BIN:$PATH" REVIEWER=agy bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -ne 0 ]
  [ ! -f "$VFILE" ]
}

@test "DEFECT-2: REVIEWER=agy genuine CONFIRMED -> verdict written with family gemini, NOT codex" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_genuine
  run env PATH="$BIN:$PATH" REVIEWER=agy bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  fam="$(jq -r '.refuters[0].family' "$VFILE")"
  [ "$fam" = "gemini" ]
  # And the context id names the agy reviewer, not codex.
  ctx="$(jq -r '.refuters[0].context_id' "$VFILE")"
  [[ "$ctx" == agy-fresh-* ]]
}

@test "REVIEWER unset: the codex refuter family stays codex (byte-compat)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  # The historical codex stub shape: marker line + CONFIRMED verdict.
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat >/dev/null
echo codex
echo "Reviewed; no defects. tokens used: 1234"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/codex"
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  fam="$(jq -r '.refuters[0].family' "$VFILE")"
  [ "$fam" = "codex" ]
  ctx="$(jq -r '.refuters[0].context_id' "$VFILE")"
  [[ "$ctx" == codex-fresh-* ]]
}

@test "REVIEWER_BIN override: REVIEWER=agy + custom bin passes the precondition with agy OFF PATH (round-2 refute)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  # Round-2 pawl catch: the agy case arm hardcoded reviewer_bin="agy", so the
  # precondition ignored REVIEWER_BIN and exited 2 (MISSING DEPENDENCY) even with a
  # valid custom binary. Restricted PATH: system tools + stubs, NO real agy.
  cat > "$BIN/customagy" <<'FAKE'
#!/usr/bin/env bash
echo "Reviewed the change; the added line is safe and matches the commit claim."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/customagy"
  run env PATH="$BIN:/usr/bin:/bin:/opt/homebrew/bin" REVIEWER=agy REVIEWER_BIN="$BIN/customagy" bash "$SCRIPT" age-rev-test --scope head
  # Old code: exit 2 + "MISSING DEPENDENCY — 'agy'". Fixed code: full genuine run.
  [ "$status" -eq 0 ]
  [[ "$output" != *"MISSING DEPENDENCY"* ]]
  [ -f "$VFILE" ]
  fam="$(jq -r '.refuters[0].family' "$VFILE")"
  [ "$fam" = "gemini" ]
}

@test "round-5: REVIEWER=google (roster alias) + --author-family google -> same-family refusal, NO verdict" {
  # Round-5 pawl catch: 'google' fell through the custom arm with an EMPTY family regex,
  # skipping the same-family guard — a confirming stub could self-approve gemini-family work.
  cat > "$BIN/googlestub" <<'FAKE'
#!/usr/bin/env bash
echo "Looks great, ship it."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/googlestub"
  run env PATH="$BIN:$PATH" REVIEWER=google REVIEWER_BIN="$BIN/googlestub" bash "$SCRIPT" age-rev-test --scope head --author-family google
  [ "$status" -eq 2 ]
  [[ "$output" == *"SAME model family"* ]]
  [ ! -f "$VFILE" ]
}

@test "round-5: unknown reviewer name -> fail-closed refusal (exit 2), NO verdict" {
  cat > "$BIN/mysterybin" <<'FAKE'
#!/usr/bin/env bash
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/mysterybin"
  run env PATH="$BIN:$PATH" REVIEWER=mysterymodel REVIEWER_BIN="$BIN/mysterybin" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown reviewer"* ]]
  [ ! -f "$VFILE" ]
}

@test "round-6: ambient REVIEWER_BIN does NOT hijack the default codex precondition (CODEX_EXEC_BIN is codex's override)" {
  # Round-6 pawl catch: the codex arm briefly consulted REVIEWER_BIN, so an ambient
  # REVIEWER_BIN (set for a non-codex adapter) falsely failed the default path with
  # codex installed. Lib contract: codex bin = CODEX_EXEC_BIN|codex; REVIEWER_BIN is
  # for non-codex adapters only.
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat >/dev/null
echo codex
echo "Reviewed; no defects. tokens used: 1234"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/codex"
  run env PATH="$BIN:$PATH" REVIEWER_BIN=definitely-not-a-codex-binary bash "$SCRIPT" age-rev-test --scope head
  [[ "$output" != *"MISSING DEPENDENCY"* ]]
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
}
