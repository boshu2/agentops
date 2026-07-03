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
# scan_lineage_for_equivalence <sha> <tip_pid> <tip_sig> — iterate the ledger's
# CONFIRMED lineage roots and test each for byte-equivalence to the tip. This is the
# AUTHORITATIVE, SECURITY-LOAD-BEARING scan (extracted so rebound_authorizes can run it
# BEFORE and AFTER a keep-ref fetch, age-rk3r.19, with identical semantics each time):
#   exit 0 — a reachable, re-derivable CONFIRMED lineage R (R != sha) is byte-equivalent.
#   exit 2 — no match AND >=1 well-formed hex lineage to_id could NOT be resolved/re-derived
#            here (the orphaned-reviewed-commit case).
#   exit 1 — no match and EVERY candidate was reachable (genuine no-proof / reachable forge).
# The candidates come ONLY from the ledger's hex-validated to_ids (confirmed_lineage_shas +
# hex_commit_object_id) — never from "whatever a ref points at". So a fetched keep-ref only
# makes the LEDGER-NAMED R's object present; it can never inject a DIFFERENT commit R' into
# this scan. That is the invariant that keeps a forged keep-ref harmless.
scan_lineage_for_equivalence() {
  local sha="$1" tip_pid="$2" tip_sig="$3"
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
  [[ "$saw_unreachable_lineage" -eq 1 ]] && return 2
  return 1
}

# ci_origin_remote — the remote a CI clone fetches from. Prefer an explicit override
# (AGENTOPS_KEEPREF_REMOTE, for tests / non-default CI), then a configured `origin`,
# then the FIRST configured remote. Empty if none (a keep-ref fetch is then impossible
# → fall through to fail-closed). git is invoked hardened as everywhere here. (age-rk3r.19)
ci_origin_remote() {
  if [[ -n "${AGENTOPS_KEEPREF_REMOTE:-}" ]]; then printf '%s' "$AGENTOPS_KEEPREF_REMOTE"; return 0; fi
  if git -c core.fsmonitor= -C "$REPO" remote get-url origin >/dev/null 2>&1; then printf 'origin'; return 0; fi
  local first
  first="$(git -c core.fsmonitor= -C "$REPO" remote 2>/dev/null | head -1)"
  [[ -n "$first" ]] && printf '%s' "$first"
  return 0
}

# fetch_rebound_keep_ref <tip-C-sha> — try to fetch refs/agentops/rebound/<C> from the CI
# origin so the reviewed commit R (which the write side pinned there, age-rk3r.19) becomes
# reachable in this clone. Exit 0 iff the fetch SUCCEEDED and the keep-ref now resolves to a
# commit object locally; exit 1 otherwise (no remote, ref absent, offline, denied).
#
# SECURITY: this is a REACHABILITY AID ONLY. It does NOT decide authorization — it merely
# makes an object available. The authoritative scan (scan_lineage_for_equivalence) then
# resolves R STRICTLY from the ledger's hex to_id and re-derives byte-equivalence, so a
# keep-ref pointing at a WRONG or non-equivalent commit CANNOT launder anything: it either
# fails to make the ledger-named R present (wrong R → scan still sees R unreachable → exit 2)
# or R is present but non-equivalent (→ exit 1). A forged keep-ref is inert here.
fetch_rebound_keep_ref() {
  local tip_c="$1"
  local remote; remote="$(ci_origin_remote)"
  [[ -n "$remote" ]] || return 1
  local ref="refs/agentops/rebound/${tip_c}"
  # Fetch the keep-ref into the SAME name locally (FETCH_HEAD also gets it). Hardened git;
  # a ref/object transfer (no diff drivers). Quiet + non-fatal.
  git -c core.fsmonitor= -C "$REPO" fetch --quiet "$remote" "${ref}:${ref}" >/dev/null 2>&1 || return 1
  # Confirm the fetched ref resolves to a commit object here (defensive; a server could
  # in principle answer without delivering — but git fetch of a missing ref errors, caught
  # above). The scan re-verifies sha==ledger-R independently regardless.
  git -c core.fsmonitor= -C "$REPO" rev-parse --verify --quiet "${ref}^{commit}" >/dev/null 2>&1 || return 1
  return 0
}

rebound_authorizes() {
  local sha="$1"
  rebound_edge_bound_to "$sha" || return 1   # no well-formed REBOUND edge → plain no-proof
  local tip_pid tip_sig
  tip_pid="$(commit_patch_id "$sha" "$REPO")"
  tip_sig="$(commit_content_sig "$sha" "$REPO")"
  # Empty key = cannot prove even the TIP's identity → plain no-proof (exit 1).
  [[ -n "$tip_pid" && -n "$tip_sig" ]] || return 1

  # FIRST scan against what is already reachable in this checkout.
  local rc=0
  scan_lineage_for_equivalence "$sha" "$tip_pid" "$tip_sig" || rc=$?
  if [[ "$rc" -ne 2 ]]; then
    return "$rc"   # 0 = authorized; 1 = reachable-but-no-match (genuine no-proof)
  fi

  # rc == 2: a well-formed CONFIRMED lineage exists but its reviewed commit R is NOT
  # reachable here (orphaned by a rebase — the clean-CI-clone case). Before giving up
  # (age-rk3r.18 exit-2), try to fetch the keep-ref the write side pushed to pin R
  # (refs/agentops/rebound/<C>). If it makes R reachable, RE-SCAN — the re-scan resolves
  # R strictly from the ledger's hex to_id and re-derives byte-equivalence, so this can only
  # AUTHORIZE a genuinely-equivalent, ledger-named, now-fetchable R (a forged/wrong/non-
  # equivalent keep-ref stays refused). (age-rk3r.19)
  if fetch_rebound_keep_ref "$sha"; then
    echo "tip-verdict-ci: fetched keep-ref refs/agentops/rebound/${sha:0:12} — re-verifying the reviewed commit's byte-equivalence" >&2
    local rc2=0
    scan_lineage_for_equivalence "$sha" "$tip_pid" "$tip_sig" || rc2=$?
    return "$rc2"   # 0 authorize; 2 still-unreachable (wrong-R keep-ref); 1 fetched-but-non-equivalent
  fi
  # No keep-ref (absent / no remote / offline) → unchanged age-rk3r.18 fail-closed exit-2.
  return 2
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
  # whose reviewed commit is unreachable here AND no keep-ref made it fetchable)
  # from plain NO-PROOF (1). When R is orphaned, rebound_authorizes first tries to
  # fetch the keep-ref refs/agentops/rebound/<C> (age-rk3r.19) and RE-VERIFIES the
  # ledger-named R's byte-equivalence — so an orphaned-but-keep-ref'd legitimate
  # REBOUND now AUTHORIZES; only when NEITHER R is reachable NOR a valid keep-ref
  # exists does it stay exit 2.
  rebound_rc=0
  rebound_authorizes "$sha" || rebound_rc=$?
  if [[ "$rebound_rc" -eq 0 ]]; then
    echo "::notice::commit ${sha:0:12} verified — REBOUND authorized (byte-equivalent to a CONFIRMED-reviewed commit) (${subject})"
    continue
  fi
  if [[ "$rebound_rc" -eq 2 ]]; then
    # HONEST SCOPING (age-rk3r.18/.19): the REBOUND edge is well-formed but the reviewed
    # commit it descends from is NOT reachable here AND no keep-ref made it fetchable
    # (refs/agentops/rebound/<C> absent, or the remote/network could not deliver it, or a
    # keep-ref resolved to the WRONG commit so the LEDGER-named R is still absent), so CI
    # cannot INDEPENDENTLY re-verify the byte-equivalence. Fail-closed (do NOT authorize),
    # with a DISTINCT, accurate message — never the misleading "lacks a bound verdict".
    # The LOCAL pre-push gate is authoritative for REBOUND (the reviewed commit is
    # reachable at push time); CI honors it when R is reachable OR a keep-ref makes it
    # fetchable. age-rk3r.19 shipped the keep-ref; reaching here means neither held.
    echo "::warning::commit ${sha:0:12} REBOUND: the reviewed commit it descends from is NOT reachable in this checkout and no keep-ref (refs/agentops/rebound/${sha:0:12}) made it fetchable (likely orphaned by a rebase, keep-ref unpushed/pruned, or offline), so the byte-equivalence cannot be independently re-verified — NOT authorized (fail-closed). The local pre-push gate is authoritative for REBOUND; for CI to honor it, ensure the keep-ref reached the remote or the commit needs a fresh verdict (${subject})"
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
