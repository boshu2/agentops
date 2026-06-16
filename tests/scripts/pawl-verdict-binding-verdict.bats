#!/usr/bin/env bats
# Binding-verdict join guard (age-d16-self-hosting-route-nkr.1 / M3).
#
# M3 proves the gate writes the BINDING verdict: a bead is accepted ONLY via a
# fresh-context pawl verdict that is ALSO recorded in the provenance ledger, and
# a self-approval (refuter context_id == author) is refused.
#
# The door-level coverage already exists and passes (tests/scripts/reconcile-pr.bats:
# test "fresh-context (default): refuter whose context_id == author: HOLD exit 5"
# is scenario 2 through reconcile-pr.sh; tests 23/25 are the authorize path).
# The ONE un-asserted seam this guards: the SAME verdict artifact that
# `pawl-verdict.sh check` authorizes (the accept half) is the one whose
# `pawl-verdict.sh write` fired `ao provenance emit-verdict` (the ledger half).
# If those two halves ever read/write different artifacts, "accepted" and
# "recorded in the ledger" silently decouple — that is the join this locks.
#
# `ao` is STUBBED via PATH (logs its args) so the test is hermetic; `jq` is real.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  AO_LOG="$TMP/ao.log"
  : > "$AO_LOG"
  cat >"$TMP/bin/ao" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$AO_LOG"
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

@test "scenario 1 join: the fresh-context verdict that check AUTHORIZES is the one whose write RECORDED it in the ledger" {
  # WRITE a valid fresh-context verdict (refuter context_id != author).
  run bash "$SCRIPT" write age-d16-join 941 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  local out="$TMP/verdicts/age-d16-join.json"
  [ -f "$out" ]

  # LEDGER half: write fired the provenance verdict sensor against THIS artifact.
  grep -q "provenance emit-verdict --file $out" "$AO_LOG"

  # ACCEPT half: check AUTHORIZES the SAME artifact (commit-current head) — exit 0.
  run bash "$SCRIPT" check age-d16-join 941 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
}

@test "scenario 2 control: a self-approval verdict (refuter context_id == author) is REFUSED by check" {
  # Same producer→check path, but the only refuter ran in the author's context.
  run bash "$SCRIPT" write age-d16-self 942 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:author-ctx:"$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]

  # check REFUSES (non-zero): no fresh red-team, so it cannot bind acceptance.
  run bash "$SCRIPT" check age-d16-self 942 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -ne 0 ]
  [[ "$output" == *"fresh-context"* ]]
}

@test "scenario 1 negative: a STALE verdict (head moved after review) cannot bind acceptance" {
  run bash "$SCRIPT" write age-d16-stale 943 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]

  # A new commit landed after review → current head != verdict head → refused.
  run bash "$SCRIPT" check age-d16-stale 943 --dir "$TMP/verdicts" --head "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
  [ "$status" -ne 0 ]
}
