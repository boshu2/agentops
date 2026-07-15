#!/usr/bin/env bash
# Regenerate or check every metadata-owned projection in dependency order.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mode=regen
skills="${REGEN_SKILLS:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) mode=check ;;
    --skills) shift; [[ $# -gt 0 ]] || { echo "--skills requires a value" >&2; exit 2; }; skills="$1" ;;
    --skills=*) skills="${1#--skills=}" ;;
    *) echo "usage: $0 [--check] [--skills <skill[,skill,...]>]" >&2; exit 2 ;;
  esac
  shift
done

fail=0
log="$(mktemp "${TMPDIR:-/tmp}/regen-all.XXXXXX")"
trap 'rm -f "$log"' EXIT

step() {
  local label="$1"
  shift
  if "$@" >"$log" 2>&1; then
    printf '  ✓ %s\n' "$label"
  else
    printf '  ✗ %s\n' "$label"
    tail -n 12 "$log" | sed 's/^/      /'
    fail=1
  fi
}

codex_sync() {
  local args=()
  [[ "$mode" == check ]] && args+=(--check)
  [[ -n "$skills" ]] && args+=(--only "$skills")
  bash scripts/codex-sync.sh "${args[@]}"
}

codex_hashes() {
  local args=()
  [[ "$mode" == check ]] && args+=(--check)
  [[ -n "$skills" ]] && args+=(--only "$skills")
  bash scripts/regen-codex-hashes.sh "${args[@]}"
}

if [[ "$mode" == regen ]]; then
  echo "== regenerate metadata-owned projections =="
  step "Codex twins" codex_sync
  step "Codex hashes" codex_hashes
  step "skill mesh" python3 scripts/generate-skill-mesh.py
  step "embedded runtime files" make -C cli sync-hooks
  step "CLI reference" bash scripts/generate-cli-reference.sh
  step "command heading projections" bash scripts/regen-command-surfaces.sh
  step "CLI surface inventory" bash scripts/check-cmdao-surface-parity.sh --write-surface
  step "documentation index" python3 scripts/generate-documentation-index.py
  echo
  [[ $fail -eq 0 ]] && echo "Regeneration complete. Review the diff and run scripts/regen-all.sh --check." || echo "Regeneration failed."
else
  echo "== check metadata-owned projections =="
  step "Codex twins" codex_sync
  step "Codex hashes" codex_hashes
  step "skill mesh" python3 scripts/generate-skill-mesh.py --check
  step "Codex parity" bash scripts/audit-codex-parity.sh
  step "Codex runtime sections" bash scripts/validate-codex-runtime-sections.sh
  step "embedded runtime files" bash scripts/validate-embedded-sync.sh
  step "CLI reference" bash scripts/generate-cli-reference.sh --check
  step "command heading projections" bash scripts/regen-command-surfaces.sh --check
  step "CLI surface inventory" bash scripts/check-cmdao-surface-parity.sh
  step "documentation index" python3 scripts/generate-documentation-index.py --check
  step "documentation release checks" bash tests/docs/validate-doc-release.sh
  echo
  [[ $fail -eq 0 ]] && echo "All generated projections are current." || echo "Projection drift or validation failure detected."
fi

exit "$fail"
