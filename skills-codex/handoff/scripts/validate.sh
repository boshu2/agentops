#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check_function() {
  local name="$1" function_name="$2"
  if "$function_name"; then
    echo "PASS: $name"; PASS=$((PASS + 1))
  else
    echo "FAIL: $name"; FAIL=$((FAIL + 1))
  fi
}

require_handoff_artifact() {
  local file="$1" heading
  for heading in '## Objective' '## Verified state' '## Where we paused' '## Next action' '## Files to read' '## Validation evidence'; do
    grep -Fqx -- "$heading" "$file" || return 1
  done
  grep -Eq '^\*\*Captured:\*\* [0-9]{4}-[0-9]{2}-[0-9]{2}T[^ ]+$' "$file" || return 1
  grep -Eq '^\*\*Repository:\*\* .+$' "$file" || return 1
  grep -Eq '^\*\*HEAD:\*\* [0-9a-f]{40}$' "$file" || return 1
  grep -Eq '^\*\*Tracker:\*\* .+$' "$file" || return 1
  grep -Eq '^\*\*Pawl disposition:\*\* (CONFIRMED|REFUTED|HOLD|ESCALATE|REBOUND|none)$' "$file" || return 1
  grep -Eq '^\*\*Helper outcome:\*\* (UNSTUCK|ESCALATE|not-run)$' "$file" || return 1
}

require_prompt_artifact() {
  local file="$1" marker
  for marker in 'Read first:' 'First action:'; do
    grep -Fq -- "$marker" "$file" || return 1
  done
}

rejects_each_missing_handoff_marker() {
  local complete variant marker
  local -a markers=('## Objective' '## Verified state' '## Where we paused' '## Next action' '## Files to read' '## Validation evidence' '**Captured:** 2026-07-12T00:00:00Z' '**Repository:** /repo' '**HEAD:** 0123456789abcdef0123456789abcdef01234567' '**Tracker:** age-test in_progress' '**Pawl disposition:** CONFIRMED' '**Helper outcome:** not-run')
  complete="$(mktemp)"; variant="$(mktemp)"
  printf '%s\n' "${markers[@]}" >"$complete"
  require_handoff_artifact "$complete" || return 1
  for marker in "${markers[@]}"; do
    grep -Fvx -- "$marker" "$complete" >"$variant"
    if require_handoff_artifact "$variant"; then rm -f "$complete" "$variant"; return 1; fi
  done
  rm -f "$complete" "$variant"
}

rejects_each_missing_prompt_marker() {
  local complete variant marker
  local -a markers=('Read first: handoff.md' 'First action: git status')
  complete="$(mktemp)"; variant="$(mktemp)"
  printf '%s\n' "${markers[@]}" >"$complete"
  require_prompt_artifact "$complete" || return 1
  for marker in "${markers[@]}"; do
    grep -Fvx -- "$marker" "$complete" >"$variant"
    if require_prompt_artifact "$variant"; then rm -f "$complete" "$variant"; return 1; fi
  done
  rm -f "$complete" "$variant"
}

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: handoff" "grep -q '^name: handoff' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions session context" "grep -qi 'session context\|session continuation' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions .agents/ artifacts" "grep -q '\.agents/' '$SKILL_DIR/SKILL.md'"
check "constraints are front-loaded" "awk 'BEGIN{n=0;i=0;found=0} /^---$/{n++;next} n==2{i++; if (/^## Constraints$/){found=1;exit} if (i>80) exit} END{exit !found}' '$SKILL_DIR/SKILL.md'"
check "pawl transitions stay distinct" "grep -Fq 'continues auto-redo' '$SKILL_DIR/SKILL.md' && grep -Fq 'only a breaker enters' '$SKILL_DIR/SKILL.md' && grep -Fq 'alone authorizes the door' '$SKILL_DIR/SKILL.md' && grep -Fq 'resumes work but must re-earn' '$SKILL_DIR/SKILL.md'"
check "human escalation transitions are pinned" "grep -Fq 'reaches a human; refusal-lane work' '$SKILL_DIR/SKILL.md' && grep -Fq 'explicit judgment, and exhausted time/cost/quota budgets skip the helper and go directly to a human' '$SKILL_DIR/SKILL.md'"
check "verification checkpoints are explicit" "test \$(grep -c '^\*\*Checkpoint:\*\*' '$SKILL_DIR/SKILL.md') -ge 2"
check "output specification is complete" "grep -q '^## Output Specification$' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Path:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Filename:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Format:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Validation command:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Downstream handoff:\*\*' '$SKILL_DIR/SKILL.md'"
check "quality rubric has at least three rules" "awk '/^## Quality Checklist$/{f=1;next} f&&/^## /{exit} f&&/^- /{n++} END{exit !(n>=3)}' '$SKILL_DIR/SKILL.md'"
check "kernel stays within 250 lines" "test \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250"
check "artifact templates are embedded" "grep -q '^## Artifact Templates$' '$SKILL_DIR/SKILL.md' && grep -Fq '# Handoff: <Topic>' '$SKILL_DIR/SKILL.md' && grep -Fq '# Continuation: <Topic>' '$SKILL_DIR/SKILL.md'"
check "pawl disposition enum is pinned" "grep -Fq '**Pawl disposition:** <CONFIRMED | REFUTED | HOLD | ESCALATE | REBOUND | none>' '$SKILL_DIR/SKILL.md'"
check "helper outcome enum is separate" "grep -Fq '**Helper outcome:** <UNSTUCK | ESCALATE | not-run>' '$SKILL_DIR/SKILL.md' && grep -Fq 'as a disposition or treat it as authorization' '$SKILL_DIR/SKILL.md'"
check "runtime validation is all-of" "grep -Fq \"for heading in '## Objective' '## Verified state' '## Where we paused' '## Next action' '## Files to read' '## Validation evidence'\" '$SKILL_DIR/SKILL.md' && grep -Fq \"for marker in 'Read first:' 'First action:'\" '$SKILL_DIR/SKILL.md'"
check "restart metadata is runtime validated" "grep -Fq \"rg -q '^\\*\\*Captured:\\*\\*\" '$SKILL_DIR/SKILL.md' && grep -Fq \"rg -q '^\\*\\*Repository:\\*\\*\" '$SKILL_DIR/SKILL.md' && grep -Fq \"rg -q '^\\*\\*HEAD:\\*\\*\" '$SKILL_DIR/SKILL.md' && grep -Fq \"rg -q '^\\*\\*Tracker:\\*\\*\" '$SKILL_DIR/SKILL.md'"
check_function "every missing handoff marker is rejected" rejects_each_missing_handoff_marker
check_function "every missing prompt marker is rejected" rejects_each_missing_prompt_marker
check "Codex execution profile is retained" "grep -q '^## Codex Execution Profile$' '$SKILL_DIR/SKILL.md' && grep -Fq 'ao codex ensure-stop --auto-extract' '$SKILL_DIR/SKILL.md'"
check "Codex guardrails are retained" "grep -q '^## Guardrails$' '$SKILL_DIR/SKILL.md' && grep -Fq 'Do not leave the next session guessing what to do first.' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
