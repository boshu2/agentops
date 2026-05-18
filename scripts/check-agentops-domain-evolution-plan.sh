#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BDD="$ROOT/docs/reference/agentops-domain-evolution-bdd.md"
MAP="$ROOT/docs/reference/agentops-skill-domain-map.md"
ARCH="$ROOT/docs/reference/agentops-hexagonal-architecture-map.md"
PLAN="$ROOT/docs/reference/agentops-domain-evolution-plan.md"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

for file in "$BDD" "$MAP" "$ARCH" "$PLAN"; do
  [[ -f "$file" ]] || fail "missing required artifact: ${file#"$ROOT"/}"
done

for token in Feature Scenario Given When Then; do
  grep -q "$token" "$BDD" || fail "BDD artifact missing Gherkin token: $token"
done

for context in "BC1 Corpus" "BC2 Validation" "BC3 Loop" "BC4 Factory" "BC5 Runtime"; do
  grep -q "$context" "$MAP" || fail "domain map missing context: $context"
  grep -q "$context" "$ARCH" || fail "architecture map missing context: $context"
done

for port in HypothesisLedgerPort ConvergenceCheckPort CorpusReaderPort GateRunnerPort HarnessPort; do
  grep -q "$port" "$ARCH" || fail "architecture map missing port: $port"
done

for phrase in "context compiler" "SDLC control plane" "small provable changes"; do
  grep -qi "$phrase" "$BDD" "$MAP" "$ARCH" "$PLAN" || fail "missing product framing phrase: $phrase"
done

for phrase in "ao evolve" "landing-policy off" "source-built CLI"; do
  grep -qi "$phrase" "$BDD" "$ARCH" "$PLAN" || fail "missing CLI orchestration phrase: $phrase"
done

for disposition in keep update refactor merge-review cut-review; do
  grep -q "$disposition" "$MAP" || fail "domain map missing disposition: $disposition"
done

skill_count=0
while IFS= read -r skill_md; do
  skill="$(basename "$(dirname "$skill_md")")"
  skill_count=$((skill_count + 1))
  grep -Fq "| \`$skill\` |" "$MAP" || fail "domain map missing skill: $skill"
done < <(find "$ROOT/skills" -mindepth 2 -maxdepth 2 -name SKILL.md -print | sort)

[[ "$skill_count" -gt 0 ]] || fail "no skills found"
grep -q "Skills audited | $skill_count" "$MAP" || fail "audit summary does not match skill count $skill_count"
grep -q "soc-y5vh" "$PLAN" || fail "evolution plan missing soc-y5vh source"

printf 'PASS: domain evolution plan covers %s skills\n' "$skill_count"
