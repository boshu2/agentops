#!/usr/bin/env bash
# test-ci-policy-parity-wave-gate.sh — assert Wave 1G wiring of
# scripts/validate-ci-policy-parity.sh into crank's Step 5.5 wave acceptance
# as a conditional gate that fires when a wave touches a workflow YAML file.
#
# Bead: soc-il9k (Wave 1G of epic soc-xlw8).
# Motivation: commit c587b361 was a manual fix after soc-lmww1 added an
# advisory job to validate.yml without updating AGENTS.md or summary.needs.
# This test pins the formalization of that recurrence-prevention rule.
#
# Asserts:
#   T1. The conditional gate is documented in skills/crank/SKILL.md Step 5.5.
#   T2. The gate uses the narrow trigger pattern (workflow YAML files only).
#   T3. references/wave-patterns.md has a "CI-Policy Parity Gate" section
#       linked from SKILL.md.
#   T4. The worker prompt template (skills/swarm/references/local-mode.md)
#       references the parity gate as a conditional preflight.
#   T5. Codex parity copies (skills-codex/crank/, skills-codex/swarm/) carry
#       matching gate references.
#   T6. The validator dependency exists and is executable.
#   T7. Behavioral fixture: synthesize a drifted (validate.yml, AGENTS.md)
#       pair under CI_POLICY_PARITY_* env overrides and assert exit 1.
#   T8. Behavioral fixture: synthesize an aligned pair and assert exit 0.
#   T9. Sanity: invoking the validator on the live repo exits 0
#       (current main is parity-clean — guards against repo-level regression).
#
# Usage: bash tests/integration/test-ci-policy-parity-wave-gate.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# shellcheck disable=SC1091
source "$REPO_ROOT/tests/lib/colors.sh"

PASS=0
FAIL=0

ok() {
    pass "$1"
    PASS=$((PASS + 1))
}

ko() {
    fail "$1"
    FAIL=$((FAIL + 1))
}

VALIDATOR="$REPO_ROOT/scripts/validate-ci-policy-parity.sh"
CRANK_SKILL="$REPO_ROOT/skills/crank/SKILL.md"
WAVE_PATTERNS="$REPO_ROOT/skills/crank/references/wave-patterns.md"
WORKER_TEMPLATE="$REPO_ROOT/skills/swarm/references/local-mode.md"
CODEX_CRANK_SKILL="$REPO_ROOT/skills-codex/crank/SKILL.md"
CODEX_WAVE_PATTERNS="$REPO_ROOT/skills-codex/crank/references/wave-patterns.md"
CODEX_WORKER_TEMPLATE="$REPO_ROOT/skills-codex/swarm/references/local-mode.md"

# ---------- T1: gate documented in crank SKILL.md ----------
if grep -q 'validate-ci-policy-parity' "$CRANK_SKILL" \
   && grep -q 'CI-Policy Parity Gate' "$CRANK_SKILL"; then
    ok "T1 crank SKILL.md Step 5.5 references CI-Policy Parity Gate + validator"
else
    ko "T1 crank SKILL.md missing CI-Policy Parity Gate wiring"
fi

# ---------- T2: narrow trigger pattern documented (workflows YAML only) ----------
# Combined check across SKILL.md + wave-patterns.md (trigger is documented in
# wave-patterns; SKILL.md may carry only the summary form).
if grep -qE '\^\\\.github/workflows/.*\\\.ya\?ml\$' "$CRANK_SKILL" "$WAVE_PATTERNS"; then
    ok "T2 narrow trigger pattern documented (workflow YAML only)"
else
    ko "T2 narrow trigger pattern missing in crank docs"
fi

# ---------- T3: wave-patterns.md has worked example, linked from SKILL.md ----------
if grep -q '## CI-Policy Parity Gate' "$WAVE_PATTERNS" \
   && grep -q 'c587b361' "$WAVE_PATTERNS" \
   && grep -q 'wave-patterns.md' "$CRANK_SKILL"; then
    ok "T3 wave-patterns.md carries worked example with c587b361 motivation, linked from SKILL.md"
else
    ko "T3 wave-patterns.md worked example missing or not linked"
fi

# ---------- T4: worker prompt template carries the preflight ----------
if grep -q 'CI-policy parity preflight' "$WORKER_TEMPLATE" \
   && grep -q 'validate-ci-policy-parity' "$WORKER_TEMPLATE"; then
    ok "T4 worker prompt template (swarm/local-mode.md) references parity preflight"
else
    ko "T4 worker prompt template missing parity preflight"
fi

# ---------- T5: codex parity copies carry matching wiring ----------
codex_ok=1
for f in "$CODEX_CRANK_SKILL" "$CODEX_WAVE_PATTERNS" "$CODEX_WORKER_TEMPLATE"; do
    if [[ ! -f "$f" ]]; then
        codex_ok=0
        ko "T5 codex parity file missing: ${f#$REPO_ROOT/}"
        continue
    fi
    if ! grep -q 'validate-ci-policy-parity' "$f"; then
        codex_ok=0
        ko "T5 codex parity file lacks validator reference: ${f#$REPO_ROOT/}"
    fi
done
if [[ "$codex_ok" -eq 1 ]]; then
    ok "T5 codex parity copies (crank + swarm) reference validate-ci-policy-parity"
fi

# ---------- T6: validator dependency exists ----------
if [[ -x "$VALIDATOR" ]]; then
    ok "T6 validator is present and executable: scripts/validate-ci-policy-parity.sh"
else
    ko "T6 validator missing or not executable: scripts/validate-ci-policy-parity.sh"
fi

# ---------- Behavioral fixtures (T7, T8) ----------
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Minimal AGENTS.md with the table header the validator looks for.
# The validator extracts non-blocking jobs by grepping for the literal
# "<job> (non-blocking)" prose pattern, so the stub emits a
# "Non-blocking jobs:" enumeration ABOVE the table for any nonblocking entries.
make_agents_md() {
    local out="$1"
    shift
    {
        echo "# Stub AGENTS.md"
        echo ""
        # Inline non-blocking enumeration the validator's regex matches.
        local nb_line=""
        local sep=""
        for entry in "$@"; do
            local job="${entry%%:*}"
            local kind="${entry##*:}"
            if [[ "$kind" == "nonblocking" ]]; then
                nb_line+="${sep}${job} (non-blocking)"
                sep=", "
            fi
        done
        if [[ -n "$nb_line" ]]; then
            echo "Non-blocking jobs: ${nb_line}."
            echo ""
        fi
        echo "### CI Jobs and What They Check"
        echo ""
        echo "| Job | What it validates | Common failure |"
        echo "|-----|-------------------|----------------|"
        for entry in "$@"; do
            local job="${entry%%:*}"
            local kind="${entry##*:}"
            if [[ "$kind" == "blocking" ]]; then
                echo "| **${job}** | stub | stub |"
            else
                echo "| **${job}** | stub | Non-blocking (\`continue-on-error: true\`); stub |"
            fi
        done
    } > "$out"
}

# Minimal validate.yml with summary.needs and modern contains() failset rule.
# Args: pairs of "job:blocking" or "job:nonblocking".
make_workflow_yml() {
    local out="$1"
    shift
    {
        echo "name: stub"
        echo "on: [push]"
        echo "jobs:"
        for entry in "$@"; do
            local job="${entry%%:*}"
            local kind="${entry##*:}"
            echo "  ${job}:"
            echo "    runs-on: ubuntu-latest"
            if [[ "$kind" == "nonblocking" ]]; then
                echo "    continue-on-error: true"
            fi
            echo "    steps:"
            echo "      - run: echo ${job}"
        done
        # Build needs list
        local needs=""
        local sep=""
        for entry in "$@"; do
            local job="${entry%%:*}"
            needs+="${sep}${job}"
            sep=", "
        done
        echo "  summary:"
        echo "    needs: [${needs}]"
        echo "    runs-on: ubuntu-latest"
        echo "    if: \${{ always() && contains(needs.*.result, 'failure') }}"
        echo "    steps:"
        echo "      - run: echo summary"
    } > "$out"
}

# T7: drifted state — workflow has new advisory job, AGENTS.md does NOT.
DRIFT_DIR="$TMP_DIR/drift"
mkdir -p "$DRIFT_DIR/.github/workflows"
make_agents_md "$DRIFT_DIR/AGENTS.md" \
    "alpha-gate:blocking" \
    "beta-gate:blocking"
make_workflow_yml "$DRIFT_DIR/.github/workflows/validate.yml" \
    "alpha-gate:blocking" \
    "beta-gate:blocking" \
    "factory-claim-ledger-strict:nonblocking"

if CI_POLICY_PARITY_AGENTS_PATH="$DRIFT_DIR/AGENTS.md" \
   CI_POLICY_PARITY_WORKFLOW_PATH="$DRIFT_DIR/.github/workflows/validate.yml" \
   bash "$VALIDATOR" >"$TMP_DIR/drift.out" 2>&1; then
    ko "T7 drifted fixture should exit non-zero but exited 0"
    sed 's/^/    /' "$TMP_DIR/drift.out" >&2
else
    drift_rc=$?
    if [[ "$drift_rc" -ne 0 ]] && grep -q 'CI_POLICY_PARITY' "$TMP_DIR/drift.out"; then
        ok "T7 drifted fixture (advisory job missing from AGENTS.md) exits ${drift_rc} with parity error"
    else
        ko "T7 drifted fixture exited ${drift_rc} but did not emit a CI_POLICY_PARITY error"
        sed 's/^/    /' "$TMP_DIR/drift.out" >&2
    fi
fi

# T8: aligned state — same job set + non-blocking annotation aligned.
ALIGN_DIR="$TMP_DIR/align"
mkdir -p "$ALIGN_DIR/.github/workflows"
make_agents_md "$ALIGN_DIR/AGENTS.md" \
    "alpha-gate:blocking" \
    "beta-gate:blocking" \
    "factory-claim-ledger-strict:nonblocking"
make_workflow_yml "$ALIGN_DIR/.github/workflows/validate.yml" \
    "alpha-gate:blocking" \
    "beta-gate:blocking" \
    "factory-claim-ledger-strict:nonblocking"

if CI_POLICY_PARITY_AGENTS_PATH="$ALIGN_DIR/AGENTS.md" \
   CI_POLICY_PARITY_WORKFLOW_PATH="$ALIGN_DIR/.github/workflows/validate.yml" \
   bash "$VALIDATOR" >"$TMP_DIR/align.out" 2>&1; then
    ok "T8 aligned fixture exits 0 (PASS)"
else
    align_rc=$?
    ko "T8 aligned fixture should exit 0 but exited ${align_rc}"
    sed 's/^/    /' "$TMP_DIR/align.out" >&2
fi

# ---------- T9: live repo sanity ----------
if bash "$VALIDATOR" >"$TMP_DIR/live.out" 2>&1; then
    ok "T9 live repo passes validate-ci-policy-parity (no drift on main)"
else
    live_rc=$?
    ko "T9 live repo validate-ci-policy-parity exited ${live_rc} (drift on main?)"
    sed 's/^/    /' "$TMP_DIR/live.out" >&2
fi

echo ""
if [[ "$FAIL" -gt 0 ]]; then
    red "FAILED — $FAIL/$((PASS + FAIL)) checks failed" >&2
    exit 1
fi

green "PASSED — $PASS/$PASS checks passed"
exit 0
