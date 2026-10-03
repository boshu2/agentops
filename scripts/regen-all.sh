#!/usr/bin/env bash
# Regenerate or check every metadata-owned projection in dependency order.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mode=regen
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) mode=check ;;
    *) echo "usage: $0 [--check]" >&2; exit 2 ;;
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

if [[ "$mode" == regen ]]; then
  echo "== regenerate metadata-owned projections =="
  step "skill mesh" python3 scripts/generate-skill-mesh.py
  step "CLI reference" bash scripts/generate-cli-reference.sh
  step "command heading projections" bash scripts/regen-command-surfaces.sh
  step "CLI surface inventory" bash scripts/check-cmdao-surface-parity.sh --write-surface
  step "documentation index" python3 scripts/generate-documentation-index.py
  echo
  [[ $fail -eq 0 ]] && echo "Regeneration complete. Review the diff and run scripts/regen-all.sh --check." || echo "Regeneration failed."
else
  echo "== check metadata-owned projections =="
  step "skill mesh" python3 scripts/generate-skill-mesh.py --check
  step "Codex skill conformance" bash scripts/validate-codex-api-conformance.sh
  step "CLI reference" bash scripts/generate-cli-reference.sh --check
  step "command heading projections" bash scripts/regen-command-surfaces.sh --check
  step "CLI surface inventory" bash scripts/check-cmdao-surface-parity.sh
  step "documentation index" python3 scripts/generate-documentation-index.py --check
  step "documentation release checks" bash tests/docs/validate-doc-release.sh
  echo
  [[ $fail -eq 0 ]] && echo "All generated projections are current." || echo "Projection drift or validation failure detected."
fi

exit "$fail"
