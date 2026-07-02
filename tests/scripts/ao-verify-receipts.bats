#!/usr/bin/env bats
# ao-verify-receipts.bats — `ao verify receipts` renders a repo's membrane-receipts
# proof page from the EMBEDDED generator on the stranger path (no AgentOps checkout):
# every number derived from that repo's OWN provenance ledger, chain-verify gated,
# fail-closed on tamper, output landing in the target repo. (age-rk3r.12)
#
# The ledger is built with the PRODUCTION writer (`ao provenance emit-verdict`) over
# real commits in a throwaway git repo — the real persisted shape, never a hand-built
# fixture. The throwaway repo is NOT an AgentOps checkout and the ao binary lives
# outside it, so `ao verify receipts` deterministically takes the embedded path.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  # Build ao from THIS source (it must carry the new `verify receipts` command) to a
  # path OUTSIDE the agentops checkout, so `ao verify receipts` run inside the
  # throwaway repo takes the stranger/embedded path (aoBinaryInside is false and the
  # repo is not an AgentOps checkout). NEVER use a PATH ao — it may predate this cmd.
  AO_BIN="$BATS_FILE_TMPDIR/ao"
  (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao)
  export AO_BIN
  for tool in git jq; do
    command -v "$tool" >/dev/null 2>&1 || { echo "# missing required tool: $tool" >&3; return 1; }
  done
}

# emit_verdict <bead> <sha> <disposition> — write a real pawl-verdict artifact and
# append its verdict->commit edge to the ledger via the production Go writer.
emit_verdict() {
  local bead="$1" sha="$2" disp="$3"
  local f="$REPO/.agents/pawl-verdicts/${bead}-${sha:0:7}-${disp}.json"
  printf '{"bead_id":"%s","head_sha":"%s","disposition":"%s"}\n' "$bead" "$sha" "$disp" > "$f"
  ( cd "$REPO" && "$AO_BIN" provenance emit-verdict --file "$f" ) >/dev/null
}

setup() {
  # Throwaway git repo (NOT an AgentOps checkout) under the bats-managed tmp, which
  # is outside the agentops checkout → the stranger path is forced.
  REPO="$BATS_TEST_TMPDIR/stranger"
  mkdir -p "$REPO/.agents/pawl-verdicts"
  git -C "$REPO" init --quiet
  git -C "$REPO" config user.email t@e.com
  git -C "$REPO" config user.name T
  printf 'v1\n' > "$REPO/app.txt"; git -C "$REPO" add app.txt; git -C "$REPO" commit --quiet -m "feat: base (str-1)"
  SHA_A="$(git -C "$REPO" rev-parse HEAD)"
  printf 'v2\n' >> "$REPO/app.txt"; git -C "$REPO" add app.txt; git -C "$REPO" commit --quiet -m "fix: patch (str-1)"
  SHA_B="$(git -C "$REPO" rev-parse HEAD)"
  # A REFUTED-then-fixed arc for str-1 (refuted on A, CONFIRMED on B) + one plain
  # CONFIRMED for str-2 → 3 verdict edges, 2 CONFIRMED / 1 REFUTED / 1 caught arc.
  emit_verdict str-1 "$SHA_A" REFUTED
  emit_verdict str-1 "$SHA_B" CONFIRMED
  emit_verdict str-2 "$SHA_B" CONFIRMED
  LEDGER="$REPO/docs/provenance/ledger.jsonl"
  OUT_MD="$REPO/docs/evidence/membrane-receipts.md"
  OUT_JSON="$REPO/docs/releases/membrane-receipts.json"
}

run_receipts() {
  run bash -c "cd '$REPO' && '$AO_BIN' verify receipts"
  echo "# ao verify receipts exit=$status" >&3
  printf '%s\n' "$output" | sed 's/^/# /' >&3
}

jqv() { jq -r "$1" "$OUT_JSON"; }

@test "ao verify receipts renders the proof page from the repo's OWN ledger (stranger/embedded path)" {
  run_receipts
  [ "$status" -eq 0 ]
  [ -f "$OUT_MD" ]
  [ -f "$OUT_JSON" ]

  echo "# every number derived from THIS repo's 3-edge ledger" >&3
  [ "$(jqv '.totals.ledger_records')" = "3" ]
  [ "$(jqv '.totals.verdict_events')" = "3" ]
  [ "$(jqv '.dispositions.CONFIRMED')" = "2" ]
  [ "$(jqv '.dispositions.REFUTED')" = "1" ]
  [ "$(jqv '.caught_defects.refuted_then_fixed')" = "1" ]
  [ "$(jqv '.source.chain_verified')" = "true" ]

  echo "# the human page carries the same derived numbers + the real short SHA" >&3
  grep -Fq '| Ledger records | 3 |' "$OUT_MD"
  grep -Fq '| CONFIRMED verdicts | 2 |' "$OUT_MD"
  grep -Fq '| REFUTED verdicts | 1 |' "$OUT_MD"
  grep -Fq '| REFUTED-then-fixed arcs (caught defects) | 1 |' "$OUT_MD"
  grep -Fq "${SHA_A:0:7}" "$OUT_MD"
  grep -Eq '^Generated: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$' "$OUT_MD"

  echo "# output landed in the TARGET repo docs/evidence + docs/releases" >&3
  [ -f "$REPO/docs/evidence/membrane-receipts.md" ]
  [ -f "$REPO/docs/releases/membrane-receipts.json" ]
}

@test "ao verify receipts REFUSES on a tampered ledger, names the break, writes nothing (fail-closed)" {
  run_receipts
  [ "$status" -eq 0 ]
  before="$(shasum "$OUT_MD" | cut -d' ' -f1)"

  echo "# tamper: flip a disposition without re-hashing the chain" >&3
  sed 's/disposition=REFUTED/disposition=CONFIRMED/' "$LEDGER" > "$LEDGER.t" && mv "$LEDGER.t" "$LEDGER"

  run_receipts
  [ "$status" -eq 1 ]
  [[ "$output" == *"REFUSING to render"* ]]
  [[ "$output" == *"verification FAILED"* ]]
  [[ "$output" == *"payload_hash mismatch"* ]]

  echo "# the refused run wrote nothing — the prior page is untouched" >&3
  after="$(shasum "$OUT_MD" | cut -d' ' -f1)"
  [ "$before" = "$after" ]
}

@test "ao verify receipts (embedded) matches the in-repo generator byte-for-byte (modulo Generated)" {
  run_receipts
  [ "$status" -eq 0 ]
  cp "$OUT_MD" "$BATS_TEST_TMPDIR/via-cmd.md"

  echo "# the canonical in-repo generator on the SAME ledger must produce the same page" >&3
  run bash -c "cd '$REPO' && env PROVENANCE_LEDGER='$LEDGER' \
    RECEIPTS_MD='$BATS_TEST_TMPDIR/via-script.md' RECEIPTS_JSON='$BATS_TEST_TMPDIR/via-script.json' \
    AO_BIN='$AO_BIN' '$REPO_ROOT/scripts/gen-membrane-receipts.sh'"
  [ "$status" -eq 0 ]

  diff <(grep -v '^Generated: ' "$BATS_TEST_TMPDIR/via-cmd.md") \
       <(grep -v '^Generated: ' "$BATS_TEST_TMPDIR/via-script.md")
}
