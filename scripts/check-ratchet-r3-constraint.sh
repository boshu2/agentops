#!/usr/bin/env bash
set -euo pipefail

# check-ratchet-r3-constraint.sh — enforce Ratchet rule R3:
# "no learning without a constraint."
#
# Doctrine (docs/3.0.md, docs/architecture/operating-loop.md): a learning is
# durable only when it COMPILES into a gate/test/rule. The ratchet mechanism
# (cli/internal/ratchet/) already models maturity promotion, but R3 itself was
# never enforced — promotion to a durable tier was a manual operator decision
# with no check that the learning actually produced a constraint. This script
# closes that gap: a durable-tier learning that cites NO constraint is flagged.
#
# Why a script (not a CI path-filter gate): the operator's learnings corpus at
# .agents/learnings/** is gitignored (see .gitignore; the retired
# learning-coherence CI job is dead-by-design for the same reason). R3 therefore
# enforces against the LIVE local corpus at ratchet/flywheel time, and its own
# correctness is gated in CI by tests/scripts/check-ratchet-r3-constraint.bats.
#
# Promotion ladder (mirrors the ratchet's own warn-then-fail pattern, itself a
# canonical learning: .agents/learnings/pattern/users-agentops-warn-then-fail-ratchet.md):
#   - Default: WARN-only. Flags durable learnings missing a constraint, exit 0.
#   - Strict:  --strict OR RATCHET_R3_BLOCKING=true -> the same gaps FAIL, exit 1.
#
# A learning is "durable-tier" when maturity is one of: candidate, established,
# canonical, stable, promoted (i.e. past provisional). Provisional learnings are
# still forming and are exempt — R3 binds only once a learning claims durability.
#
# A learning "cites a constraint" when EITHER:
#   1. Frontmatter carries a constraint-link field:
#      constraint / enforced_by / gate / ratchet_gate / compiled_to / enforces
#   2. The body references a concrete constraint surface:
#      - a path under scripts/ (a *.sh gate)
#      - a path under .github/workflows/ (a CI gate)
#      - a *_test.go / tests/ reference (a test)
#      - a skills/**/SKILL.md reference (an encoded skill step / rule)
#      - an explicit "Constraint:" or "Enforced-by:" line
#
# Usage:
#   scripts/check-ratchet-r3-constraint.sh [LEARNINGS_DIR]
#   RATCHET_R3_BLOCKING=true scripts/check-ratchet-r3-constraint.sh
#   scripts/check-ratchet-r3-constraint.sh --strict [LEARNINGS_DIR]
#
# Exit codes: 0 = pass (or warn-only), 1 = blocking failures (strict mode).

STRICT="${RATCHET_R3_BLOCKING:-false}"
VERBOSE="${VERBOSE:-false}"

POSITIONAL=()
for arg in "$@"; do
    case "$arg" in
        --strict) STRICT=true ;;
        --verbose) VERBOSE=true ;;
        *) POSITIONAL+=("$arg") ;;
    esac
done

LEARNINGS_DIR="${POSITIONAL[0]:-.agents/learnings}"

FLAGGED=0
CHECKED=0
DURABLE=0

log() { [[ "$VERBOSE" == "true" ]] && echo "$@" || true; }

is_learning_artifact() {
    local basename
    basename=$(basename "$1")
    [[ "$basename" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}-.*\.md$ ]] || \
        [[ "$basename" =~ ^(learn|learning|pattern|users-).*\.md$ ]]
}

# Extract frontmatter (between the first and second '---' fences).
frontmatter_of() {
    awk 'NR==1 && $0!="---"{exit} NR==1{next} /^---$/{exit} {print}' "$1"
}

# Extract body (everything after the second '---').
body_of() {
    awk 'BEGIN{n=0} /^---$/{n++; if(n==2){found=1; next}} found{print}' "$1"
}

is_durable() {
    # $1 = frontmatter text
    echo "$1" | grep -qiE '^maturity:[[:space:]]*(candidate|established|canonical|stable|promoted)\b'
}

cites_constraint() {
    local frontmatter="$1" body="$2"
    # 1. Frontmatter constraint-link field (must have a non-empty value).
    if echo "$frontmatter" | grep -qiE '^(constraint|enforced_by|gate|ratchet_gate|compiled_to|enforces):[[:space:]]*[^[:space:]]'; then
        return 0
    fi
    # 2. Body references a concrete constraint surface.
    if echo "$body" | grep -qE 'scripts/[A-Za-z0-9_./-]+\.sh|\.github/workflows/|[A-Za-z0-9_./-]+_test\.go|(^|[[:space:]])tests/|skills/[A-Za-z0-9_./-]+/SKILL\.md'; then
        return 0
    fi
    # 3. Explicit constraint declaration line in the body.
    if echo "$body" | grep -qiE '^[[:space:]]*(Constraint|Enforced-by):[[:space:]]*[^[:space:]]'; then
        return 0
    fi
    return 1
}

check_file() {
    local file="$1" basename
    basename=$(basename "$file")
    [[ "$file" == *.md ]] || return 0
    is_learning_artifact "$file" || return 0

    CHECKED=$((CHECKED + 1))
    local frontmatter body
    frontmatter=$(frontmatter_of "$file")
    body=$(body_of "$file")

    # No frontmatter => no maturity claim => not durable-tier; exempt.
    [[ -n "$frontmatter" ]] || { log "skip(no-frontmatter): $basename"; return 0; }

    if ! is_durable "$frontmatter"; then
        log "skip(provisional): $basename"
        return 0
    fi
    DURABLE=$((DURABLE + 1))

    if cites_constraint "$frontmatter" "$body"; then
        log "PASS: $basename (durable + constraint cited)"
        return 0
    fi

    local rel="$file"
    if [[ "$STRICT" == "true" ]]; then
        echo "FAIL R3: $rel — durable-tier learning cites NO constraint (gate/test/SKILL/rule)"
    else
        echo "WARN R3: $rel — durable-tier learning cites NO constraint (gate/test/SKILL/rule)"
    fi
    FLAGGED=$((FLAGGED + 1))
}

main() {
    if [[ ! -d "$LEARNINGS_DIR" ]]; then
        echo "R3: no learnings directory at $LEARNINGS_DIR — nothing to check"
        exit 0
    fi

    while IFS= read -r -d '' file; do
        check_file "$file"
    done < <(find "$LEARNINGS_DIR" -type f -name '*.md' -print0 2>/dev/null)

    echo ""
    echo "R3 constraint check: $CHECKED learnings scanned, $DURABLE durable-tier, $FLAGGED missing a constraint"

    if [[ "$FLAGGED" -eq 0 ]]; then
        echo "R3 gate passed — every durable-tier learning compiles into a constraint"
        exit 0
    fi

    if [[ "$STRICT" == "true" ]]; then
        echo "R3 gate FAILED (strict): $FLAGGED durable-tier learning(s) without a constraint"
        echo "  Fix: add a constraint (gate/test/SKILL step) and cite it, or demote to maturity: provisional."
        exit 1
    fi

    echo "R3 gate WARN-ONLY: $FLAGGED durable-tier learning(s) without a constraint"
    echo "  Set RATCHET_R3_BLOCKING=true (or pass --strict) to make this blocking."
    exit 0
}

main
