#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
ADJUDICATION="$SKILL_DIR/references/mandatory-checks.md"
CODEX_ADJUDICATION="$SKILL_DIR/../../skills-codex/premortem/references/mandatory-checks.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

# Exact shared plan-pawl contract. Backticks are Markdown data, not shell syntax.
# shellcheck disable=SC2016
KERNEL_MARKERS=(
  '## Constraints'
  '- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED is repair evidence: revise the plan and rerun automatically. Raise the andon and route one helper only for a true breaker such as missing authority, unavailable required trust domain after retry, or an impossible invariant.'
  '**Checkpoint:** before deliberation, confirm the packet records `scope_mode`, blast radius/reversibility, `author_id`, a distinct `judge_id`, and any required pre-registered `decision_rule`. Do not emit PASS while an invariant is missing.'
  'Apply the no-self-grading rule, cross-family rule for one-way doors, pre-registered decision rule, and discovery plan-pawl equivalence exactly as specified in [references/mandatory-checks.md](references/mandatory-checks.md#steps-2911-independent-adjudication-and-plan-pawl). A completed discovery plan-pawl duel is the premortem verdict for fanout-class discovery; do not run a duplicate council.'
  '## Output Specification'
  '- **Validator command:** `bash skills/premortem/scripts/validate.sh && grep -Eq '\''^## Council Verdict: (PASS|WARN|FAIL)$'\'' .agents/council/YYYY-MM-DD-premortem-<topic>.md`.'
  '- **Downstream handoff:** PASS proceeds to `/implement`; WARN or FAIL returns the plan to its author for repair and automatic re-review. Only a breaker raises the andon or routes one helper.'
  '## Quality Checklist'
  '- WARN, FAIL, and REFUTED routes repair and rerun; only a breaker routes the andon/helper path.'
)

# These backticks are exact Markdown state-machine markers, never substitutions.
# shellcheck disable=SC2016
STATE_MARKERS=(
  '### Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the plan and rerun the pawl; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** pause automation in HOLD and route exactly one bounded helper consultation.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, resume the automatic repair path, and re-earn an independent verdict before proceeding.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and surface the helper'\''s escalation to the human operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** stop automation and route directly to the human operator with the helper skipped.'
)

validate_markers() {
  local file="$1" marker
  shift
  [[ -s "$file" ]] || return 1
  for marker in "$@"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local file="$1" label="$2" marker variant="$TMP/missing-marker.md"
  shift 2
  for marker in "$@"; do
    grep -Fvx -- "$marker" "$file" >"$variant"
    if validate_markers "$variant" "$@"; then
      echo "premortem validator accepted a missing $label marker: $marker" >&2
      return 1
    fi
  done
}

validate_markers "$SKILL" "${KERNEL_MARKERS[@]}"
delete_one_negative_fixture "$SKILL" "kernel" "${KERNEL_MARKERS[@]}"
validate_markers "$ADJUDICATION" "${STATE_MARKERS[@]}"
delete_one_negative_fixture "$ADJUDICATION" "source state-machine" "${STATE_MARKERS[@]}"
validate_markers "$CODEX_ADJUDICATION" "${STATE_MARKERS[@]}"
delete_one_negative_fixture "$CODEX_ADJUDICATION" "Codex state-machine" "${STATE_MARKERS[@]}"

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: premortem" "grep -q '^name: premortem' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 3 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 3 ]"
check "SKILL.md mentions council delegation" "grep -Eq '(/|\\$)council' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions compiled premortem checks" "grep -q '\.agents/premortem-checks' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions finding registry fallback" "grep -q 'registry.jsonl' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions known_risks" "grep -q 'known_risks' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions dedup_key" "grep -q 'dedup_key' '$SKILL_DIR/SKILL.md'"
check "SKILL.md routes refresh through ao without a hook" "grep -q 'ao membrane digest' '$SKILL_DIR/SKILL.md' && ! grep -q 'finding-compiler.sh' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions plan-review preset" "grep -qi 'plan-review' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions PASS/WARN/FAIL verdicts" "grep -q 'PASS.*WARN.*FAIL\|PASS | WARN | FAIL' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions .agents/council/ output path" "grep -q '\.agents/council/' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions premortem report format" "grep -qi 'premortem report\|Premortem:' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions --deep mode" "grep -q '\-\-deep' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions --mixed mode" "grep -q '\-\-mixed' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions --debate mode" "grep -q '\-\-debate' '$SKILL_DIR/SKILL.md'"
check "SKILL.md stays within the 250-line kernel budget" "test \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250"
check "adjudication reference exists" "grep -q '^## Steps 2.9–2.11: Independent adjudication and plan-pawl$' '$SKILL_DIR/references/mandatory-checks.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
