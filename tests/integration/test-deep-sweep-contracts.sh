#!/usr/bin/env bash
# Contract test: Deep Sweep Architecture
# Validates that the two-phase sweep+adjudicate architecture is correctly
# wired across vibe, post-mortem, council, and gate-retry-logic.
#
# No Claude CLI needed — pure structural/contract validation.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PASS=0; FAIL=0; TOTAL=0

check() {
  local label="$1"
  local cmd="$2"
  TOTAL=$((TOTAL + 1))
  if bash -c "$cmd" >/dev/null 2>&1; then
    echo "  ✓ $label"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $label"
    FAIL=$((FAIL + 1))
  fi
}

echo "═══════════════════════════════════════════"
echo "Deep Sweep Architecture Contract Tests"
echo "═══════════════════════════════════════════"
echo ""

# ── 1. deep-audit-protocol.md existence and key sections ──────────────
echo "1. Deep Audit Protocol Reference"

PROTOCOL="$REPO_ROOT/skills/validate/references/deep-audit-protocol.md"

check "deep-audit-protocol.md exists" \
  "[ -f '$PROTOCOL' ]"

check "Has file chunking rules" \
  "grep -q 'File Chunking Rules' '$PROTOCOL'"

check "Has 8-category checklist" \
  "grep -q '8-Category Checklist' '$PROTOCOL'"

check "Lists all 8 categories" \
  "[ \$(grep -cE '^[0-9]+\. \*\*' '$PROTOCOL') -ge 8 ]"

check "Has explorer prompt template" \
  "grep -q 'Explorer Prompt Template' '$PROTOCOL'"

check "Has sweep manifest format" \
  "grep -q 'Sweep Manifest' '$PROTOCOL'"

check "Has council adjudication section" \
  "grep -q 'Council Adjudication' '$PROTOCOL'"

check "Has flag behavior table" \
  "grep -q 'Flag Behavior' '$PROTOCOL'"

check "References --sweep flag" \
  "grep -q '\-\-sweep' '$PROTOCOL'"

check "References --skip-sweep flag" \
  "grep -q '\-\-skip-sweep' '$PROTOCOL'"

echo ""

# ── 2. vibe SKILL.md wiring ──────────────────────────────────────────
echo "2. Vibe SKILL.md"

VIBE="$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"

check "--sweep flag in Quick Start" \
  "grep -q '\-\-sweep recent' '$VIBE'"

check "Deep audit sweep documented for vibe" \
  "grep -q 'sweep manifest' '$VIBE' && grep -q 'deep-audit-protocol.md' '$VIBE'"

check "Lightweight bug hunt path still documented" \
  "grep -q 'Step 2e: Bug Hunt' '$VIBE'"

check "Step 2e references deep-audit-protocol.md" \
  "grep -q 'deep-audit-protocol.md' '$VIBE'"

check "Council receives sweep manifest" \
  "grep -q 'sweep_manifest' '$VIBE'"

check "All findings section in report template" \
  "grep -qi 'all findings' '$VIBE'"

check "No 'top 5' cap in Step 9" \
  "! grep -q 'top 5 findings' '$VIBE'"

check "No ':0:3' slice in Step 9.5" \
  "! grep -q ':0:3' '$VIBE'"

check "Reference Documents links deep-audit-protocol.md" \
  "grep -q '\[references/deep-audit-protocol.md\]' '$VIBE'"

echo ""

# ── 3. Council agent-prompts.md adjudication mode ────────────────────
# RETIRED (ag-s43tg, 2026-06-12): council's judge prompt pack — including the
# adjudication-mode block — was deliberately extracted to mt-olympus in Lane A
# (0c7ef56c0); skills/council is now a pointer stub and carries no
# references/agent-prompts.md. The sweep→adjudication handoff contract now
# lives in the olympusd gate, not this corpus. Sections 1–2 above still guard
# the surviving surfaces (the rescued deep-audit protocol + quick-mode doc
# under skills/validate/references/).

# ── 4. Post-mortem SKILL.md wiring ───────────────────────────────────
# Step 2.6 / sweep-block asserts retired with the Lane A extraction (see §3
# note). The no-cap reporting asserts below still apply to the surviving
# post-mortem skill.
echo "4. Post-Mortem sweep wiring (references/execution-steps.md) + no-cap reporting"

PM="$REPO_ROOT/skills/postmortem/SKILL.md"
PMSTEPS="$REPO_ROOT/skills/postmortem/references/execution-steps.md"

check "Step 2.6 exists in execution steps (Pre-Council Deep Audit Sweep)" \
  "grep -q 'Step 2.6' '$PMSTEPS'"

check "Step 2.6 title mentions deep audit" \
  "grep -q 'Pre-Council Deep Audit Sweep' '$PMSTEPS'"

check "--skip-sweep flag documented in execution steps" \
  "grep -q '\-\-skip-sweep' '$PMSTEPS'"

check "No 'at least 5 improvements' cap in Step 5.5" \
  "! grep -q 'at least \*\*5\*\*' '$PM'"

check "No 'top 3' cap in Step 7 report" \
  "! grep -q '(top 3)' '$PM'"

echo ""

# ── 5. Gate-retry-logic.md cap removal ───────────────────────────────
echo "5. Gate Retry Logic"

GATES="$REPO_ROOT/skills/rpi/references/gate-retry-logic.md"

check "No 'top 5' cap in pre-mortem gate" \
  "! grep -q 'top 5' '$GATES'"

check "No '(max 5)' cap in pre-mortem gate" \
  "! grep -q '(max 5)' '$GATES'"

check "Pre-mortem gate extracts ALL findings" \
  "grep -q 'Extract ALL findings' '$GATES'"

check "Vibe gate extracts ALL findings" \
  "[ \$(grep -c 'Extract ALL findings' '$GATES') -eq 2 ]"

check "Group by category hint present" \
  "grep -q 'group by category' '$GATES'"

echo ""

# ── 6. Cross-file consistency ────────────────────────────────────────
echo "6. Cross-File Consistency"

check "Quick-mode doc and protocol agree on batch sizes (3-5)" \
  "grep -q 'batch.* 3' '$PROTOCOL' && grep -Eq '3[-–]5' '$VIBE'"

echo ""

# ── Summary ──────────────────────────────────────────────────────────
echo "═══════════════════════════════════════════"
if [ "$FAIL" -eq 0 ]; then
  echo "PASS: All $TOTAL contract checks passed"
else
  echo "FAIL: $PASS/$TOTAL passed, $FAIL failed"
fi
echo "═══════════════════════════════════════════"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
