#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

check() {
  if bash -c "$2"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
  fi
}

check "SKILL.md exists" "test -f '$SKILL_DIR/SKILL.md'"
check "SKILL.md has name: domain" "grep -q '^name: domain' '$SKILL_DIR/SKILL.md'"
check "constraints preserve JIT loading" "grep -q '^## Constraints' '$SKILL_DIR/SKILL.md' && grep -qi 'preloading.*defeats.*JIT' '$SKILL_DIR/SKILL.md'"
check "canonical promotion requires operator approval" "grep -qi 'never self-promote.*canonical' '$SKILL_DIR/SKILL.md'"
check "output specification is explicit" "grep -q '^## Output Specification' '$SKILL_DIR/SKILL.md'"
check "output declares path and filename" "grep -q '\*\*Path:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Filename:\*\*' '$SKILL_DIR/SKILL.md'"
check "output declares validation and handoff" "grep -qi 'validation command' '$SKILL_DIR/SKILL.md' && grep -qi 'downstream handoff' '$SKILL_DIR/SKILL.md'"
check "mutations target canonical domain source" "grep -q 'skills/domain/references/<slug>.md' '$SKILL_DIR/SKILL.md' && grep -q 'skills/domain/references/INDEX.md' '$SKILL_DIR/SKILL.md'"
check "Codex projection is read-only" "grep -Eqi 'skills-codex/domain/.*generated, read-only consumption projection' '$SKILL_DIR/SKILL.md' && grep -qi 'Never edit it' '$SKILL_DIR/SKILL.md'"
check "Codex projection requires regeneration and convergence" "grep -q 'codex-sync.sh --force --only domain' '$SKILL_DIR/SKILL.md' && grep -q 'codex-sync.sh --check --only domain' '$SKILL_DIR/SKILL.md'"
check "no generated-tree mutation instruction" "! grep -Eqi 'skills-codex/domain/.*(write|mutat|update)|(write|mutat|update).*skills-codex/domain/' '$SKILL_DIR/SKILL.md'"
check "quality checklist is explicit" "grep -q '^## Quality Checklist' '$SKILL_DIR/SKILL.md'"
check "domain index and entry schema exist" "test -f '$SKILL_DIR/references/INDEX.md' && test -f '$SKILL_DIR/references/entry.md'"

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
