#!/usr/bin/env bash
# check-gate-tightening-ratchet.sh — ADVISORY gate.tightening-ratchet.
#
# WHY: this repo's law is "never obtain green by weakening acceptance"
# (AGENTS.md, Honest work and anti-ceremony). Nothing enforced it. A raised
# error budget, a lowered required count, a `Blocking: true -> false` demotion,
# a dropped `set -euo pipefail`, or a fresh `|| true` all buy a green run and
# all look like ordinary maintenance in review. `check-test-count-regression.sh`
# already proved the shape that works for one such knob (a per-package test
# count) — a diff-scoped detector with a named commit trailer as the deliberate
# escape hatch. This is that mechanism generalized to the gate surface itself.
#
# WHAT: over BASE_REF...HEAD, for changed files in the two governed classes
#   * cli/internal/gates/**            (the Go registry and its checks)
#   * scripts/check-*.sh               (top-level gate-backing scripts)
# detect LOOSENING and fail unless a `Gate-Loosen-Reason:` trailer appears in a
# commit body in range. TIGHTENING always passes, unconditionally.
#
# DETECTION CLASSES — the honest inventory. Each is fail-closed on what it
# detects and SILENT on what it cannot parse. This is a heuristic over a text
# diff, not a semantic analysis of gate behavior; a determined loosening can be
# written so no class below fires. Read a finding as a prompt to look, and read
# a clean run as "none of these six shapes was present", never as "acceptance
# was not weakened".
#
#   L1 numeric threshold moved in the loosening direction.
#      A removed line and an added line that are IDENTICAL except for their
#      numbers are paired. Direction is decided by the identifier on the line:
#        * an increase is loosening when the line names a ceiling
#          (max/budget/tolerance/allowance/limit/cap/grace/threshold/...);
#        * a decrease is loosening when the line names a floor
#          (min/required/at_least/floor/expected/...).
#      A line naming BOTH families, or NEITHER, is UNPARSED — reported in the
#      summary count, never as a finding. Lines are paired by normalized text
#      across the whole file diff, so an edit that both moves a threshold and
#      rewrites its line is invisible to this class.
#   L2 a registered check demoted from `Blocking: true` to `Blocking: false`
#      (matched by check ID across the removed/added lines).
#   L3 shell strictness removed: the count of `set -e` / `set -u` /
#      `pipefail` occurrences in a governed file DECREASED between the refs.
#   L4 a fail-open swallow ADDED: the count of `|| true` / `|| :` in a governed
#      file INCREASED. (The repo's own recurring defect — a `|| true` that turns
#      a helper's death into a clean certify-empty PASS.)
#   L5 a lint/security suppression ADDED: the count of `#nosec`, `nolint`,
#      `nosemgrep`, or `shellcheck disable` in a governed file INCREASED.
#   L6 a governed file DELETED (status D). A rename (status R) is not a
#      deletion and is not flagged.
#
#   NOT detected, deliberately: semantic weakening that leaves the numbers and
#   directives alone (a predicate inverted, a scope narrowed, a loop that stops
#   early, a fixture rewritten), any change to a gate's *inputs* (allowlists,
#   baselines — those carry their own shrink-only ratchets), and anything in a
#   file outside the two governed classes.
#
# ADVISORY-FIRST: registered Blocking:false in cli/internal/gates, so a finding
# surfaces as WARN. The flip to blocking is made later, deliberately, on
# measured evidence — never on a calendar (the evidence-floor lesson).
#
# Usage:
#   bash scripts/check-gate-tightening-ratchet.sh
#   BASE_REF=<ref> bash scripts/check-gate-tightening-ratchet.sh
#
# Env:
#   BASE_REF               base ref to diff against (default origin/main)
#   GATE_TIGHTENING_ROOT   repo root to operate on (test seam; default REPO_ROOT)
#
# Exit: 0 = no loosening detected / trailer present / not applicable / SKIP
#       1 = loosening detected with no Gate-Loosen-Reason: trailer in range
#       2 = misuse
#
# practices: [continuous-integration, measurement-over-assertion]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

ROOT="${GATE_TIGHTENING_ROOT:-$REPO_ROOT}"
BASE_REF="${BASE_REF:-origin/main}"

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --base) shift; [[ $# -gt 0 ]] || { echo "--base requires a ref" >&2; exit 2; }; BASE_REF="$1"; shift;;
        --base=*) BASE_REF="${1#--base=}"; shift;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

cd "$ROOT" || { echo "cannot enter '$ROOT'" >&2; exit 2; }

if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "SKIP: not inside a git repository"
    exit 0
fi

# Fail-open when the base ref cannot be resolved (shallow clone, missing fetch).
# The gate fails closed only on a loosening it can actually see, exactly like
# check-test-count-regression.sh.
if ! base_sha="$(git rev-parse --verify --quiet "${BASE_REF}^{commit}")"; then
    echo "SKIP: base ref '${BASE_REF}' is unresolvable (shallow clone or missing fetch) — fail-open"
    exit 0
fi

# --- governed scope ----------------------------------------------------------

# is_governed PATH — the two governed classes. scripts/check-*.sh is TOP-LEVEL
# only (scripts/lib/** and scripts/<sub>/** are libraries and sub-tools).
is_governed() {
    case "$1" in
        cli/internal/gates/*) return 0 ;;
        scripts/check-*.sh) [[ "$1" == scripts/*/* ]] && return 1; return 0 ;;
    esac
    return 1
}

changed_status="$(git diff --name-status "${base_sha}...HEAD" 2>/dev/null || true)"

governed=()
deleted=()
while IFS=$'\t' read -r status path rest; do
    [[ -n "${status:-}" && -n "${path:-}" ]] || continue
    # Renames arrive as "R<score>\told\tnew"; govern the NEW path.
    if [[ "$status" == R* && -n "${rest:-}" ]]; then
        path="$rest"
        status="M"
    fi
    is_governed "$path" || continue
    case "$status" in
        D) deleted+=("$path") ;;
        *) governed+=("$path") ;;
    esac
done <<< "$changed_status"

if [[ "${#governed[@]}" -eq 0 && "${#deleted[@]}" -eq 0 ]]; then
    echo "PASS: no governed gate files changed between ${BASE_REF} and HEAD — nothing to ratchet"
    exit 0
fi

# A single Gate-Loosen-Reason: trailer anywhere in the range is the deliberate
# escape hatch for the whole push (the Test-Removal-Reason: contract).
trailer_present=0
if git log "${base_sha}..HEAD" --format='%B' 2>/dev/null \
    | grep -qiE '^[[:space:]]*Gate-Loosen-Reason:[[:space:]]*[^[:space:]]'; then
    trailer_present=1
fi

# --- detection helpers -------------------------------------------------------

# count_at REF FILE ERE — occurrences of ERE in FILE at REF (HEAD reads the
# worktree). A file absent at the ref counts 0.
count_at() {
    local ref="$1" file="$2" ere="$3" out=""
    if [[ "$ref" == "WORKTREE" ]]; then
        [[ -f "$file" ]] || { echo 0; return 0; }
        out="$(grep -cE "$ere" "$file" 2>/dev/null || true)"
    else
        out="$(git show "$ref:$file" 2>/dev/null | grep -cE "$ere" || true)"
    fi
    [[ "$out" =~ ^[0-9]+$ ]] || out=0
    echo "$out"
}

# norm_numbers — replace every digit run with a fixed token, so two lines that
# differ ONLY in their numbers normalize to the same key.
norm_numbers() { sed -E 's/[0-9]+/@N@/g'; }

STRICTNESS_ERE='set[[:space:]]+-[a-zA-Z]*[eu]|pipefail'
FAILOPEN_ERE='\|\|[[:space:]]*(true|:)([[:space:]]|$)'
SUPPRESSION_ERE='#[[:space:]]*nosec|nolint|nosemgrep|shellcheck[[:space:]]+disable'
# Identifier families that decide the direction of a numeric move.
CEILING_ERE='(^|[^a-z])(max|maximum|budget|tolerance|allowance|allowed|limit|cap|ceiling|grace|threshold|warn_at|fail_at)([^a-z]|$)'
FLOOR_ERE='(^|[^a-z])(min|minimum|required|require|at_least|atleast|floor|expected)([^a-z]|$)'

findings=()
unparsed=0

# --- L6: deleted governed files ----------------------------------------------
for p in ${deleted[@]+"${deleted[@]}"}; do
    [[ -n "$p" ]] || continue
    findings+=("$p: governed gate file DELETED (a removed check is a removed acceptance)")
done

# --- per-file detection ------------------------------------------------------
for f in ${governed[@]+"${governed[@]}"}; do
    [[ -n "$f" ]] || continue
    present_at_base=0
    git cat-file -e "${base_sha}:${f}" 2>/dev/null && present_at_base=1

    # A file that did not exist at the base ref has no baseline to loosen: its
    # counts are all new. Only the paired-line class could apply, and it cannot
    # (no removed lines). Skip it entirely.
    [[ "$present_at_base" -eq 1 ]] || continue

    # L3/L4/L5 — count deltas between the refs.
    base_strict="$(count_at "$base_sha" "$f" "$STRICTNESS_ERE")"
    head_strict="$(count_at WORKTREE "$f" "$STRICTNESS_ERE")"
    if (( head_strict < base_strict )); then
        findings+=("$f: shell strictness removed (set -e / set -u / pipefail occurrences ${base_strict} -> ${head_strict})")
    fi

    base_open="$(count_at "$base_sha" "$f" "$FAILOPEN_ERE")"
    head_open="$(count_at WORKTREE "$f" "$FAILOPEN_ERE")"
    if (( head_open > base_open )); then
        findings+=("$f: fail-open swallow added (|| true / || : occurrences ${base_open} -> ${head_open})")
    fi

    base_sup="$(count_at "$base_sha" "$f" "$SUPPRESSION_ERE")"
    head_sup="$(count_at WORKTREE "$f" "$SUPPRESSION_ERE")"
    if (( head_sup > base_sup )); then
        findings+=("$f: lint/security suppression added (#nosec / nolint / nosemgrep / shellcheck disable occurrences ${base_sup} -> ${head_sup})")
    fi

    # Removed and added lines for this file, whole-diff (not per hunk): a
    # threshold edit shows up as one removed line and one added line that
    # normalize to the same key.
    file_diff="$(git diff --no-color -U0 "$base_sha" HEAD -- "$f" 2>/dev/null || true)"
    [[ -n "$file_diff" ]] || continue

    mapfile -t removed_lines < <(printf '%s\n' "$file_diff" | grep '^-' | grep -v '^---' | sed 's/^-//')
    mapfile -t added_lines < <(printf '%s\n' "$file_diff" | grep '^+' | grep -v '^+++' | sed 's/^+//')

    # Index the added lines by their number-normalized form. An empty key is
    # not a valid associative-array subscript and carries no signal anyway.
    declare -A added_by_key=()
    for a in ${added_lines[@]+"${added_lines[@]}"}; do
        [[ -n "$a" ]] || continue
        key="$(printf '%s' "$a" | norm_numbers)"
        [[ -n "$key" ]] || continue
        added_by_key["$key"]="$a"
    done

    # L2 — Blocking: true -> false, matched by check ID.
    for r in ${removed_lines[@]+"${removed_lines[@]}"}; do
        [[ "$r" == *"Blocking:"*"true"* ]] || continue
        [[ "$r" == *'ID:'* ]] || continue
        id="$(printf '%s' "$r" | sed -E 's/.*ID:[[:space:]]*"([^"]*)".*/\1/')"
        [[ -n "$id" && "$id" != "$r" ]] || continue
        for a in ${added_lines[@]+"${added_lines[@]}"}; do
            [[ "$a" == *"\"$id\""* ]] || continue
            [[ "$a" == *"Blocking:"*"false"* ]] || continue
            findings+=("$f: check \"$id\" demoted from blocking to advisory (Blocking: true -> false)")
            break
        done
    done

    # L1 — paired numeric threshold moves.
    for r in ${removed_lines[@]+"${removed_lines[@]}"}; do
        [[ -n "$r" ]] || continue
        [[ "$r" =~ [0-9] ]] || continue
        key="$(printf '%s' "$r" | norm_numbers)"
        a="${added_by_key[$key]:-}"
        [[ -n "$a" ]] || continue
        [[ "$a" != "$r" ]] || continue

        mapfile -t old_nums < <(printf '%s\n' "$r" | grep -oE '[0-9]+')
        mapfile -t new_nums < <(printf '%s\n' "$a" | grep -oE '[0-9]+')
        [[ "${#old_nums[@]}" -eq "${#new_nums[@]}" ]] || continue

        lc="$(printf '%s' "$r" | tr '[:upper:]' '[:lower:]')"
        has_ceiling=0; has_floor=0
        grep -qE "$CEILING_ERE" <<<"$lc" && has_ceiling=1
        grep -qE "$FLOOR_ERE" <<<"$lc" && has_floor=1

        for i in "${!old_nums[@]}"; do
            [[ "${old_nums[$i]}" != "${new_nums[$i]}" ]] || continue
            if (( has_ceiling && has_floor )); then
                unparsed=$((unparsed + 1))
            elif (( has_ceiling )) && (( 10#${new_nums[$i]} > 10#${old_nums[$i]} )); then
                findings+=("$f: ceiling raised — ${old_nums[$i]} -> ${new_nums[$i]} in \`$(printf '%s' "$r" | sed 's/^[[:space:]]*//')\`")
            elif (( has_floor )) && (( 10#${new_nums[$i]} < 10#${old_nums[$i]} )); then
                findings+=("$f: floor lowered — ${old_nums[$i]} -> ${new_nums[$i]} in \`$(printf '%s' "$r" | sed 's/^[[:space:]]*//')\`")
            else
                unparsed=$((unparsed + 1))
            fi
            break
        done
    done
    unset added_by_key
done

# --- report ------------------------------------------------------------------

if [[ "${#findings[@]}" -eq 0 ]]; then
    echo "PASS: gate tightening ratchet — no loosening detected in ${#governed[@]} governed file(s) (${unparsed} numeric change(s) undecidable, reported as unparsed)"
    exit 0
fi

if [[ "$trailer_present" -eq 1 ]]; then
    echo "PASS: loosening allowed by a Gate-Loosen-Reason: trailer in ${BASE_REF}..HEAD"
    printf '  %s\n' "${findings[@]}"
    exit 0
fi

{
    echo "FAIL: gate acceptance was LOOSENED without a Gate-Loosen-Reason: trailer"
    printf '  %s\n' "${findings[@]}"
    echo ""
    echo "Tightening is always free. If this loosening is deliberate, add a"
    echo "'Gate-Loosen-Reason: <one-line argument>' trailer to a commit in this range."
} >&2
exit 1
