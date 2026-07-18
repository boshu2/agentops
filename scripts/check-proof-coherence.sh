#!/usr/bin/env bash
# check-proof-coherence.sh — proof-coherence gate (reconciliation engine, Wave 3C; soc-57hbj).
#
# The reconciliation engine has two axes (plan: .agents/plans/2026-05-07-reconciliation-engine-full-arc.md):
#   - claim coherence  (surface-to-surface wording alignment) — advisory.
#   - proof coherence  (claim-ledger row -> daemon criterion-verdict event) — BLOCKING for L2/L3 claims.
#
# This is the proof-coherence axis. A claim-ledger row promoted to release-posture
# L2 or L3 asserts "this claim is backed by a passing factory event." That assertion
# is only honest if the referenced event actually exists in the daemon event stream
# AND carries verdict: pass. This gate closes the gap between "the ledger says it's
# proven" and "the event stream proves it" — it is the difference between a claim and
# a coherent claim.
#
# Coherence predicate (applied to every L2/L3 ledger row; L0/L1 rows are not gated):
#   For each entry in the row's evidence_event_refs[]:
#     1. an event with matching event_type AND run_id (AND criterion_id, if the ref
#        names one) MUST exist in the daemon ledger  -> else: DANGLING (incoherent).
#     2. that event's verdict MUST be "pass"                -> else: FAILED  (incoherent, demote).
#   A row with zero evidence_event_refs at L2/L3 is itself incoherent (UNBACKED).
#
# Inputs (auto-discovered; all optional so the gate is CI-safe before the ledger ships):
#   --ledger <file>   Claim ledger JSON. Default: first existing of
#                       docs/contracts/factory-claim-ledger.json
#                       docs/contracts/factory-claim-ledger.example.json
#   --events <file>   Daemon event-stream JSONL. Default: first existing of
#                       cli/.agents/daemon/ledger.jsonl
#                       .agents/daemon/ledger.jsonl
#   --json            Emit a machine-readable JSON report on stdout instead of text.
#
# When no claim ledger exists, the gate is a PASS no-op (nothing to prove yet) — this
# lets it land in CI as blocking before Wave 1's ledger lands, then become load-bearing
# automatically once L2/L3 rows with evidence_event_refs appear.
#
# Claim-ledger row shape (the fields this gate reads):
#   {
#     "rows": [
#       { "claim_id": "C-worker-context",
#         "promotion_state": "L2",                          # L0|L1|L2|L3
#         "evidence_event_refs": [
#           { "event_type": "agent_update.criterion_verdict",
#             "run_id": "rpi-2026-05-12T...",
#             "criterion_id": "context-on-passes" }         # criterion_id optional
#         ] } ] }
#
# Daemon event shape (the fields this gate reads; extra fields ignored):
#   { "event_type": "agent_update.criterion_verdict",
#     "run_id": "rpi-2026-05-12T...",                       # or request_id as fallback
#     "payload": { "criterion_id": "context-on-passes", "verdict": "pass" } }
#   (verdict / criterion_id are also accepted at top level.)
#
# Exit codes:
#   0 = coherent (every L2/L3 row's refs resolve to a passing event), or no ledger to check
#   1 = incoherent (a dangling ref, a failed-verdict ref, or an unbacked L2/L3 row)
#   2 = usage / environment error (bad flag, missing jq, unreadable input)
set -euo pipefail

PROG="check-proof-coherence"

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"

ledger=""
events=""
json_mode=false

usage() {
	cat >&2 <<USAGE
Usage: scripts/${PROG}.sh [--ledger <file>] [--events <file>] [--json]

Proof-coherence gate: every L2/L3 claim-ledger row must reference daemon events
that exist and carry verdict: pass. Dangling refs, failed verdicts, and unbacked
L2/L3 rows are incoherent.

Options:
  --ledger <file>   Claim ledger JSON (default: auto-discover under docs/contracts/).
  --events <file>   Daemon event-stream JSONL (default: auto-discover under .agents/daemon/).
  --json            Emit a JSON report instead of human-readable text.
  -h, --help        Show this help.

Exit: 0 coherent / no ledger · 1 incoherent · 2 usage or environment error
USAGE
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--ledger)
			[ "$#" -ge 2 ] || { echo "${PROG}: --ledger needs a value" >&2; usage; exit 2; }
			ledger="$2"; shift 2 ;;
		--events)
			[ "$#" -ge 2 ] || { echo "${PROG}: --events needs a value" >&2; usage; exit 2; }
			events="$2"; shift 2 ;;
		--json)
			json_mode=true; shift ;;
		-h|--help)
			usage; exit 0 ;;
		*)
			echo "${PROG}: unknown argument: $1" >&2; usage; exit 2 ;;
	esac
done

if ! command -v jq >/dev/null 2>&1; then
	echo "${PROG}: jq is required but not found on PATH" >&2
	exit 2
fi

# --- discover inputs ---------------------------------------------------------
if [ -z "$ledger" ]; then
	for cand in \
		"$repo_root/docs/contracts/factory-claim-ledger.json" \
		"$repo_root/docs/contracts/factory-claim-ledger.example.json"; do
		if [ -f "$cand" ]; then ledger="$cand"; break; fi
	done
fi

# No ledger anywhere -> nothing to prove yet. PASS no-op (forward-compatible gate).
if [ -z "$ledger" ]; then
	if $json_mode; then
		printf '{"gate":"proof-coherence","status":"pass","reason":"no-claim-ledger","rows_checked":0,"incoherent":[]}\n'
	else
		echo "ok: ${PROG}: no claim ledger present — nothing to prove (gate is a no-op until the ledger ships)"
	fi
	exit 0
fi

if [ ! -r "$ledger" ]; then
	echo "${PROG}: claim ledger not readable: $ledger" >&2
	exit 2
fi
if ! jq -e . "$ledger" >/dev/null 2>&1; then
	echo "${PROG}: claim ledger is not valid JSON: $ledger" >&2
	exit 2
fi

if [ -z "$events" ]; then
	for cand in \
		"$repo_root/cli/.agents/daemon/ledger.jsonl" \
		"$repo_root/.agents/daemon/ledger.jsonl"; do
		if [ -f "$cand" ]; then events="$cand"; break; fi
	done
fi
# An empty/absent event stream is valid input: it just means every ref is dangling.
if [ -n "$events" ] && [ ! -r "$events" ]; then
	echo "${PROG}: daemon event stream not readable: $events" >&2
	exit 2
fi

# --- build a lookup of daemon criterion-verdict events -----------------------
# Key = "<event_type>\t<run_id>\t<criterion_id>" ; value = verdict.
# run_id falls back to request_id; criterion_id/verdict accepted top-level or in payload.
events_index="$(mktemp -t "${PROG}.events.XXXXXX")"
trap 'rm -f "$events_index"' EXIT

if [ -n "$events" ] && [ -s "$events" ]; then
	# Parse each line independently; tolerate blank/garbage lines via fromjson?.
	jq -rR '
		fromjson? // empty
		| {
			et: (.event_type // ""),
			rid: (.run_id // .request_id // ""),
			cid: (.criterion_id // .payload.criterion_id // ""),
			v:  (.verdict // .payload.verdict // "")
		}
		| select(.et != "")
		| [.et, .rid, .cid, .v] | @tsv
	' "$events" > "$events_index" 2>/dev/null || true
fi

# Look up the verdict for a ref. Echoes one of:
#   pass | fail | <other-verdict-string>   (an event matched)
#   __MISSING__                            (no event matched -> dangling ref)
lookup_verdict() {
	local et="$1" rid="$2" cid="$3"
	# Exact match including criterion_id when the ref names one; otherwise match
	# on (event_type, run_id) and take the first verdict for that pair.
	awk -F'\t' -v et="$et" -v rid="$rid" -v cid="$cid" '
		$1 == et && $2 == rid {
			if (cid == "" || $3 == cid) { print $4; found=1; exit }
		}
		END { if (!found) print "__MISSING__" }
	' "$events_index"
}

# --- iterate L2/L3 rows ------------------------------------------------------
# Emit one TSV record per L2/L3 row: claim_id, promotion_state, refs-as-json.
rows_tsv="$(jq -r '
	(.rows // [])
	| map(select(((.promotion_state // .promotion // .level // "") | ascii_upcase) as $p
		| ($p == "L2" or $p == "L3")))
	| .[]
	| [ (.claim_id // .id // "<unnamed>"),
	    ((.promotion_state // .promotion // .level // "") | ascii_upcase),
	    ((.evidence_event_refs // []) | tojson) ]
	| @tsv
' "$ledger")"

rows_checked=0
incoherent_count=0
# Collected incoherence records for the JSON report.
incoherent_json="[]"

add_incoherent() {
	local claim="$1" state="$2" kind="$3" detail="$4"
	incoherent_count=$((incoherent_count + 1))
	incoherent_json="$(jq -c \
		--arg claim "$claim" --arg state "$state" --arg kind "$kind" --arg detail "$detail" \
		'. + [{claim_id:$claim, promotion_state:$state, kind:$kind, detail:$detail}]' \
		<<<"$incoherent_json")"
}

if [ -n "$rows_tsv" ]; then
	while IFS=$'\t' read -r claim_id state refs_json; do
		[ -n "$claim_id" ] || continue
		rows_checked=$((rows_checked + 1))

		ref_count="$(jq 'length' <<<"$refs_json")"
		if [ "$ref_count" -eq 0 ]; then
			add_incoherent "$claim_id" "$state" "unbacked" \
				"row promoted to ${state} but has no evidence_event_refs"
			$json_mode || echo "INCOHERENT: ${claim_id} (${state}): UNBACKED — no evidence_event_refs" >&2
			continue
		fi

		# Walk each ref.
		while IFS=$'\t' read -r et rid cid; do
			[ -n "${et}${rid}${cid}" ] || continue
			verdict="$(lookup_verdict "$et" "$rid" "$cid")"
			refdesc="event_type=${et} run_id=${rid}${cid:+ criterion_id=${cid}}"
			case "$verdict" in
				pass)
					$json_mode || echo "ok: ${claim_id} (${state}): ${refdesc} -> pass" ;;
				__MISSING__)
					add_incoherent "$claim_id" "$state" "dangling" \
						"ref has no matching daemon event: ${refdesc}"
					$json_mode || echo "INCOHERENT: ${claim_id} (${state}): DANGLING — no daemon event for ${refdesc}" >&2 ;;
				"")
					add_incoherent "$claim_id" "$state" "no-verdict" \
						"matched daemon event carries no verdict: ${refdesc}"
					$json_mode || echo "INCOHERENT: ${claim_id} (${state}): NO-VERDICT — event present but no verdict for ${refdesc}" >&2 ;;
				*)
					add_incoherent "$claim_id" "$state" "failed" \
						"referenced event verdict is '${verdict}', not pass: ${refdesc}"
					$json_mode || echo "INCOHERENT: ${claim_id} (${state}): FAILED — verdict='${verdict}' (not pass) for ${refdesc}" >&2 ;;
			esac
		done < <(jq -r '.[] | [ (.event_type // ""), (.run_id // ""), (.criterion_id // "") ] | @tsv' <<<"$refs_json")
	done <<<"$rows_tsv"
fi

# --- verdict -----------------------------------------------------------------
status=0
[ "$incoherent_count" -gt 0 ] && status=1

if $json_mode; then
	jq -nc \
		--argjson rows "$rows_checked" \
		--argjson n "$incoherent_count" \
		--argjson inc "$incoherent_json" \
		--arg ledger "$ledger" \
		--arg events "${events:-}" \
		'{
			gate: "proof-coherence",
			status: (if $n == 0 then "pass" else "fail" end),
			ledger: $ledger,
			events: $events,
			rows_checked: $rows,
			incoherent_count: $n,
			incoherent: $inc
		}'
else
	if [ "$incoherent_count" -eq 0 ]; then
		echo "ok: ${PROG}: ${rows_checked} L2/L3 row(s) coherent (all refs resolve to passing daemon events)"
	else
		echo "${PROG}: ${incoherent_count} incoherence(s) across ${rows_checked} L2/L3 row(s) — proof coherence FAILED" >&2
	fi
fi

exit "$status"
