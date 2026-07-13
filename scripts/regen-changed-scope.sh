#!/usr/bin/env bash
# regen-changed-scope.sh - changed-scope derived-artifact repair.
#
# This is the narrow default for ordinary slices. It maps changed files to the
# smallest known generator/checker set and leaves release-wide regeneration to
# scripts/regen-all.sh.
#
# Usage:
#   bash scripts/regen-changed-scope.sh
#   bash scripts/regen-changed-scope.sh --check --scope head
#   bash scripts/regen-changed-scope.sh --list --file skills/discovery/SKILL.md
#   bash scripts/regen-changed-scope.sh --files skills/foo/SKILL.md,docs/contracts/x.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

MODE="regen"
SCOPE="worktree"
LIST_ONLY=false
DRY_RUN=false
FILES=()

usage() {
  sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
}

add_file_arg() {
  local raw="$1"
  local part
  local old_ifs="$IFS"
  IFS=,
  for part in $raw; do
    [[ -n "$part" ]] && FILES+=("$part")
  done
  IFS="$old_ifs"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) MODE="check" ;;
    --scope)
      shift
      [[ $# -gt 0 ]] || { echo "--scope requires a value" >&2; exit 2; }
      SCOPE="$1"
      ;;
    --scope=*) SCOPE="${1#--scope=}" ;;
    --file)
      shift
      [[ $# -gt 0 ]] || { echo "--file requires a path" >&2; exit 2; }
      FILES+=("$1")
      ;;
    --files)
      shift
      [[ $# -gt 0 ]] || { echo "--files requires a comma-separated path list" >&2; exit 2; }
      add_file_arg "$1"
      ;;
    --files=*) add_file_arg "${1#--files=}" ;;
    --list|--dry-run) LIST_ONLY=true; DRY_RUN=true ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$SCOPE" in
  head|staged|worktree|upstream|auto) ;;
  *) echo "Invalid --scope: $SCOPE" >&2; exit 2 ;;
esac

collect_changed_files() {
  case "$SCOPE" in
    head)
      git diff-tree --no-commit-id --name-only -r HEAD
      ;;
    staged)
      git diff --cached --name-only
      ;;
    worktree)
      git diff --name-only
      git ls-files --others --exclude-standard
      ;;
    upstream)
      local upstream_ref base
      upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
      if [[ -n "$upstream_ref" ]]; then
        base="$(git merge-base HEAD "$upstream_ref")"
        git diff --name-only "$base"...HEAD
      else
        git diff-tree --no-commit-id --name-only -r HEAD
      fi
      ;;
    auto)
      if [[ -n "$(git diff --cached --name-only)" ]]; then
        git diff --cached --name-only
      elif [[ -n "$(git diff --name-only)" ]]; then
        git diff --name-only
        git ls-files --others --exclude-standard
      else
        git diff-tree --no-commit-id --name-only -r HEAD
      fi
      ;;
  esac
}

if [[ "${#FILES[@]}" -eq 0 ]]; then
  while IFS= read -r file; do
    [[ -n "$file" ]] && FILES+=("$file")
  done < <(collect_changed_files | sort -u)
fi

if [[ "${#FILES[@]}" -eq 0 ]]; then
  echo "changed-scope: no changed files; release-wide regen-all not needed"
  exit 0
fi

NEED_CODEX=false
NEED_CODEX_ALL=false
NEED_CONTEXT_MAP=false
NEED_SKILL_DOMAIN=false
NEED_REGISTRY=false
NEED_CLI_REFERENCE=false
NEED_COMMAND_SURFACES=false
NEED_CONTRACT_COMPAT=false
CODEX_SKILLS=()
SOURCE_SKILLS=()
STEPS=()

add_unique_codex_skill() {
  local skill="$1"
  local existing
  [[ -n "$skill" ]] || return 0
  for existing in "${CODEX_SKILLS[@]}"; do
    [[ "$existing" == "$skill" ]] && return 0
  done
  CODEX_SKILLS+=("$skill")
}

add_unique_source_skill() {
  local skill="$1"
  local existing skill_md="skills/$1/SKILL.md"
  [[ -n "$skill" ]] || return 0
  # Redirect-only runtime packages are compatibility aliases, not independent
  # implementations. They still route Codex/registry/context projections below,
  # but the deep implementation audit would manufacture false output-contract,
  # rubric, and trigger failures for their intentionally tiny pointer bodies.
  if [[ -f "$skill_md" ]] \
    && grep -Eq '^implementation:[[:space:]]+false([[:space:]]|$)' "$skill_md"; then
    return 0
  fi
  for existing in "${SOURCE_SKILLS[@]}"; do
    [[ "$existing" == "$skill" ]] && return 0
  done
  SOURCE_SKILLS+=("$skill")
}

skill_from_path() {
  local path="$1"
  local rest
  case "$path" in
    skills/*/*)
      rest="${path#skills/}"
      printf '%s\n' "${rest%%/*}"
      ;;
    skills-codex/*/*)
      rest="${path#skills-codex/}"
      printf '%s\n' "${rest%%/*}"
      ;;
    skills-codex-overrides/*/*)
      rest="${path#skills-codex-overrides/}"
      printf '%s\n' "${rest%%/*}"
      ;;
  esac
}

for file in "${FILES[@]}"; do
  case "$file" in
    skills/*)
      NEED_CODEX=true
      source_skill="$(skill_from_path "$file")"
      add_unique_codex_skill "$source_skill"
      [[ -f "skills/$source_skill/SKILL.md" ]] && add_unique_source_skill "$source_skill"
      NEED_SKILL_DOMAIN=true
      NEED_REGISTRY=true
      [[ "$file" == skills/*/SKILL.md ]] && NEED_CONTEXT_MAP=true
      ;;
    skills-codex/*)
      NEED_CODEX=true
      add_unique_codex_skill "$(skill_from_path "$file")"
      ;;
    skills-codex-overrides/*)
      NEED_CODEX=true
      NEED_CODEX_ALL=true
      add_unique_codex_skill "$(skill_from_path "$file")"
      ;;
    docs/contracts/context-map.md)
      NEED_CONTEXT_MAP=true
      ;;
    docs/contracts/bounded-contexts.yaml|docs/contracts/skill-dispositions.yaml)
      NEED_SKILL_DOMAIN=true
      NEED_CONTRACT_COMPAT=true
      ;;
    docs/contracts/*)
      NEED_CONTRACT_COMPAT=true
      ;;
    docs/reference/agentops-skill-domain-map.md)
      NEED_SKILL_DOMAIN=true
      ;;
    registry.json)
      NEED_REGISTRY=true
      ;;
    cli/cmd/ao/*)
      NEED_CLI_REFERENCE=true
      NEED_COMMAND_SURFACES=true
      NEED_REGISTRY=true
      ;;
    cli/docs/COMMANDS.md)
      NEED_CLI_REFERENCE=true
      NEED_COMMAND_SURFACES=true
      ;;
    docs/cli-surface.json|docs/cli-surface.md|evals/agentops-core/cli-command-surface-matrix.json|evals/agentops-core/fixtures/cli-command-surface-smoke.sh)
      NEED_COMMAND_SURFACES=true
      ;;
  esac
done

join_codex_skills() {
  local joined=""
  local skill
  for skill in "${CODEX_SKILLS[@]}"; do
    if [[ -z "$joined" ]]; then
      joined="$skill"
    else
      joined="$joined,$skill"
    fi
  done
  printf '%s\n' "$joined"
}

add_step() {
  STEPS+=("$1")
}

if [[ "${#SOURCE_SKILLS[@]}" -gt 0 ]]; then
  audit_cmd=""
  for source_skill in "${SOURCE_SKILLS[@]}"; do
    printf -v source_target '%q' "skills/$source_skill"
    if [[ -n "$audit_cmd" ]]; then
      audit_cmd+=" && "
    fi
    audit_cmd+="bash skills/heal-skill/scripts/audit.sh --strict $source_target"
  done
  add_step "changed skill deep conformance|$audit_cmd|$audit_cmd"
fi

if $NEED_CONTEXT_MAP; then
  if [[ "$MODE" == "check" ]]; then
    add_step "context-map drift|bash scripts/validate-context-map-drift.sh|bash scripts/generate-context-map.sh"
  else
    add_step "context-map|bash scripts/generate-context-map.sh|"
  fi
fi

if $NEED_SKILL_DOMAIN; then
  if [[ "$MODE" == "check" ]]; then
    add_step "skill-domain map drift|bash scripts/generate-skill-domain-map.sh --check|bash scripts/generate-skill-domain-map.sh"
  else
    add_step "skill-domain map|bash scripts/generate-skill-domain-map.sh|"
  fi
fi

if $NEED_REGISTRY; then
  if [[ "$MODE" == "check" ]]; then
    add_step "registry drift|bash scripts/generate-registry.sh --check|bash scripts/generate-registry.sh"
  else
    add_step "registry|bash scripts/generate-registry.sh|"
  fi
fi

if $NEED_CLI_REFERENCE; then
  if [[ "$MODE" == "check" ]]; then
    add_step "cli reference drift|bash scripts/generate-cli-reference.sh --check|bash scripts/generate-cli-reference.sh"
  else
    add_step "cli reference|bash scripts/generate-cli-reference.sh|"
  fi
fi

if $NEED_COMMAND_SURFACES; then
  if [[ "$MODE" == "check" ]]; then
    add_step "command surfaces drift|bash scripts/regen-command-surfaces.sh --check && bash scripts/check-cmdao-surface-parity.sh|bash scripts/generate-cli-reference.sh && bash scripts/regen-command-surfaces.sh && bash scripts/check-cmdao-surface-parity.sh --write-surface"
  else
    add_step "command surfaces|bash scripts/regen-command-surfaces.sh && bash scripts/check-cmdao-surface-parity.sh --write-surface|"
  fi
fi

if $NEED_CODEX; then
  codex_only="$(join_codex_skills)"
  codex_arg=""
  if [[ -n "$codex_only" && "$NEED_CODEX_ALL" == false ]]; then
    codex_arg=" --only $codex_only"
  fi
  if [[ "$MODE" == "check" ]]; then
    add_step "codex artifact drift|bash scripts/codex-sync.sh --check$codex_arg && bash scripts/regen-codex-hashes.sh --check$codex_arg && bash scripts/validate-codex-generated-artifacts.sh --scope $SCOPE && bash scripts/audit-codex-parity.sh|bash scripts/codex-sync.sh$codex_arg && bash scripts/regen-codex-hashes.sh$codex_arg && bash scripts/validate-codex-generated-artifacts.sh --scope $SCOPE"
  else
    add_step "codex artifacts|bash scripts/codex-sync.sh$codex_arg && bash scripts/regen-codex-hashes.sh$codex_arg && bash scripts/validate-codex-generated-artifacts.sh --scope $SCOPE|"
  fi
fi

if $NEED_CONTRACT_COMPAT; then
  if [[ "$MODE" == "check" ]]; then
    add_step "contract indexes and structural floor|bash scripts/check-contracts-structural-floor.sh && bash scripts/check-contract-compatibility.sh|update docs/contracts/index.md + docs/documentation-index.md, then rerun this changed-scope check"
  else
    add_step "contract indexes and structural floor|bash scripts/check-contracts-structural-floor.sh && bash scripts/check-contract-compatibility.sh|"
  fi
fi

if [[ "${#STEPS[@]}" -eq 0 ]]; then
  echo "changed-scope: no derived-artifact generator selected for ${#FILES[@]} changed file(s)"
  echo "changed-scope: release-wide regen-all not needed"
  exit 0
fi

echo "== changed-scope derived artifact ${MODE} =="
echo "scope: $SCOPE"
echo "files: ${#FILES[@]}"
echo "release-wide regen-all: not selected"

fail=0
for step in "${STEPS[@]}"; do
  IFS='|' read -r label cmd repair_hint <<< "$step"
  if $LIST_ONLY; then
    echo "changed-scope: $label -> $cmd"
    [[ -n "${repair_hint:-}" ]] && echo "  repair: $repair_hint"
    continue
  fi
  echo "changed-scope: $label"
  if $DRY_RUN; then
    echo "  $cmd"
    continue
  fi
  if bash -lc "$cmd"; then
    echo "  ok"
  else
    echo "  fail"
    if [[ -n "${repair_hint:-}" ]]; then
      echo "  repair: $repair_hint"
    elif [[ "$MODE" == "check" ]]; then
      echo "  repair: run 'bash scripts/regen-changed-scope.sh --scope $SCOPE' for the changed-scope repair, not release-wide regen-all"
    fi
    fail=1
  fi
done

exit "$fail"
