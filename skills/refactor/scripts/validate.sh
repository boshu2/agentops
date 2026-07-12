#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
SUMMARY_VALIDATOR="$SKILL_DIR/scripts/validate-summary.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Backticks are exact Markdown markers, never shell execution.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- **Preserve behavior, not implementation.** Establish the observable contract and a green baseline before editing because a refactor that changes output, ordering, persistence, timing, or errors is feature work.'
  '- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED results repair and rerun automatically because ordinary rejection is evidence about the transformation; only a breaker may enter HOLD or consume the helper lane.'
  '## Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** revert or narrow the transformation, repair the named defect, and rerun the focused plus full checks; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** freeze edits when behavior cannot be proved or the safe boundary is ambiguous, then route exactly one bounded helper consultation with the baseline, diff, and failing evidence.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn focused tests, full tests, and the pawl verdict.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided evidence packet to the operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.'
  '**Checkpoint:** after each transformation, compare the focused-test result with the recorded baseline before committing; after the final transformation, require the full suite and before/after metrics before writing the summary.'
  '## Output Specification'
  '- **Path:** `.agents/refactor/YYYY-MM-DD-refactor-<scope>.md` in the active repository.'
  '- **Filename convention:** `YYYY-MM-DD-refactor-<scope>.md`, where `<scope>` contains only letters, digits, dots, underscores, or hyphens.'
  '- **Serialization/schema format:** Markdown with one `# Refactor:` title and exact `## Targets`, `## Metrics`, `## Transformations Applied`, `## Tests`, and `## Learnings` sections; mode, file count, metric rows, and commit-bound transformations are required, and the unique baseline/final lines each use `<N> passing, <N> failing, <N> skipped`.'
  '- **Validator command:** run `bash skills/refactor/scripts/validate-summary.sh ".agents/refactor/YYYY-MM-DD-refactor-<scope>.md"`.'
  '- **Downstream handoff:** send the validated summary path, transformation commit SHAs, focused/full test commands, and before/after metric deltas to review or closeout; a breaker packet stays in HOLD and follows the state machine above.'
  '## Quality Checklist'
  '- Each committed transformation is behavior-preserving, independently reviewable, and protected by a focused test that passed before the commit.'
  '- The final full suite is at least as green as the recorded baseline, with any ambient failures reproduced on the untouched base and explicitly excluded from the claim.'
  '- Before/after metrics show the intended structural improvement without moving complexity into an unmeasured helper or abstraction.'
  '- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.'
  '[references/behavior-preserving-simplification.md](references/behavior-preserving-simplification.md)'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  [[ "$(wc -l <"$file")" -le 250 ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fq -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fv -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "refactor contract validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

summary_fixtures() {
  local valid="$TMP/2026-07-12-refactor-parser.md" invalid="$TMP/2026-07-12-refactor-parser-bad.md" heading marker
  {
    echo '# Refactor: parser'
    echo
    echo '**Date:** 2026-07-12'
    echo '**Mode:** target'
    echo '**Files changed:** 2'
    echo
    echo '## Targets'
    echo '- parser.go:parse -- extract token validation'
    echo
    echo '## Metrics'
    echo '| Metric | Before | After | Delta |'
    echo '|---|---:|---:|---:|'
    echo '| Cyclomatic complexity | 18 | 9 | -9 |'
    echo
    echo '## Transformations Applied'
    echo '1. Extract token validation -- abcdef1'
    echo
    echo '## Tests'
    echo '- Baseline: 42 passing, 0 failing, 1 skipped'
    echo '- Final: 42 passing, 0 failing, 1 skipped'
    echo '- New tests added: 0'
    echo
    echo '## Learnings'
    echo '- Guard clauses exposed the parser contract.'
  } >"$valid"
  bash "$SUMMARY_VALIDATOR" "$valid" >/dev/null
  for heading in '## Targets' '## Metrics' '## Transformations Applied' '## Tests' '## Learnings'; do
    grep -Fvx -- "$heading" "$valid" >"$invalid"
    if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
      echo "summary validator accepted missing heading: $heading" >&2
      return 1
    fi
  done
  cp "$valid" "$TMP/not-a-summary.md"
  if bash "$SUMMARY_VALIDATOR" "$TMP/not-a-summary.md" >/dev/null 2>&1; then
    echo "summary validator accepted an invalid filename" >&2
    return 1
  fi
  for marker in '- Baseline:' '- Final:' '- New tests added:' '| Cyclomatic complexity |'; do
    grep -Fv -- "$marker" "$valid" >"$invalid"
    if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
      echo "summary validator accepted missing evidence: $marker" >&2
      return 1
    fi
  done
  sed 's/- Baseline: 42 passing, 0 failing, 1 skipped/- Baseline: malformed/' "$valid" >"$invalid"
  if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
    echo "summary validator accepted malformed baseline counts" >&2
    return 1
  fi
  sed 's/- Final: 42 passing, 0 failing, 1 skipped/- Final: not-a-count/' "$valid" >"$invalid"
  if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
    echo "summary validator accepted malformed final counts" >&2
    return 1
  fi
  sed 's/\*\*Mode:\*\* target/**Mode:** mutate/' "$valid" >"$invalid"
  if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
    echo "summary validator accepted invalid mode" >&2
    return 1
  fi
  sed 's/ -- abcdef1//' "$valid" >"$invalid"
  if bash "$SUMMARY_VALIDATOR" "$invalid" >/dev/null 2>&1; then
    echo "summary validator accepted transformation without commit SHA" >&2
    return 1
  fi
}

[[ -f "$SKILL" ]] || { echo "FAIL: missing SKILL.md" >&2; exit 1; }
[[ -f "$SKILL_DIR/references/behavior-preserving-simplification.md" ]] || { echo "FAIL: missing refactoring patterns reference" >&2; exit 1; }
[[ -x "$SUMMARY_VALIDATOR" ]] || { echo "FAIL: missing executable summary validator" >&2; exit 1; }
head -1 "$SKILL" | grep -q '^---$' || { echo "FAIL: missing frontmatter" >&2; exit 1; }
validate_contract "$SKILL"
delete_one_negative_fixture
summary_fixtures
echo "OK: refactor"
