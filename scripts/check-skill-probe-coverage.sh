#!/usr/bin/env bash
# check-skill-probe-coverage.sh — ADVISORY skill.probe-coverage gate (age-e508.1).
#
# WHY: skills are half the product, but tier badges are editorial — the only
# enforcement is an enum-membership check. The historical 2026-06-30 graphify
# report (0/2 treatment responses obeyed the guidance) is reconstructed and
# LEGACY-UNVERIFIED, so it does not count as current coverage. A catalog whose
# product-/judgment-tier badges are UNMEASURED is noise wearing a product badge.
# This gate NAMES every product-/judgment-tier skill that carries no current,
# manifest-backed behavioral-probe RESULT.
#
# HONESTY: a probe measures BEHAVIOR-CHANGE (did the canonical skill treatment
# change what the agent DID), not quality-uplift. "Measured" here means "we ran
# a control vs treatment behavioral probe whose treatment prompt was sourced
# from the bound canonical SKILL.md, bound its capture metadata, and recorded a
# current verdict" — it does NOT assert the skill is good, only that this
# probe/config is no longer unknown (ADR-0011 discipline: do not overclaim).
#
# WHAT it checks:
#   * enumerate skills declaring `tier: product` or `tier: judgment` in the
#     first SKILL.md YAML frontmatter document (body text cannot spoof it);
#   * read the probe-status ledger in evals/skill-probes/LEDGER.md (the table
#     under "## Behavioral Probe Ledger (MEASUREMENT STATUS)"): rows "| skill | probe |
#     date | verdict | notes |". A current-result row must contain exactly one
#     `scorecard: `repo/relative/path.json`` note pointer. The ledger is
#     HAND-MAINTAINED in its own file — it
#     previously lived inside generated skills/SKILL-TIERS.md and a
#     regeneration wiped it (measured results cannot be derived from
#     frontmatter, so they must never live in a generated file);
#   * a skill "has a probe result" iff a BEHAVIORAL, INERT, or REGRESSIVE row
#     resolves to a safe repo-local v3 scorecard whose fixture manifest, bound inputs,
#     prompt events, transcript hashes, canonical skill source, non-overrideable
#     native runtime identity, reps, treatment mode, and recomputed discriminator
#     result all agree. Prelude-only evidence,
#     LEGACY-UNVERIFIED, UNMEASURED, missing/tampered evidence, or no row is not
#     a tier-coverage result;
#   * every product/judgment skill with no result is a finding — NAMED on output.
#
# ADVISORY-FIRST (the egwt warn-then-fail flip): default mode reports findings
# and exits 0 (WARN). --strict flips to a hard fail (exit 1) on any finding.
# The gate is registered non-blocking in the cockpit (cli/internal/gates), so it
# surfaces as WARN, never a release-blocking FAIL, until the spine is covered and
# the flip is made deliberately (Blocking:false -> true + this script's default).
#
# Usage:
#   bash scripts/check-skill-probe-coverage.sh            # advisory (exit 0)
#   bash scripts/check-skill-probe-coverage.sh --strict   # blocking (exit 1 on finding)
#   bash scripts/check-skill-probe-coverage.sh --json     # machine-readable summary
#
# Env overrides (test seams):
#   SKILL_PROBE_SKILLS_DIR     skills root (default: $REPO_ROOT/skills)
#   SKILL_PROBE_LEDGER_FILE    status ledger (default: $REPO_ROOT/evals/skill-probes/LEDGER.md)
#   SKILL_PROBE_TIERS_FILE     compatibility alias for SKILL_PROBE_LEDGER_FILE
#   SKILL_PROBE_EVIDENCE_ROOT  repository root for scorecards/probes (default: $REPO_ROOT)
#   SKILL_PROBE_METADATA_TOOL  verifier helper (default: scripts/lib/probe-fixture-metadata.py)
#
# Exit: 0 advisory/clean, 1 finding under --strict, 2 misuse.
#
# practices: [continuous-integration, measurement-over-assertion]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

SKILLS_DIR="${SKILL_PROBE_SKILLS_DIR:-$REPO_ROOT/skills}"
LEDGER_FILE="${SKILL_PROBE_LEDGER_FILE:-${SKILL_PROBE_TIERS_FILE:-$REPO_ROOT/evals/skill-probes/LEDGER.md}}"
EVIDENCE_ROOT="${SKILL_PROBE_EVIDENCE_ROOT:-$REPO_ROOT}"
METADATA_TOOL="${SKILL_PROBE_METADATA_TOOL:-$REPO_ROOT/scripts/lib/probe-fixture-metadata.py}"
STRICT=0
JSON=0

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift;;
        --json)   JSON=1; shift;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

if [[ ! -d "$SKILLS_DIR" ]]; then
    echo "skills dir not found: $SKILLS_DIR" >&2
    exit 2
fi
if [[ ! -d "$EVIDENCE_ROOT" ]]; then
    echo "evidence root not found: $EVIDENCE_ROOT" >&2
    exit 2
fi
if [[ ! -f "$METADATA_TOOL" ]]; then
    echo "probe evidence verifier not found: $METADATA_TOOL" >&2
    exit 2
fi

trim_cell() {
    printf '%s' "${1:-}" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//'
}

scorecard_ref_from_notes() {
    # shellcheck disable=SC2016 # Python source is intentionally single-quoted shell data.
    python3 -c '
import re, sys
refs = re.findall(r"scorecard:\s*`([^`]+)`", sys.argv[1])
if len(refs) != 1:
    raise SystemExit(1)
print(refs[0])
' "$1"
}

# Resolve the gate denominator first. Ledger rows outside this product/judgment
# set may be useful evidence, but they are not tier-coverage candidates and
# should not emit misleading "not measured" warnings from this gate.
declare -A GATED=()
declare -a GATED_NAMES=()
gated_total=0
if ! TIER_SUMMARY="$(python3 "$METADATA_TOOL" tier-skills --skills-dir "$SKILLS_DIR")"; then
    echo "could not parse canonical skill frontmatter for probe coverage" >&2
    exit 2
fi
while IFS= read -r name; do
    [[ -n "$name" ]] || continue
    GATED["$name"]=1
    GATED_NAMES+=("$name")
    gated_total=$((gated_total + 1))
done < <(python3 -c 'import json,sys; print("\n".join(json.loads(sys.argv[1])["skills"]))' "$TIER_SUMMARY")

# --- collect the set of skills that HAVE a current probe result ----------------
# A result = a ledger row with a directional current verdict whose referenced v3
# scorecard passes the mechanical verifier. The ledger lives under the
# "## Behavioral Probe Ledger" heading; rows are markdown table rows
# "| skill | probe | date | verdict | notes |". Parse defensively: a missing
# ledger file/section simply yields an empty measured set (every gated skill is
# then a finding — surfaced as advisory).
declare -A MEASURED=()
if [[ -f "$LEDGER_FILE" ]]; then
    in_ledger=0
    while IFS= read -r line; do
        # Enter the ledger section on its heading; exit on the next H2.
        if [[ "$line" =~ ^##[[:space:]]+Behavioral[[:space:]]+Probe[[:space:]]+Ledger ]]; then
            in_ledger=1
            continue
        fi
        if [[ $in_ledger -eq 1 && "$line" =~ ^##[[:space:]] ]]; then
            in_ledger=0
            continue
        fi
        [[ $in_ledger -eq 1 ]] || continue
        [[ "$line" == \|* ]] || continue
        # Columns: | skill | probe | date | verdict | notes |. For a current
        # verdict, skill/probe/verdict plus the notes' scorecard pointer are all
        # load-bearing and are checked again against the evidence.
        IFS='|' read -r _lead skill probe _date verdict notes _tail <<<"$line"
        skill="$(trim_cell "$skill")"
        probe="$(trim_cell "$probe")"
        verdict="$(trim_cell "$verdict" | tr '[:lower:]' '[:upper:]')"
        notes="$(trim_cell "$notes")"
        # Skip the header + separator rows.
        [[ -z "$skill" || "$skill" == "Skill" || "$skill" =~ ^:?-+:?$ ]] && continue
        # Strip surrounding backticks/asterisks a table author may add.
        skill="$(printf '%s' "$skill" | tr -d '`*')"
        probe="$(printf '%s' "$probe" | tr -d '`*')"
        if [[ "$verdict" == "BEHAVIORAL" || "$verdict" == "INERT" || "$verdict" == "REGRESSIVE" ]]; then
            [[ "$skill" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || continue
            [[ -n "${GATED[$skill]:-}" ]] || continue
            if ! scorecard_path="$(scorecard_ref_from_notes "$notes")"; then
                echo "::warning::probe ledger row '${skill}/${probe}' is not measured: current verdict lacks exactly one scorecard: \`path\` evidence pointer." >&2
                continue
            fi
            verification=""
            if verification="$(python3 "$METADATA_TOOL" verify-scorecard \
                --repo-root "$EVIDENCE_ROOT" \
                --skills-dir "$SKILLS_DIR" \
                --scorecard "$scorecard_path" \
                --ledger-skill "$skill" \
                --ledger-probe "$probe" \
                --ledger-verdict "$verdict" 2>&1)"; then
                MEASURED["$skill"]=1
            else
                verification="$(printf '%s' "$verification" | tail -n 1)"
                echo "::warning::probe ledger row '${skill}/${probe}' is not measured: ${verification:-evidence verification failed}" >&2
            fi
        fi
    done < "$LEDGER_FILE"
fi

# --- find gated skills without a verified current result ----------------------
declare -a UNMEASURED=()
for name in "${GATED_NAMES[@]}"; do
    if [[ -z "${MEASURED[$name]:-}" ]]; then
        UNMEASURED+=("$name")
    fi
done

finding_count=${#UNMEASURED[@]}

if [[ $JSON -eq 1 ]]; then
    printf '{"gated_total":%d,"measured":%d,"unmeasured_count":%d,"unmeasured":[' \
        "$gated_total" "$((gated_total - finding_count))" "$finding_count"
    for i in "${!UNMEASURED[@]}"; do
        [[ $i -gt 0 ]] && printf ','
        printf '"%s"' "${UNMEASURED[$i]}"
    done
    printf ']}\n'
fi

if [[ $finding_count -eq 0 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-skill-probe-coverage: PASS (${gated_total}/${gated_total} product/judgment skills carry a probe result)"
    exit 0
fi

if [[ $JSON -eq 0 ]]; then
    for name in "${UNMEASURED[@]}"; do
        echo "::warning::skill '${name}' (product/judgment tier) has NO current manifest-backed behavioral-probe result in the status ledger — its tier badge is unmeasured." >&2
    done
fi

if [[ $STRICT -eq 1 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-skill-probe-coverage: FAIL (${finding_count}/${gated_total} product/judgment skills unmeasured; --strict)" >&2
    exit 1
fi

[[ $JSON -eq 0 ]] && echo "check-skill-probe-coverage: WARN (${finding_count}/${gated_total} product/judgment skills unmeasured — probe them or accept doc-only value; advisory, not blocking)" >&2
exit 0
