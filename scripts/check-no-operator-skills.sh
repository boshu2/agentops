#!/bin/bash
#
# check-no-operator-skills.sh — keep operator/personal-identity skills out of
# the published product catalog (bead age-focus-membrane-bookkeeper-m1wg.11).
#
# WHY: AgentOps' published surface (registry.json, docs/SKILLS.md, the mkdocs
# site) is generated from skills/**/SKILL.md. The operator's PERSONAL-IDENTITY
# skills — athena (Bo's AI partner), wealth-mentor, and the bo-* brand/voice
# family — live in the operator's own ~/.claude/skills and must NEVER be checked
# into this repo, where they would leak into the public catalog. Today the
# separation holds structurally (none are in-repo); this guard makes "gated out"
# durable by failing fast if one is ever added.
#
# WHAT it checks (fail-closed on any hit):
#   1. No skills/<denylisted-slug>/ directory exists.
#   2. No skills-codex/<denylisted-slug>/ twin exists.
#   3. The denylisted slug is not referenced as a skill in the published
#      catalog surfaces (docs/SKILLS.md, registry.json).
#
# SCOPE: only unambiguous operator-personal-IDENTITY slugs are denied. General
# craft skills (de-slopify, teacher-mode, etc.) are product skills and are NOT
# on this list. Substrate skills (ntm, using-atm, vibing-with-ntm, swarm) are
# legitimate product skills, not operator-personal — also NOT denied.
#
# Usage:
#   check-no-operator-skills.sh            # audit the repo
#   check-no-operator-skills.sh -q         # quiet, exit code only
#   check-no-operator-skills.sh --self-test
#
# Exit codes:
#   0 = clean (no operator/personal-identity skill in the published surface)
#   1 = at least one operator/personal-identity skill leaked
#   2 = script error (bad invocation, missing repo root)

set -euo pipefail

# Operator/personal-IDENTITY skills that must never enter the product catalog.
DENYLIST=(
  athena
  wealth-mentor
  bo-voice
  bo-brand
  brand-story
  on-brand
  jargon-translator
)

QUIET=0
SELF_TEST=0
ROOT=""

for arg in "$@"; do
  case "$arg" in
    -q|--quiet) QUIET=1 ;;
    --self-test) SELF_TEST=1 ;;
    *) ROOT="$arg" ;;
  esac
done

log() { [ "$QUIET" -eq 1 ] || printf '%s\n' "$*"; }

# Resolve repo root: explicit arg, else git toplevel, else script's parent dir.
resolve_root() {
  if [ -n "$ROOT" ]; then printf '%s' "$ROOT"; return; fi
  if git rev-parse --show-toplevel >/dev/null 2>&1; then
    git rev-parse --show-toplevel; return
  fi
  cd "$(dirname "$0")/.." && pwd
}

run_audit() {
  local root="$1"
  local hits=0 slug

  for slug in "${DENYLIST[@]}"; do
    if [ -d "$root/skills/$slug" ]; then
      log "LEAK: skills/$slug/ is an operator/personal-identity skill — must not be in the product catalog"
      hits=$((hits + 1))
    fi
    if [ -d "$root/skills-codex/$slug" ]; then
      log "LEAK: skills-codex/$slug/ — operator/personal-identity twin must not ship"
      hits=$((hits + 1))
    fi
    # Published narrative + generated registry: deny a markdown/skill reference
    # to the slug as a skill (e.g. "### /athena" or a registry "name": "athena").
    if [ -f "$root/docs/SKILLS.md" ] && grep -Eq "(^|[^A-Za-z0-9_-])/?$slug([^A-Za-z0-9_-]|$)" "$root/docs/SKILLS.md"; then
      log "LEAK: docs/SKILLS.md references operator/personal-identity skill '$slug'"
      hits=$((hits + 1))
    fi
    if [ -f "$root/registry.json" ] && grep -Eq "\"(name|id)\"[[:space:]]*:[[:space:]]*\"$slug\"" "$root/registry.json"; then
      log "LEAK: registry.json publishes operator/personal-identity skill '$slug'"
      hits=$((hits + 1))
    fi
  done

  if [ "$hits" -gt 0 ]; then
    log "FAIL: $hits operator/personal-identity skill leak(s) into the published catalog."
    log "Fix: operator-personal skills belong in ~/.claude/skills, never in this repo's skills/."
    return 1
  fi
  log "OK: published catalog presents product skills only (no operator/personal-identity leakage)."
  return 0
}

self_test() {
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  # Clean fixture passes.
  mkdir -p "$tmp/skills/research"
  if ! run_audit "$tmp" >/dev/null 2>&1; then
    echo "self-test FAIL: clean fixture should pass"; return 1
  fi
  # Leaking fixture fails.
  mkdir -p "$tmp/skills/athena"
  if run_audit "$tmp" >/dev/null 2>&1; then
    echo "self-test FAIL: athena fixture should fail"; return 1
  fi
  echo "self-test OK"
  return 0
}

main() {
  if [ "$SELF_TEST" -eq 1 ]; then
    self_test
    exit $?
  fi
  local root
  root="$(resolve_root)"
  if [ ! -d "$root/skills" ]; then
    echo "error: no skills/ under repo root '$root'" >&2
    exit 2
  fi
  run_audit "$root"
}

main
