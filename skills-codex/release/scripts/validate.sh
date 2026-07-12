#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
RECONCILE_VALIDATOR="$SKILL_DIR/scripts/validate-reconcile.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Backticks and command strings are exact Markdown markers, never shell execution.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- **Keep preparation reversible and publication operator-owned.** This skill may create local release artifacts, a commit, and an annotated tag, but it never pushes, publishes, or triggers CI because those actions cross the reversible local boundary.'
  '- **Bind completion to the tagged commit.** Record the tag SHA, exact-SHA CI run, and reconciliation result because a green branch run or an unverified tag does not prove the released artifact.'
  '- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED release evidence repairs and reruns automatically because ordinary rejection identifies incomplete preparation; only a real publication breaker may enter HOLD or consume the helper lane.'
  '## Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the owned release artifact or route the defect back to its producing bead, then rerun pre-flight and pawl; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** freeze tag or publication guidance when an irreversible remote action, ambiguous artifact identity, or unavailable release authority prevents safe progress, then route exactly one bounded helper consultation with the audit packet.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn local validation, exact-SHA evidence, reconciliation, and the pawl verdict.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided release evidence to the operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.'
  '**Checkpoint:** before writing, confirm the operator approved the displayed changelog and version diff; before handoff, prove the release commit, annotated tag, audit, and notes agree on the version; after push, require exact-SHA CI plus reconciliation before declaring complete.'
  '## Output Specification'
  '- **Path:** `docs/releases/YYYY-MM-DD-v<version>-audit.md`, paired with `docs/releases/YYYY-MM-DD-v<version>-notes.md` and annotated ref `refs/tags/v<version>`.'
  '- **Filename convention:** `YYYY-MM-DD-v<version>-audit.md` and `YYYY-MM-DD-v<version>-notes.md`, where `<version>` is the confirmed SemVer without a duplicate `v` prefix.'
  '- **Serialization/schema format:** Markdown audit with release heading, date, previous tag, commit count, local-CI artifact path, version bumps, pre-flight results, and remote CI verdict; the annotated Git tag targets the release commit, and the verdict records exact SHA, run id, status, and conclusion.'
  '- **Validator command:** set `VERSION="<version>"`, then run `bash scripts/validate-release-audit-artifacts.sh --mode target --target-release "$VERSION" && bash scripts/verify-release-ci.sh "v$VERSION" && ao reconcile --json | bash skills/release/scripts/validate-reconcile.sh "v$VERSION"`; command success alone is insufficient because the reconciliation JSON must name that exact expected tag, be semantically green, and contain no medium/high release finding.'
  '- **Downstream handoff:** before push, send the operator the release commit SHA, annotated tag SHA, audit/notes paths, rollback commands, and exact push/verification commands; after push, append the exact-SHA CI and reconciliation evidence before closeout.'
  '## Quality Checklist'
  '- Changelog and notes contain only changes supported by the selected git range, use the repository style, and serve their distinct contributor and feed-reader audiences.'
  '- Version files, release commit, annotated tag, audit, and notes all name the same confirmed SemVer and resolve to one release boundary.'
  '- Full local release validation and artifact checks pass before handoff; exact-tag CI and reconciliation pass after the operator pushes.'
  '- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.'
)

# `$release` is an exact prompt marker, not a shell variable.
# shellcheck disable=SC2016
CODEX_PROMPT_MARKERS=(
  '## Codex Execution Profile'
  'Keep the release boundary explicit: everything up to the tag, with validations and changelog evidence called out.'
  'Prefer deterministic command sequences and clear rollback points over narrative release notes during execution.'
  'Do not run `ao codex stop` after the release commit/tag boundary; finish Codex closeout before `$release` if those artifacts must be part of the release boundary.'
  '## Guardrails'
  'Do not blur preparation work with post-tag publishing tasks.'
)

# Backticks are literal Markdown markers.
# shellcheck disable=SC2016
CODEX_SKILL_MARKER='15. **Post-push exact-SHA CI and reconciliation verification** — after the operator pushes, require both `scripts/verify-release-ci.sh v<version>` to print `GO release-ci` and `ao reconcile --json | bash skills/release/scripts/validate-reconcile.sh "v<version>"` to pass for that exact expected tag; exact-SHA CI alone never authorizes closeout.'

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "release contract validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

validate_codex_prompt() {
  local prompt="$SKILL_DIR/prompt.md" marker
  [[ -f "$prompt" ]] || return 0
  for marker in "${CODEX_PROMPT_MARKERS[@]}"; do
    grep -Fq -- "$marker" "$prompt" || {
      echo "release Codex prompt missing operator marker: $marker" >&2
      return 1
    }
  done
}

validate_exact_tag_step() {
  grep -Fqx -- "$CODEX_SKILL_MARKER" "$SKILL" || {
    echo "release skill permits CI-only or wrong-tag closeout" >&2
    return 1
  }
}

reconcile_fixtures() {
  local release='"release":{"available":true,"tag_name":"v3.2.0","tag_validate_runs":[{"status":"completed","conclusion":"success"}]}'
  local valid="{\"schema_version\":\"ao.reconcile.v1\",\"overall_status\":\"green\",$release,\"findings\":[]}"
  local warning="{\"schema_version\":\"ao.reconcile.v1\",\"overall_status\":\"green_with_warnings\",$release,\"findings\":[{\"id\":\"docs-warning\",\"severity\":\"low\",\"surface\":\"docs\"}]}"
  local complex='{"schema_version":"ao.reconcile.v1","overall_status":"green","release":{"available":true,"tag_name":"v1.2.3-rc.1+build.7","tag_validate_runs":[{"status":"completed","conclusion":"success"}]},"findings":[]}'
  local bad invalid_tag

  printf '%s\n' "$valid" | bash "$RECONCILE_VALIDATOR" "v3.2.0"
  printf '%s\n' "$warning" | bash "$RECONCILE_VALIDATOR" "v3.2.0"
  printf '%s\n' "$complex" | bash "$RECONCILE_VALIDATOR" "v1.2.3-rc.1+build.7"

  for bad in \
    '{"schema_version":"ao.reconcile.v1","overall_status":"needs_attention","release":{"available":true,"tag_name":"v3.2.0","tag_validate_runs":[{"status":"completed","conclusion":"success"}]},"findings":[{"id":"release-tag-validate-not-green","severity":"high","surface":"release"}]}' \
    '{"schema_version":"ao.reconcile.v1","overall_status":"needs_reconciliation","release":{"available":true,"tag_name":"v3.2.0","tag_validate_runs":[]},"findings":[{"id":"release-tag-validate-missing","severity":"medium","surface":"release"}]}' \
    '{"schema_version":"ao.reconcile.v1","overall_status":"green","release":{"available":true,"tag_name":"v3.2.0","tag_validate_runs":[{"status":"completed","conclusion":"success"}]},"findings":[{"id":"release-tag-validate-not-green","severity":"high","surface":"release"}]}' \
    '{"schema_version":"ao.reconcile.v1","overall_status":"green","release":{"available":true,"tag_name":"v3.2.0"},"findings":[]}' \
    '{"schema_version":"ao.reconcile.v1","overall_status":"green","release":{"available":true,"tag_name":"v3.2.0","tag_validate_runs":[{"status":"completed","conclusion":"failure"}]},"findings":[]}' \
    '{"schema_version":"ao.reconcile.v1","overall_status":"green","release":{"available":true,"tag_name":"v3.3.0","tag_validate_runs":[{"status":"completed","conclusion":"success"}]},"findings":[]}' \
    'not-json'; do
    if printf '%s\n' "$bad" | bash "$RECONCILE_VALIDATOR" "v3.2.0" >/dev/null 2>&1; then
      echo "release reconcile validator accepted a red or malformed report: $bad" >&2
      return 1
    fi
  done
  for invalid_tag in v01.2.3 v1.02.3 v1.2.03 v1.2 v1.2.3-01 1.2.3; do
    if printf '%s\n' "$valid" | bash "$RECONCILE_VALIDATOR" "$invalid_tag" >/dev/null 2>&1; then
      echo "release reconcile validator accepted invalid SemVer tag: $invalid_tag" >&2
      return 1
    fi
  done
}

[[ -f "$SKILL" ]] || { echo "FAIL: missing SKILL.md" >&2; exit 1; }
[[ -x "$RECONCILE_VALIDATOR" ]] || { echo "FAIL: missing executable reconcile validator" >&2; exit 1; }
head -1 "$SKILL" | grep -q '^---$' || { echo "FAIL: missing frontmatter" >&2; exit 1; }
validate_contract "$SKILL"
delete_one_negative_fixture
validate_codex_prompt
validate_exact_tag_step
reconcile_fixtures
echo "OK: release"
