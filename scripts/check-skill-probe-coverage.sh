#!/usr/bin/env bash
# check-skill-probe-coverage.sh — ADVISORY skill.probe-coverage gate (age-e508.1).
#
# WHY: skills are half the product, but tier badges are editorial — the only
# enforcement is an enum-membership check. A 2026-06-30 A/B measured a
# doc-instruction skill (graphify) as behaviorally INERT: 0/2 treatment agents
# obeyed it. A catalog whose product-/judgment-tier badges are UNMEASURED is
# noise wearing a product badge. This gate NAMES every product-/judgment-tier
# skill that carries no behavioral-probe RESULT.
#
# HONESTY: a probe measures BEHAVIOR-CHANGE (did the loaded skill change what the
# agent DID), not quality-uplift. "Measured" here means "we ran a control vs
# treatment behavioral probe and recorded a verdict" — it does NOT assert the
# skill is good, only that its behavioral value is no longer unknown (ADR-0011
# discipline: do not overclaim).
#
# WHAT it checks:
#   * enumerate skills declaring `tier: product` or `tier: judgment` in their
#     SKILL.md metadata frontmatter;
#   * read the MEASURED probe ledger in skills/SKILL-TIERS.md (the table under
#     "## Behavioral Probe Ledger (MEASURED)"): rows "| skill | probe | date |
#     verdict |";
#   * a skill "has a probe result" iff a ledger row names it with verdict
#     BEHAVIORAL or INERT. An UNMEASURED verdict, or no row at all, is NOT a
#     result;
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
#   SKILL_PROBE_SKILLS_DIR   skills root (default: $REPO_ROOT/skills)
#   SKILL_PROBE_TIERS_FILE   MEASURED ledger file (default: $REPO_ROOT/skills/SKILL-TIERS.md)
#
# Exit: 0 advisory/clean, 1 finding under --strict, 2 misuse.
#
# practices: [continuous-integration, measurement-over-assertion]
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

SKILLS_DIR="${SKILL_PROBE_SKILLS_DIR:-$REPO_ROOT/skills}"
TIERS_FILE="${SKILL_PROBE_TIERS_FILE:-$REPO_ROOT/skills/SKILL-TIERS.md}"
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

# --- collect the set of skills that HAVE a measured probe result --------------
# A result = a ledger row with verdict BEHAVIORAL or INERT. The ledger lives
# under the "## Behavioral Probe Ledger" heading; rows are markdown table rows
# "| skill | probe | date | verdict |". Parse defensively: a missing ledger
# file/section simply yields an empty measured set (every gated skill is then a
# finding — surfaced as advisory).
declare -A MEASURED=()
if [[ -f "$TIERS_FILE" ]]; then
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
        # Split the table row on '|'. Columns: | skill | probe | date | verdict |
        # — only skill (col 2) and verdict (col 5) are load-bearing; the rest are
        # throwaways (`_`).
        IFS='|' read -r _ skill _ _ verdict _rest <<<"$line"
        skill="$(printf '%s' "${skill:-}" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
        verdict="$(printf '%s' "${verdict:-}" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' | tr '[:lower:]' '[:upper:]')"
        # Skip the header + separator rows.
        [[ -z "$skill" || "$skill" == "Skill" || "$skill" =~ ^:?-+:?$ ]] && continue
        # Strip surrounding backticks/asterisks a table author may add.
        skill="$(printf '%s' "$skill" | tr -d '`*')"
        if [[ "$verdict" == "BEHAVIORAL" || "$verdict" == "INERT" ]]; then
            MEASURED["$skill"]=1
        fi
    done < "$TIERS_FILE"
fi

# --- enumerate product/judgment skills and find the unmeasured ones -----------
declare -a UNMEASURED=()
gated_total=0
for skill_md in "$SKILLS_DIR"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    # Runtime compatibility pointers are aliases, not independent skills with
    # behavioral value to measure. Their redirect contract has its own gate.
    grep -Eq '^implementation:[[:space:]]+false([[:space:]]|$)' "$skill_md" && continue
    # tier lives in the metadata frontmatter as `  tier: <value>` (first hit).
    # A skill without a tier is outside this advisory gate; do not let grep's
    # no-match status abort the whole scan under the shared strict preamble.
    tier="$({ grep -m1 -E '^[[:space:]]*tier:[[:space:]]*' "$skill_md" 2>/dev/null || true; } \
              | sed -E 's/^[[:space:]]*tier:[[:space:]]*//; s/[[:space:]].*$//' | tr -d '"'"'"'`' )"
    case "$tier" in
        product|judgment) ;;
        *) continue;;
    esac
    gated_total=$((gated_total + 1))
    name="$(basename "$(dirname "$skill_md")")"
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
        echo "::warning::skill '${name}' (product/judgment tier) has NO behavioral-probe result in the MEASURED ledger — its tier badge is unmeasured." >&2
    done
fi

if [[ $STRICT -eq 1 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-skill-probe-coverage: FAIL (${finding_count}/${gated_total} product/judgment skills unmeasured; --strict)" >&2
    exit 1
fi

[[ $JSON -eq 0 ]] && echo "check-skill-probe-coverage: WARN (${finding_count}/${gated_total} product/judgment skills unmeasured — probe them or accept doc-only value; advisory, not blocking)" >&2
exit 0
