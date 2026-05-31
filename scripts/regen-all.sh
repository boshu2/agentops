#!/usr/bin/env bash
# regen-all.sh — one-command finalizer for the derived-artifact gate surface.
#
# Adding a skill or `ao` command makes ~8 generated registries/goldens stale, each
# guarded by an independent CI gate that stops at the first error. Discovering them
# one CI round at a time is slow (see PR #598 / ag-nk67: 9 rounds, 0 feature-code
# failures). This composes every generator into one local pass, with a --check mode
# that runs the matching drift validators as a pre-push gate.
#
# Usage:
#   scripts/regen-all.sh                      # regenerate all derived artifacts (writes)
#   scripts/regen-all.sh --check              # run all drift validators (no writes); exit 1 on drift
#   scripts/regen-all.sh --skills foo,bar     # scope the codex-hash step to foo,bar
#   REGEN_SKILLS=foo,bar scripts/regen-all.sh # same scoping via env
#
# NOTE: Codex twins (skills-codex/<name>/{SKILL.md,prompt.md}) are MANUAL — this
# refreshes their hashes but cannot author a missing twin. Create twins by hand
# (runtime-native: no `.claude/...` tokens), then re-run.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MODE="regen"
REGEN_SKILLS="${REGEN_SKILLS:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) MODE="check" ;;
    --skills) shift; [[ $# -gt 0 ]] || { echo "--skills requires a value" >&2; exit 2; }; REGEN_SKILLS="$1" ;;
    --skills=*) REGEN_SKILLS="${1#--skills=}" ;;
    *) echo "usage: $0 [--check] [--skills <skill[,skill,...]>]" >&2; exit 2 ;;
  esac
  shift
done

fail=0
step() { # label cmd...
  local label="$1"; shift
  if "$@" >/tmp/regen-all.$$.log 2>&1; then
    printf '  \033[0;32m✓\033[0m %s\n' "$label"
  else
    printf '  \033[0;31m✗\033[0m %s\n' "$label"; sed 's/^/      /' /tmp/regen-all.$$.log | tail -6
    fail=1
  fi
}

if [[ "$MODE" == "regen" ]]; then
  echo "== regenerating derived artifacts (dependency order) =="
  step "skill counts"          bash scripts/sync-skill-counts.sh
  step "skill-domain-map"      bash scripts/generate-skill-domain-map.sh
  step "SKU catalog (registry.json)" bash scripts/generate-registry.sh
  step "context-map"           bash scripts/generate-context-map.sh
  step "embedded skills"       make -C cli sync-hooks
  step "cli reference (COMMANDS.md)" bash scripts/generate-cli-reference.sh
  step "cli-surface inventory" bash scripts/check-cmdao-surface-parity.sh --write-surface
  step "codex hashes"          bash scripts/regen-codex-hashes.sh ${REGEN_SKILLS:+--only "$REGEN_SKILLS"}
  echo
  echo "Regenerated. Review 'git status', then run: scripts/regen-all.sh --check"
  echo "Reminder: new skills also need MANUAL skills-codex/<name>/ twins +"
  echo "  the cobra expectedCmds list + cli-help-matrix golden when adding ao commands."
  echo "Scope tip: changed only some skills? Re-run with --skills a,b (or REGEN_SKILLS=a,b)"
  echo "  so the codex-hash step skips unrelated pre-existing drift instead of sweeping it in."
else
  echo "== drift / gate sweep (no writes) =="
  step "registry drift"        bash scripts/check-registry-drift.sh
  step "skill-domain-map golden" bash scripts/generate-skill-domain-map.sh --check
  step "write-surfaces"        bash scripts/check-agents-write-surfaces.sh
  step "codex parity"          bash scripts/audit-codex-parity.sh
  step "codex runtime sections" bash scripts/validate-codex-runtime-sections.sh
  step "codex hashes (no drift)" bash scripts/regen-codex-hashes.sh --check ${REGEN_SKILLS:+--only "$REGEN_SKILLS"}
  step "SKU catalog drift"     bash scripts/validate-sku-catalog-drift.sh
  step "context-map drift"     bash scripts/validate-context-map-drift.sh
  step "doc-release gate"      bash tests/docs/validate-doc-release.sh
  echo
  [[ $fail -eq 0 ]] && echo "ALL GATES GREEN — safe to push" || echo "DRIFT DETECTED — run scripts/regen-all.sh (no flag) to fix, then commit"
fi

rm -f /tmp/regen-all.$$.log
exit $fail
