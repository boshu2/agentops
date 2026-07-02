#!/usr/bin/env bash
# gen-membrane-receipts.sh — derive the public membrane-receipts proof page
# from the provenance ledger (docs/provenance/ledger.jsonl).
#
# Every number on the page is computed from the ledger; NOTHING is
# hand-written. Outputs:
#   docs/evidence/membrane-receipts.md    (human page)
#   docs/releases/membrane-receipts.json  (machine twin for claim citations)
#
# FAIL-CLOSED HONESTY: before rendering anything, this script runs
# `ao provenance verify` (the existing tamper-detection surface) against the
# ledger and REFUSES to render (nonzero exit, nothing written) when the
# hash chain does not verify. Chain verification is NOT reimplemented here.
#
# Determinism: output is byte-stable for the same ledger, modulo the single
# generated-at timestamp (one line in the .md, one field in the .json).
#
# Env overrides (used by tests):
#   PROVENANCE_LEDGER  ledger path (must be <root>/docs/provenance/ledger.jsonl)
#   RECEIPTS_MD        output markdown path
#   RECEIPTS_JSON      output json path
#   AO_BIN             ao binary to use (else PATH, else build from cli/)
#
# Exit codes:
#   0 = receipts rendered
#   1 = REFUSED: ledger chain verification failed (nothing written)
#   2 = script error (missing ledger, missing jq, bad layout)

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${PROVENANCE_LEDGER:-$REPO_ROOT/docs/provenance/ledger.jsonl}"
OUT_MD="${RECEIPTS_MD:-$REPO_ROOT/docs/evidence/membrane-receipts.md}"
OUT_JSON="${RECEIPTS_JSON:-$REPO_ROOT/docs/releases/membrane-receipts.json}"
AO_BIN="${AO_BIN:-}"

die() {
  echo "gen-membrane-receipts: ERROR — $*" >&2
  exit 2
}

command -v jq >/dev/null 2>&1 || die "jq is required"
[ -f "$LEDGER" ] || die "ledger not found at $LEDGER"

# `ao provenance verify` resolves the ledger relative to a repo root (a dir
# holding docs/ + schemas/, or a .git). To reuse it — never reimplement the
# chain check — the ledger must live at the canonical relative path.
case "$LEDGER" in
  */docs/provenance/ledger.jsonl) LEDGER_ROOT="${LEDGER%/docs/provenance/ledger.jsonl}" ;;
  *) die "ledger must live at <root>/docs/provenance/ledger.jsonl so 'ao provenance verify' can check it (got: $LEDGER)" ;;
esac

# Resolve the ao binary: explicit override, then PATH, then build from cli/.
if [ -z "$AO_BIN" ]; then
  if command -v ao >/dev/null 2>&1; then
    AO_BIN="$(command -v ao)"
  else
    AO_BIN="$(mktemp -d)/ao-receipts"
    echo "gen-membrane-receipts: ao not on PATH; building $AO_BIN" >&2
    (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao) || die "failed to build ao from cli/"
  fi
fi
[ -x "$AO_BIN" ] || die "ao binary not executable: $AO_BIN"

# ---- FAIL-CLOSED GATE: refuse to render on a broken/tampered chain --------
VERIFY_JSON="$(cd "$LEDGER_ROOT" && "$AO_BIN" provenance verify --json 2>&1)"
VERIFY_STATUS=$?
if [ "$VERIFY_STATUS" -ne 0 ]; then
  echo "gen-membrane-receipts: REFUSING to render — ledger chain verification FAILED" >&2
  echo "  ledger: $LEDGER" >&2
  echo "  verifier: $AO_BIN provenance verify" >&2
  echo "$VERIFY_JSON" | sed 's/^/  /' >&2
  echo "  A receipts page derived from an unverifiable ledger would be fabricated evidence." >&2
  exit 1
fi

VERIFIED_COUNT="$(printf '%s\n' "$VERIFY_JSON" | jq -r '.RecordCount' 2>/dev/null || echo "")"
LEDGER_COUNT="$(jq -s 'length' "$LEDGER")"
if [ -z "$VERIFIED_COUNT" ] || [ "$VERIFIED_COUNT" != "$LEDGER_COUNT" ]; then
  die "verifier saw $VERIFIED_COUNT record(s) but the ledger at $LEDGER has $LEDGER_COUNT — refusing (verify may have checked a different file)"
fi

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
TIP_HASH="$(jq -rs 'last | .hash' "$LEDGER")"

# ---- Derive every number from the ledger (single jq pass) -----------------
STATS_JSON="$(jq -s \
  --arg generated_at "$GENERATED_AT" \
  --arg tip_hash "$TIP_HASH" '
  # v1.1 (age-rk3r.3): prefer the STRUCTURED bead_id when present, regex-parse
  # the free-text evidence_ref only as the v1 fallback. bead_id is null on v1
  # verdict edges, so a pure-v1 ledger yields byte-identical output; a v1.1 edge
  # carries bead_id == the same bead the regex would have extracted. reviewer
  # family is already read structurally below via .reviewer_family; disposition
  # has no dedicated v1.1 field, so it stays evidence_ref-derived (present on
  # both v1 and v1.1 edges).
  def bead_of:
    (.bead_id
     // (.evidence_ref // "" | capture("^pawl-verdict (?<b>.+) disposition=[A-Za-z]+$")? | .b)
     // (.from_id | tostring | split("@")[0]));
  def disp_of:
    ((.evidence_ref // "" | capture("disposition=(?<d>[A-Za-z]+)")? | .d) // "UNRECORDED");

  . as $all
  | [ to_entries[]
      | select(.value.from_type == "verdict")
      | { i: .key,
          sha: (.value.to_id | tostring),
          short: (.value.to_id | tostring | .[0:7]),
          ts: .value.ts,
          bead: (.value | bead_of),
          disp: (.value | disp_of),
          evidence: .value.evidence_ref } ] as $V
  | [ $V[] | select(.disp == "REFUTED") | . as $r
      | ([ $V[] | select(.bead == $r.bead and .i > $r.i and .disp == "CONFIRMED") ] | first) as $f
      | { bead: $r.bead,
          refuted_commit: $r.sha,
          refuted_commit_short: $r.short,
          refuted_ts: $r.ts,
          evidence: $r.evidence,
          fixed_by: (if $f == null then null
                     else { commit: $f.sha, commit_short: $f.short, ts: $f.ts } end) } ] as $refuted
  | [ $all[] | select(
        (((.relation // "") + " " + (.evidence_ref // "") + " " + (.from_type // ""))
         | test("escape"; "i")) ) ] as $esc
  | [ $all[] | (.reviewer_family // .reviewer // .family // empty) ] as $fams
  | ([ $V[].disp ] | group_by(.) | map({ key: .[0], value: length }) | from_entries) as $disp
  | { schema: "agentops-membrane-receipts.v1",
      generated_at: $generated_at,
      source: {
        ledger: "docs/provenance/ledger.jsonl",
        records: ($all | length),
        tip_hash: $tip_hash,
        chain_verified: true,
        verifier: "ao provenance verify"
      },
      totals: {
        ledger_records: ($all | length),
        verdict_events: ($V | length),
        bead_commit_edges: ([ $all[] | select(.relation == "wasGeneratedBy") ] | length),
        distinct_beads_reviewed: ([ $V[].bead ] | unique | length),
        distinct_commits_reviewed: ([ $V[].sha ] | unique | length)
      },
      dispositions: {
        CONFIRMED: ($disp.CONFIRMED // 0),
        REFUTED: ($disp.REFUTED // 0),
        other: ($disp | to_entries
                      | map(select(.key != "CONFIRMED" and .key != "REFUTED"))
                      | sort_by(.key) | from_entries)
      },
      caught_defects: {
        refuted_then_fixed: ([ $refuted[] | select(.fixed_by != null) ] | length),
        refuted_without_recorded_fix: ([ $refuted[] | select(.fixed_by == null) ] | length),
        arcs: $refuted
      },
      escapes: {
        ledger_escape_records: ($esc | length),
        entries: [ $esc[] | { from_id, to_id, relation, evidence_ref, ts } ],
        note: "count of escape-tagged records in this ledger; absence of records is evidence, not proof, of absence"
      },
      reviewer_families: {
        recorded_in_ledger: (($fams | length) > 0),
        mix: (if ($fams | length) > 0
              then ($fams | group_by(.) | map({ key: .[0], value: length }) | from_entries)
              else {} end)
      },
      time_range: {
        first: ([ $all[].ts ] | sort | first),
        last: ([ $all[].ts ] | sort | last)
      },
      exemplar_catches: ($refuted | .[0:10])
    }' "$LEDGER")" || die "jq derivation failed"

# ---- Verify exemplar SHAs against the repo's CURRENT history (honesty bound:
# a ledger record can reference a commit rewritten out of history; never claim
# git-show reproducibility for a sha that no longer resolves). Derived, not assumed.
LEDGER_GIT_DIR="$(dirname "$LEDGER")"
UNRESOLVABLE="$(printf '%s\n' "$STATS_JSON" \
  | jq -r '[ .exemplar_catches[] | .refuted_commit_short, (.fixed_by.commit_short // empty) ] | unique | .[]' \
  | while IFS= read -r sha; do
      [ -n "$sha" ] || continue
      if ! git -C "$LEDGER_GIT_DIR" rev-parse --verify --quiet "${sha}^{commit}" >/dev/null 2>&1; then
        printf '%s\n' "$sha"
      fi
    done | jq -R . | jq -s .)" || die "sha verification failed"
STATS_JSON="$(printf '%s\n' "$STATS_JSON" | jq --argjson miss "$UNRESOLVABLE" '
  .unresolvable_shas = $miss
  | .exemplar_catches = [ .exemplar_catches[]
      | .refuted_commit_in_history = (([.refuted_commit_short] - $miss) | length > 0)
      | (if .fixed_by != null
         then .fixed_by.commit_in_history = (([.fixed_by.commit_short] - $miss) | length > 0)
         else . end) ]')" || die "sha annotation failed"

# ---- Render the human page from the machine twin (numbers stay in sync) ---
MD_BODY="$(printf '%s\n' "$STATS_JSON" | jq -r '
  def pl(n; s): "\(n) \(s)" + (if n == 1 then "" else "s" end);
  [ "# Membrane receipts",
    "",
    "> Auto-generated by `scripts/gen-membrane-receipts.sh` from",
    "> [`docs/provenance/ledger.jsonl`](../provenance/ledger.jsonl). Every number below is derived",
    "> from the ledger — nothing is hand-written. The generator refuses to render",
    "> if `ao provenance verify` reports a broken or tampered hash chain.",
    "> Machine twin: [`docs/releases/membrane-receipts.json`](../releases/membrane-receipts.json).",
    "",
    "Generated: \(.generated_at)",
    "",
    "Ledger tip hash: `\(.source.tip_hash)` · chain verified: \(.source.chain_verified) (`\(.source.verifier)`)",
    "",
    "## The numbers",
    "",
    "| Metric | Value |",
    "|---|---|",
    "| Ledger records | \(.totals.ledger_records) |",
    "| Verdict events (verdict → commit) | \(.totals.verdict_events) |",
    "| Bead → commit edges | \(.totals.bead_commit_edges) |",
    "| Distinct beads reviewed | \(.totals.distinct_beads_reviewed) |",
    "| Distinct commits reviewed | \(.totals.distinct_commits_reviewed) |",
    "| CONFIRMED verdicts | \(.dispositions.CONFIRMED) |",
    "| REFUTED verdicts | \(.dispositions.REFUTED) |"
  ]
  + [ .dispositions.other | to_entries[] | "| \(.key) verdicts | \(.value) |" ]
  + [ "| REFUTED-then-fixed arcs (caught defects) | \(.caught_defects.refuted_then_fixed) |",
      "| REFUTED without a recorded fix | \(.caught_defects.refuted_without_recorded_fix) |",
      "| Escape-tagged ledger records | \(.escapes.ledger_escape_records) |",
      "| Time range | \(.time_range.first) → \(.time_range.last) |",
      "",
      "A REFUTED-then-fixed arc is a defect the membrane caught: an independent",
      "reviewer refuted the change on one commit, and a later verdict on a",
      "subsequent commit for the same bead came back CONFIRMED.",
      "",
      "## Escapes",
      "",
      (if .escapes.ledger_escape_records == 0
       then "No escape-tagged records exist in this ledger. Absence of records is evidence, not proof, of absence."
       else pl(.escapes.ledger_escape_records; "escape-tagged record") +
            " — surfaced, not hidden:" end)
    ]
  + (if .escapes.ledger_escape_records > 0
     then [ .escapes.entries[] | "- `\(.from_id)` → `\(.to_id)` (\(.relation)) at \(.ts): \(.evidence_ref)" ]
     else [] end)
  + [ "",
      "## Reviewer family mix",
      "",
      (if .reviewer_families.recorded_in_ledger
       then ([ .reviewer_families.mix | to_entries[] | "- \(.key): \(.value)" ] | join("\n"))
       else "Not recorded in this ledger schema (v1 records carry no reviewer-family field)." end),
      "",
      "## Exemplar catches (from REFUTED ledger entries)",
      ""
    ]
  + (if (.exemplar_catches | length) == 0
     then [ "No REFUTED entries in this ledger." ]
     else [ .exemplar_catches[]
            | "- **\(.bead)** — REFUTED on `\(.refuted_commit_short)`"
              + (if .refuted_commit_in_history then "" else " (commit no longer in current history — predates a rewrite)" end)
              + " at \(.refuted_ts) (`\(.evidence)`)"
              + (if .fixed_by != null
                 then " → fixed: CONFIRMED on `\(.fixed_by.commit_short)`"
                      + (if .fixed_by.commit_in_history then "" else " (not in current history)" end)
                      + " at \(.fixed_by.ts)"
                 else " → no CONFIRMED follow-up recorded in this ledger" end) ]
     end)
  + [ "",
      (if (.unresolvable_shas | length) == 0
       then "Every short SHA above resolves in this repo; `git show <sha>` reproduces the evidence."
       else "SHAs not marked otherwise resolve in this repo via `git show <sha>`; marked ones are honest ledger history whose commits were rewritten away." end),
      "Re-running the generator against the same ledger reproduces this page byte-for-byte",
      "(modulo the Generated line)." ]
  | join("\n")
')" || die "markdown rendering failed"

mkdir -p "$(dirname "$OUT_MD")" "$(dirname "$OUT_JSON")" || die "cannot create output directories"
TMP_MD="$(mktemp)" || die "mktemp failed for md"
TMP_JSON="$(mktemp)" || die "mktemp failed for json"
printf '%s\n' "$MD_BODY" > "$TMP_MD" || die "writing md temp failed"
printf '%s\n' "$STATS_JSON" > "$TMP_JSON" || die "writing json temp failed"
mv "$TMP_MD" "$OUT_MD" || die "final move to $OUT_MD failed"
mv "$TMP_JSON" "$OUT_JSON" || die "final move to $OUT_JSON failed"

echo "gen-membrane-receipts: OK — rendered from $LEDGER_COUNT verified ledger record(s)"
echo "  md:   $OUT_MD"
echo "  json: $OUT_JSON"
