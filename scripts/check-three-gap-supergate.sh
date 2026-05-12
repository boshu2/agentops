#!/usr/bin/env bash
# practices: [design-by-contract, continuous-integration]
# Three-Gap super-gates: emit a unified PASS/WARN/FAIL for each of the
# three contract gaps the project tracks (council coverage, durable
# learning, loop closure). Composes existing gates so the status surface
# is one command per gap instead of N separate gate invocations.
#
# Usage:
#   bash scripts/check-three-gap-supergate.sh --gap=council-coverage
#   bash scripts/check-three-gap-supergate.sh --gap=durable-learning
#   bash scripts/check-three-gap-supergate.sh --gap=loop-closure
#   bash scripts/check-three-gap-supergate.sh --gap=all
#
# Closes beads: soc-m47k.1 (TG1), soc-m47k.2 (TG2), soc-m47k.3 (TG3),
# soc-m47k (E5 epic).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GAP="all"
STRICT_COVERAGE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --gap=*) GAP="${1#--gap=}"; shift;;
        --gap) GAP="${2:-all}"; shift 2;;
        --strict-coverage) STRICT_COVERAGE=1; shift;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "Unknown arg: $1" >&2; exit 2;;
    esac
done

# Run a child gate, return its exit status; capture output if it fails.
run_gate() {
    local label="$1"
    local cmd="$2"
    if eval "$cmd" >/tmp/sg-"${label}".out 2>&1; then
        echo "  PASS  $label"
        return 0
    else
        echo "  FAIL  $label"
        sed 's/^/      /' /tmp/sg-"${label}".out | head -5
        return 1
    fi
}

# --- Gap 1: Council coverage ---
gap_council_coverage() {
    echo "Gap 1 — Council coverage:"
    local fails=0
    # PR-bound commits should have either a /pre-mortem or /vibe verdict.
    # Default lightweight implementation: count council files; pass if >= 1.
    # The --strict-coverage flag (opt-in) maps PR commits to council
    # references for the real coverage check; see strict_council_coverage()
    # below. soc-w6vh.6 acceptance: gate report distinguishes structural
    # availability from real coverage.
    local council_dir="$REPO_ROOT/.agents/council"
    local council_count=0
    if [ -d "$council_dir" ]; then
        council_count="$(find "$council_dir" -maxdepth 1 -name '*.md' -type f | wc -l)"
    fi
    if [ "$council_count" -ge 1 ]; then
        echo "  PASS  council artifacts present ($council_count files in $council_dir) [structural]"
    else
        # Greenfield CI runners have no .agents/council/; SKIP rather than fail.
        echo "  SKIP  no .agents/council/ on this box (operator-side surface; gate is structural)"
        return 0
    fi
    if [ "$STRICT_COVERAGE" -eq 1 ]; then
        strict_council_coverage "$council_dir" || fails=$((fails+1))
    else
        echo "  INFO  use --strict-coverage to map PR commits to council references (real coverage check)"
    fi
    return "$fails"
}

# --- Strict coverage: map PR commits to council references ---
# Returns 0 if all commits covered, 1 if any are missing. Uses
# PR_BASE_REF (operator override) or GITHUB_BASE_REF (CI) or main as the
# diff base. A commit is "covered" if any council artifact mentions its
# short SHA or the commit message has a Council/Pre-mortem/Vibe header.
strict_council_coverage() {
    local council_dir="$1"
    local base_ref="${PR_BASE_REF:-${GITHUB_BASE_REF:-main}}"
    local commits
    commits="$(git -C "$REPO_ROOT" log --format=%H "${base_ref}..HEAD" 2>/dev/null || true)"
    if [ -z "$commits" ]; then
        echo "  SKIP  --strict-coverage: no commits between ${base_ref}..HEAD (single-commit PR, missing base, or non-git)"
        return 0
    fi
    local total=0 covered=0
    local missing=""
    while IFS= read -r sha; do
        [ -z "$sha" ] && continue
        total=$((total + 1))
        local short="${sha:0:7}"
        if grep -lr -- "$short" "$council_dir" 2>/dev/null | head -1 | grep -q . \
           || git -C "$REPO_ROOT" log -1 --format=%B "$sha" 2>/dev/null \
                  | grep -qiE "^(Council|Pre-mortem|Vibe verdict)"; then
            covered=$((covered + 1))
        else
            missing="$missing $short"
        fi
    done <<< "$commits"
    if [ "$covered" -eq "$total" ]; then
        echo "  PASS  --strict-coverage: $covered/$total PR commits have council reference"
    else
        echo "  FAIL  --strict-coverage: $covered/$total PR commits have council reference (missing:${missing})"
        return 1
    fi
    return 0
}

# --- Gap 2: Durable learning ---
gap_durable_learning() {
    echo "Gap 2 — Durable learning:"
    local fails=0
    run_gate "flywheel-compounding-snapshot" \
        "bash $REPO_ROOT/scripts/check-flywheel-compounding-snapshot.sh" || fails=$((fails+1))
    if [ -x "$REPO_ROOT/cli/bin/ao" ] || [ -f "$REPO_ROOT/scripts/proof-run.sh" ]; then
        run_gate "flywheel-proof" \
            "bash $REPO_ROOT/scripts/proof-run.sh" || fails=$((fails+1))
    else
        echo "  SKIP  flywheel-proof (cli/bin/ao not built)"
    fi
    # compile-health requires .agents/defrag/latest.json (or an overnight
    # preview fallback). On greenfield CI runners neither exists, so the
    # check would fail purely on artifact absence rather than on durable-
    # learning health. The runtime-artifact-tagged goals (compile-freshness,
    # compile-no-oscillation) already surface that signal on operator
    # boxes; here we SKIP rather than fail, matching the Gap 1 council
    # SKIP pattern (operator-side surface; gate is structural).
    local defrag_latest="$REPO_ROOT/.agents/defrag/latest.json"
    local overnight_root="$REPO_ROOT/.agents/overnight"
    local has_overnight_preview=0
    if [ -d "$overnight_root" ] && \
       find "$overnight_root" -path '*/defrag/latest.json' -type f -print -quit 2>/dev/null | grep -q .; then
        has_overnight_preview=1
    fi
    if [ -f "$defrag_latest" ] || [ "$has_overnight_preview" -eq 1 ]; then
        run_gate "compile-health" \
            "bash $REPO_ROOT/scripts/check-compile-health.sh" || fails=$((fails+1))
    elif [ "${AGENTOPS_THREE_GAP_REQUIRE_COMPILE_HEALTH:-0}" = "1" ]; then
        echo "  FAIL  compile-health (no defrag artifact; AGENTOPS_THREE_GAP_REQUIRE_COMPILE_HEALTH=1)"
        fails=$((fails+1))
    else
        echo "  SKIP  compile-health (no defrag artifact; operator-side surface, gate is structural)"
    fi
    return "$fails"
}

# --- Gap 3: Loop closure ---
gap_loop_closure() {
    echo "Gap 3 — Loop closure:"
    local fails=0
    run_gate "goals-validate" \
        "bash -c 'cd $REPO_ROOT/cli && go build -o /tmp/ao-sg ./cmd/ao && cd .. && /tmp/ao-sg goals validate --json | jq -e .valid==true'" \
        || fails=$((fails+1))
    run_gate "wiring-closure" \
        "timeout 60 bash $REPO_ROOT/scripts/check-wiring-closure.sh" || fails=$((fails+1))
    if [ -x "$REPO_ROOT/cli/bin/ao" ] || [ -f "$REPO_ROOT/scripts/proof-run.sh" ]; then
        run_gate "flywheel-proof" \
            "bash $REPO_ROOT/scripts/proof-run.sh" || fails=$((fails+1))
    else
        echo "  SKIP  flywheel-proof (cli/bin/ao not built)"
    fi
    return "$fails"
}

case "$GAP" in
    council-coverage)
        gap_council_coverage
        rc=$?
        ;;
    durable-learning)
        gap_durable_learning
        rc=$?
        ;;
    loop-closure)
        gap_loop_closure
        rc=$?
        ;;
    all)
        gap_council_coverage; rc1=$?
        echo ""
        gap_durable_learning; rc2=$?
        echo ""
        gap_loop_closure; rc3=$?
        rc=$((rc1 + rc2 + rc3))
        ;;
    *)
        echo "Unknown gap: $GAP" >&2
        echo "Valid: council-coverage | durable-learning | loop-closure | all" >&2
        exit 2
        ;;
esac

echo ""
if [ "$rc" -eq 0 ]; then
    echo "three-gap super-gate ($GAP): PASS"
    exit 0
else
    echo "three-gap super-gate ($GAP): FAIL ($rc child(ren) failed)"
    exit 1
fi
