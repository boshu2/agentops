#!/usr/bin/env bash
# check-tip-verdict-ci.sh — CI verdict backstop (age-wedge-all-in-dyr0.9).
#
# The local pre-push cockpit gate (scripts/check-pawl-pre-push.sh) is the
# release AUTHORITY; this is the remote BACKSTOP: given a pushed ref range
# BASE..TIP it answers "does this push carry its proof?" by verifying RECORDS
# only — it never produces verdicts (no reviewer calls, no secrets, no
# subscription auth; posture per docs: no hosted control plane).
#
# Checks:
#   (a) the committed provenance ledger (docs/provenance/ledger.jsonl) is an
#       intact, tamper-evident hash chain (`ao provenance verify`, same surface
#       validate.yml uses via scripts/validate-provenance-ledger.sh).
#       TAMPER TRUMPS REPORT-ONLY: a broken chain exits nonzero in BOTH modes.
#   (b) every commit in BASE..TIP has a bound verdict edge in the ledger
#       (from_type=verdict, to_type=commit, disposition=CONFIRMED — the shape
#       `ao provenance emit-verdict` writes) OR satisfies the #trivial
#       provenance-only waiver. The waiver is the SAME implementation the
#       pre-push gate runs: scripts/lib/trivial-waiver.sh (single source —
#       never re-inline it here; see check-pawl-pre-push.sh).
#
# Modes:
#   default    report-only — always exit 0 on missing verdicts; emit GitHub
#              annotations (::warning:: per unverified sha, ::notice:: per
#              verified/waived one). Chain tamper still exits nonzero.
#   --enforce  exit nonzero when any commit in the range lacks proof.
#
# Usage:
#   scripts/check-tip-verdict-ci.sh [--base <sha>] [--head <ref>] [--repo <dir>] [--enforce]
#     --base   range base (exclusive). Empty, all-zeros (new branch/force
#              push), or unknown-locally → falls back to checking the tip
#              commit only.
#     --head   range tip (default: HEAD).
#     --repo   repository to check (default: git toplevel of cwd).
#   Env: AO_BIN — path to the ao binary (else <repo>/cli/bin/ao, else PATH).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die() { echo "tip-verdict-ci: ERROR: $*" >&2; exit 2; }

# SINGLE-IMPLEMENTATION (age-wedge-all-in-dyr0.9): the #trivial waiver is
# sourced from the SAME file the pre-push gate sources.
# shellcheck source=lib/trivial-waiver.sh
if ! source "$SCRIPT_DIR/lib/trivial-waiver.sh"; then
  die "shared waiver lib missing/unreadable at $SCRIPT_DIR/lib/trivial-waiver.sh"
fi
declare -F pawl_trivial_waiver >/dev/null || die "pawl_trivial_waiver not defined by $SCRIPT_DIR/lib/trivial-waiver.sh"

# REBOUND authorization (age-rk3r.18): the CI backstop honors a committed REBOUND
# verdict edge with the SAME lineage+proof discipline as the pre-push gate — but
# via SHELL + the shared diff-identity library, because CI runs INSIDE THE TRUSTED
# REPO (its own checkout, cloned from the branch under test), NOT a hostile stranger
# repo. That trust context is the difference: the portable Go gate
# (cli/cmd/ao/verify_prepush.go) is the hostile-repo-safe path and must re-derive
# everything in Go from a sanitized-PATH git (it may not run any repo-tree script);
# CI legitimately sources scripts/lib/diff-identity.sh + uses `git` on PATH. The
# equivalence PROPERTY proven is identical: a REBOUND commit is authorized iff a
# committed CONFIRMED-reviewed commit exists whose diff is BYTE-EQUIVALENT to it
# (same git patch-id --stable AND byte-exact content signature). The edge's stored
# patch_id_proof is never trusted — the keys are re-derived from git.
# shellcheck source=lib/diff-identity.sh
if ! source "$SCRIPT_DIR/lib/diff-identity.sh"; then
  die "shared diff-identity lib missing/unreadable at $SCRIPT_DIR/lib/diff-identity.sh"
fi
declare -F commit_patch_id >/dev/null || die "commit_patch_id not defined by $SCRIPT_DIR/lib/diff-identity.sh"
declare -F commit_content_sig >/dev/null || die "commit_content_sig not defined by $SCRIPT_DIR/lib/diff-identity.sh"

BASE=""
HEAD_REF="HEAD"
REPO=""
ENFORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base)    BASE="${2:-}"; shift 2 ;;
    --head)    HEAD_REF="${2:-}"; shift 2 ;;
    --repo)    REPO="${2:-}"; shift 2 ;;
    --enforce) ENFORCE=1; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

if [[ -z "$REPO" ]]; then
  REPO="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not inside a git repository and no --repo given"
fi
[[ -d "$REPO/.git" || -f "$REPO/.git" ]] || die "--repo $REPO is not a git repository"

TIP="$(git -C "$REPO" rev-parse --verify "${HEAD_REF}^{commit}" 2>/dev/null)" || die "cannot resolve --head '$HEAD_REF' in $REPO"

LEDGER="$REPO/docs/provenance/ledger.jsonl"

# ── (a) Ledger hash-chain verification — tamper trumps report-only ─────────
resolve_ao() {
  if [[ -n "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
  if [[ -x "$REPO/cli/bin/ao" ]]; then printf '%s\n' "$REPO/cli/bin/ao"; return 0; fi
  if command -v ao >/dev/null 2>&1; then printf 'ao\n'; return 0; fi
  return 1
}

if AO="$(resolve_ao)"; then
  # `ao provenance verify` resolves the ledger by walking up from cwd, so run
  # it FROM the repo under check (mirrors validate-provenance-ledger.sh --gate).
  if ( cd "$REPO" && "$AO" provenance verify ); then
    echo "tip-verdict-ci: provenance ledger hash chain intact" >&2
  else
    echo "::error::provenance ledger hash chain BROKEN/TAMPERED in $LEDGER — failing regardless of mode (tamper trumps report-only)"
    exit 1
  fi
else
  # No binary means the chain CANNOT be verified. Report-only degrades to a
  # loud warning; enforce mode is fail-closed (an unverifiable chain must not
  # pass an enforcing gate).
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::ao binary unavailable — cannot verify the provenance ledger hash chain (fail-closed under --enforce)"
    exit 1
  fi
  echo "::warning::ao binary unavailable — provenance ledger hash-chain verification SKIPPED (report-only)"
fi

command -v jq >/dev/null 2>&1 || {
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::jq unavailable — cannot resolve verdict edges (fail-closed under --enforce)"
    exit 1
  fi
  echo "::warning::jq unavailable — verdict-edge resolution SKIPPED (report-only)"
  exit 0
}

# ── build the commit list ───────────────────────────────────────────────────
ZEROS="0000000000000000000000000000000000000000"
declare -a COMMITS=()
if [[ -z "$BASE" || "$BASE" == "$ZEROS" ]] \
   || ! git -C "$REPO" rev-parse --verify --quiet "${BASE}^{commit}" >/dev/null 2>&1; then
  # New branch, force push, or shallow history without the base: fall back to
  # the tip commit — the same single-commit surface the pre-push gate checks.
  COMMITS=("$TIP")
  echo "tip-verdict-ci: base unavailable — checking tip commit only (${TIP:0:12})" >&2
else
  while IFS= read -r _sha; do
    [[ -n "$_sha" ]] && COMMITS+=("$_sha")
  done < <(git -C "$REPO" rev-list --reverse "${BASE}..${TIP}")
fi

if [[ "${#COMMITS[@]}" -eq 0 ]]; then
  echo "::notice::tip-verdict-ci: no commits in range ${BASE:0:12}..${TIP:0:12} — nothing to verify"
  exit 0
fi

# ═══════════════════════════════════════════════════════════════════════════════
# CI selectors are FIELD-FOR-FIELD parity with the Go gate predicates (named
# below); any near-miss Go rejects, CI must reject. (age-rk3r.18 comprehensive
# parity sweep — the class-wide fix after four one-per-round divergences: ref-alias,
# relation, substring, disposition-token.) The shared jq prelude JQ_VERDICT_LIB
# encodes the two Go semantics an ad-hoc selector kept getting subtly wrong:
#
#   dispvalue(ref)  ≡ Go parseDisposition (cli/cmd/ao/provenance_show.go): the
#     value of the FIRST whitespace-delimited `disposition=` token; a SECOND token
#     is IGNORED, and a non-`disposition=` field is not a match. So
#     "disposition=REFUTED disposition=REBOUND" yields REFUTED (Go rejects a REBOUND
#     read of it), and "disposition=CONFIRMEDLY" yields CONFIRMEDLY (!= CONFIRMED).
#     splits("[[:space:]]+") + drop-empties mirrors strings.Fields.
#
#   binds($q;$c)    ≡ Go shaBindsCommit (cli/cmd/ao/done.go): lowercase BOTH; BOTH
#     must be >=7 chars AND pure hex; then one is a prefix of the other (EITHER
#     direction). A non-hex / too-short / non-binding to_id is rejected — the same
#     discipline the direct CONFIRMED path always applied, now applied uniformly.
#
# Per-field audit vs the Go predicates (confirmedVerdictEdgeIn / reboundEdgeBoundTo /
# confirmedVerdictCommitSHAs / parseDisposition / shaBindsCommit):
#   relation == "wasDerivedFrom"    — EXACT (match)
#   from_type == "verdict"          — EXACT (match)
#   to_type   == "commit"           — EXACT (match)
#   disposition                     — FIRST-token exact via dispvalue (fixed here)
#   to_id binding                   — hex + >=7 + prefix-either-way via binds (fixed here)
# The lineage to_id ALSO passes through hex_commit_object_id (object-id resolution,
# not a raw revision) before it is handed to git — see rebound_authorizes.
JQ_VERDICT_LIB='
  def dispvalue($ref):
    ([$ref // "" | splits("[[:space:]]+")]
      | map(select(. != "" and startswith("disposition=")))
      | (first // "")
      | ltrimstr("disposition="));
  def isverdictedge:
    (.relation == "wasDerivedFrom" and .from_type == "verdict" and .to_type == "commit");
  def binds($q; $c):
    ($q | ascii_downcase) as $lq
    | ($c | ascii_downcase) as $lc
    | ($lq | test("^[0-9a-f]+$")) and ($lc | test("^[0-9a-f]+$"))
      and ($lq | length) >= 7 and ($lc | length) >= 7
      and (($lq | startswith($lc)) or ($lc | startswith($lq)));
'

# verdict_event_for <sha> — print "from_id<TAB>evidence_ref" of the first
# CONFIRMED verdict edge bound to the commit, if any. Edge shape is exactly what
# `ao provenance emit-verdict` writes (verdict --wasDerivedFrom--> commit,
# evidence_ref "pawl-verdict <bead> disposition=<D>"). Go twin: confirmedVerdictEdgeIn.
verdict_event_for() {
  local sha="$1"
  [[ -f "$LEDGER" ]] || return 1
  local hit
  hit="$(jq -r --arg sha "$sha" "$JQ_VERDICT_LIB"'
    select(isverdictedge)
    | select(binds($sha; .to_id))
    | select(dispvalue(.evidence_ref) == "CONFIRMED")
    | [.from_id, .evidence_ref] | @tsv' "$LEDGER" 2>/dev/null | head -1)"
  [[ -n "$hit" ]] || return 1
  printf '%s\n' "$hit"
}

# rebound_edge_bound_to <sha> — exit 0 iff the ledger carries a REBOUND verdict
# edge bound to the commit (relation=wasDerivedFrom, from_type=verdict,
# to_type=commit, FIRST disposition token == REBOUND, to_id binds sha).
# Go twin: reboundEdgeBoundTo. Exact + fail-closed.
rebound_edge_bound_to() {
  local sha="$1"
  [[ -f "$LEDGER" ]] || return 1
  local hit
  hit="$(jq -r --arg sha "$sha" "$JQ_VERDICT_LIB"'
    select(isverdictedge)
    | select(binds($sha; .to_id))
    | select(dispvalue(.evidence_ref) == "REBOUND")
    | .from_id' "$LEDGER" 2>/dev/null | head -1)"
  [[ -n "$hit" ]] || return 1
  return 0
}

# confirmed_lineage_shas — print (one per line) the DISTINCT to_id commit shas of
# every CONFIRMED verdict edge in the ledger: the candidate REBOUND lineage roots.
# Same exact relation + from/to_type + FIRST-disposition-token discipline as above.
# Go twin: confirmedVerdictCommitSHAs.
confirmed_lineage_shas() {
  [[ -f "$LEDGER" ]] || return 0
  jq -r "$JQ_VERDICT_LIB"'
    select(isverdictedge)
    | select(dispvalue(.evidence_ref) == "CONFIRMED")
    | .to_id' "$LEDGER" 2>/dev/null | awk 'NF && !seen[$0]++'
}

# hex_commit_object_id <candidate> — print the FULL resolved commit oid of
# <candidate> IFF it is a HEX commit id resolving to a committish OBJECT, else
# print nothing (reject). This is the SHELL twin of the Go hexCommitObjectID and
# the fix for the age-rk3r.18 refuter fail-open: the REBOUND lineage to_id is fed
# to `git show`/patch-id, so a ledger-supplied REVISION EXPRESSION ("HEAD~1", a
# branch/tag name, ":/msg") must NEVER be treated as the reviewed commit — that
# would let a crafted CONFIRMED edge point its lineage at an arbitrary revision
# whose diff matches the tip and certify an unreviewed commit. Discipline (each
# step fail-closed): (1) candidate must match a HEX regex of >=7 chars (rejects
# every revision expression, which all carry non-hex bytes ~ ^ : / letters); (2)
# resolve with `git rev-parse --verify --quiet <hex>^{commit}` (non-committish →
# reject); (3) DEFENSE-IN-DEPTH — the resolved full oid must have <candidate> as a
# prefix (a ref NAMED like a hex prefix whose tip oid does not start with it is
# rejected even if git resolved the name).
hex_commit_object_id() {
  local candidate="$1"
  # (1) HEX + length gate (mirrors isHexToken + minShaPrefixLen).
  [[ "$candidate" =~ ^[0-9a-fA-F]{7,}$ ]] || return 1
  # (2) resolve as an OBJECT id (^{commit} peel; quiet so a miss is silent).
  local oid
  oid="$(git -C "$REPO" rev-parse --verify --quiet "${candidate}^{commit}" 2>/dev/null)" || return 1
  [[ -n "$oid" ]] || return 1
  # (3) the resolved oid must BIND the candidate hex (prefix either way).
  local lc_oid lc_cand
  lc_oid="$(printf '%s' "$oid" | tr '[:upper:]' '[:lower:]')"
  lc_cand="$(printf '%s' "$candidate" | tr '[:upper:]' '[:lower:]')"
  if [[ "$lc_oid" == "$lc_cand" || "$lc_oid" == "$lc_cand"* || "$lc_cand" == "$lc_oid"* ]]; then
    printf '%s' "$oid"
    return 0
  fi
  return 1
}

# rebound_authorizes <sha> — DISTINGUISHES THREE OUTCOMES via exit code (the
# honest-scoping fix, age-rk3r.18): a REBOUND edge is authorized ONLY when a
# committed CONFIRMED-reviewed commit R (R != sha) exists whose diff is
# BYTE-EQUIVALENT to <sha> — same git patch-id --stable AND byte-exact content
# signature, both re-derived via the shared diff-identity library.
#
#   exit 0  — AUTHORIZED: a reachable CONFIRMED lineage R is byte-equivalent to C.
#   exit 2  — UNVERIFIABLE-HERE (fail-closed, NOT authorized): a well-formed REBOUND
#             edge is bound to C (wasDerivedFrom + hex to_id) and C's own diff is
#             derivable, but NO byte-equivalence match was found AND at least one
#             CONFIRMED lineage candidate's reviewed commit R could NOT be resolved/
#             re-derived in THIS checkout (orphaned/unreachable — the classic
#             clean-CI-clone-after-rebase case). The caller emits a DISTINCT,
#             accurate message and still REFUSES (never skip-authorizes — a forged
#             REBOUND pointing at a nonexistent R also lands here and is still
#             refused, so this is safe).
#   exit 1  — NO PROOF: no well-formed REBOUND edge, or C's own diff is not
#             derivable, or every CONFIRMED lineage candidate WAS reachable but
#             none is byte-equivalent (a genuine no-verdict / forge with a reachable
#             non-matching R).
#
# The lineage to_id is validated as a HEX commit OBJECT id (hex_commit_object_id)
# BEFORE it is ever handed to git — a revision expression like "HEAD~1" is rejected,
# closing the ref-alias fail-open. The edge's stored patch_id_proof is NEVER
# consulted; the authoritative comparison recomputes R's keys from git. Fail-closed
# on every path that is not exit 0.
rebound_authorizes() {
  local sha="$1"
  rebound_edge_bound_to "$sha" || return 1   # no well-formed REBOUND edge → plain no-proof
  local tip_pid tip_sig
  tip_pid="$(commit_patch_id "$sha" "$REPO")"
  tip_sig="$(commit_content_sig "$sha" "$REPO")"
  # Empty key = cannot prove even the TIP's identity → plain no-proof (exit 1).
  [[ -n "$tip_pid" && -n "$tip_sig" ]] || return 1
  local r r_oid r_pid r_sig
  local saw_unreachable_lineage=0   # a hex, well-formed lineage to_id we could NOT resolve/re-derive here
  while IFS= read -r r; do
    [[ -n "$r" ]] || continue
    # SECURITY (refuter fix): the lineage to_id MUST be a hex commit OBJECT id,
    # never a revision expression — resolve+validate before touching git for the
    # diff. A NON-HEX / ref-alias to_id is a malformed lineage (skip, not
    # "unreachable"). A HEX to_id that does NOT resolve to an object here is the
    # ORPHANED-REVIEWED-COMMIT case → note it for the distinct message.
    if [[ "$r" =~ ^[0-9a-fA-F]{7,}$ ]]; then
      r_oid="$(hex_commit_object_id "$r")" || { saw_unreachable_lineage=1; continue; }
    else
      r_oid="$(hex_commit_object_id "$r")" || continue   # non-hex → malformed, not "unreachable"
    fi
    [[ -n "$r_oid" ]] || { saw_unreachable_lineage=1; continue; }
    # A CONFIRMED bound to the tip itself is the verdict_event_for path, not
    # lineage — skip it so a REBOUND that names its own commit proves nothing.
    if [[ "$r_oid" == "$sha" || ( ${#r_oid} -ge 7 && "$sha" == "$r_oid"* ) || ( ${#sha} -ge 7 && "$r_oid" == "$sha"* ) ]]; then
      continue
    fi
    r_pid="$(commit_patch_id "$r_oid" "$REPO")"
    r_sig="$(commit_content_sig "$r_oid" "$REPO")"
    # A resolved-but-unre-derivable R (e.g. a shallow clone that has the commit
    # object but not its parent for the diff) is also "unverifiable here".
    [[ -n "$r_pid" && -n "$r_sig" ]] || { saw_unreachable_lineage=1; continue; }
    if [[ "$r_pid" == "$tip_pid" && "$r_sig" == "$tip_sig" ]]; then
      return 0
    fi
  done < <(confirmed_lineage_shas)
  # No byte-equivalence match. If the ONLY reason we could not match is that a
  # well-formed lineage candidate's reviewed commit is unreachable in this
  # checkout, report exit 2 (distinct message, still fail-closed). Otherwise every
  # candidate was reachable and non-matching → exit 1 (genuine no-proof).
  if [[ "$saw_unreachable_lineage" -eq 1 ]]; then
    return 2
  fi
  return 1
}

# ── (b) per-commit verdict-or-waiver check ──────────────────────────────────
violations=0
for sha in "${COMMITS[@]}"; do
  subject="$(git -C "$REPO" log -1 --format=%s "$sha" 2>/dev/null || true)"
  if event="$(verdict_event_for "$sha")"; then
    from_id="${event%%$'\t'*}"
    echo "::notice::commit ${sha:0:12} verified — bound verdict event ${from_id} (${subject})"
    continue
  fi
  # A committed REBOUND edge authorizes ONLY after lineage + byte-equivalence
  # RE-VALIDATION (age-rk3r.18) — a committed CONFIRMED-reviewed commit
  # byte-equivalent to this tip must exist. A bare disposition=REBOUND is NOT
  # accepted; the stored patch_id_proof is never trusted. rebound_authorizes
  # distinguishes AUTHORIZED (0) from UNVERIFIABLE-HERE (2: a well-formed REBOUND
  # whose reviewed commit is unreachable in this checkout — orphaned by a rebase)
  # from plain NO-PROOF (1).
  rebound_rc=0
  rebound_authorizes "$sha" || rebound_rc=$?
  if [[ "$rebound_rc" -eq 0 ]]; then
    echo "::notice::commit ${sha:0:12} verified — REBOUND authorized (byte-equivalent to a CONFIRMED-reviewed commit) (${subject})"
    continue
  fi
  if [[ "$rebound_rc" -eq 2 ]]; then
    # HONEST SCOPING (age-rk3r.18): the REBOUND edge is well-formed but the reviewed
    # commit it descends from is NOT reachable in this checkout, so CI cannot
    # INDEPENDENTLY re-verify the byte-equivalence. Fail-closed (do NOT authorize),
    # with a DISTINCT, accurate message — never the misleading "lacks a bound
    # verdict". The LOCAL pre-push gate is authoritative for REBOUND (the reviewed
    # commit is reachable at push time); CI can only honor it when the reviewed
    # commit is reachable in the CI checkout. age-rk3r.19 tracks making CI-REBOUND
    # work with an orphaned reviewed commit (a keep-ref design).
    echo "::warning::commit ${sha:0:12} REBOUND: the reviewed commit it descends from is NOT reachable in this checkout (likely orphaned by a rebase), so the byte-equivalence cannot be independently re-verified — NOT authorized (fail-closed). The local pre-push gate is authoritative for REBOUND; for CI to honor it, fetch the reviewed commit or the commit needs a fresh verdict (${subject})"
    violations=$((violations + 1))
    continue
  fi
  waiver_rc=0
  pawl_trivial_waiver "$REPO" "$sha" "tip-verdict-ci" || waiver_rc=$?
  if [[ "$waiver_rc" -eq 0 ]]; then
    echo "::notice::commit ${sha:0:12} waived — #trivial provenance-ledger-only commit (${subject})"
    continue
  fi
  echo "::warning::commit ${sha:0:12} lacks a bound verdict — no CONFIRMED verdict edge in docs/provenance/ledger.jsonl and no #trivial provenance-only waiver (${subject})"
  violations=$((violations + 1))
done

if [[ "$violations" -gt 0 ]]; then
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::tip-verdict-ci: $violations commit(s) in ${#COMMITS[@]}-commit range lack proof (no verdict = not done) — failing (--enforce)"
    exit 1
  fi
  echo "::warning::tip-verdict-ci: $violations commit(s) in ${#COMMITS[@]}-commit range lack proof — report-only, not failing (run with --enforce to gate)"
  exit 0
fi

echo "tip-verdict-ci: all ${#COMMITS[@]} commit(s) in range carry proof (verdict edge or #trivial waiver)" >&2
exit 0
