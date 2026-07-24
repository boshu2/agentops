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
    return 1
  fi
}

publish_generated_projections() {
  local args=(
    --repository "$ROOT"
    --owner-map docs/contracts/generated-projection-owners.v1.json
  )
  if [[ "$mode" == check ]]; then
    args+=(--check)
  else
    args+=(--write)
  fi
  # Shared publication is intentionally complete even when a caller supplies
  # --skills. The selector remains available to staged generators as provenance,
  # but it may not narrow the transaction into a mixed live generation.
  REGEN_SKILLS="$skills" \
    PYTHONDONTWRITEBYTECODE=1 \
    python3 -B scripts/publish-generated-projections.py "${args[@]}"
}

if [[ "$mode" == regen ]]; then
  echo "== regenerate metadata-owned projections =="
  step "transactional generated projections" publish_generated_projections
  echo
  echo "Regeneration complete. Review the diff and run scripts/regen-all.sh --check."
else
  echo "== check metadata-owned projections =="
  step "transactional generated projections" publish_generated_projections
  step "Codex parity" bash scripts/audit-codex-parity.sh
  step "Codex runtime sections" bash scripts/validate-codex-runtime-sections.sh
  step "documentation release checks" bash tests/docs/validate-doc-release.sh
  echo
  echo "All generated projections are current."
fi
