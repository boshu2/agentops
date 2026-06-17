#!/usr/bin/env bash
# regen-claim-registry.sh — additive regen of docs/contracts/claim-registry.yaml
# from agentops:claim:* markers in tracked files.
#
# ADDITIVE ONLY: creates UNPROVEN stubs for markers with no registry entry.
# NEVER overwrites curated tier / evidence / owner / eval_binding / notes.
#
# Usage:
#   scripts/regen-claim-registry.sh              # regen in place
#   scripts/regen-claim-registry.sh --check      # exit 1 if stubs would be added
#   scripts/regen-claim-registry.sh --dry-run      # print stubs that would be added
#   scripts/regen-claim-registry.sh --root DIR     # override repo root (tests)

set -euo pipefail

CHECK=0
DRY_RUN=0
ROOT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --root) ROOT="$2"; shift 2 ;;
    -h|--help) sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "regen-claim-registry: unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$ROOT" ]]; then
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

REGISTRY="$ROOT/docs/contracts/claim-registry.yaml"

if [[ ! -f "$REGISTRY" ]] && [[ "$CHECK" -eq 0 ]] && [[ "$DRY_RUN" -eq 0 ]]; then
  echo "regen-claim-registry: registry not found: $REGISTRY" >&2
  exit 3
fi

declare -A marker_surfaces

collect_from_file() {
  local rel="$1"
  local abs="$ROOT/$rel"
  [[ -f "$abs" ]] || return 0
  while IFS= read -r line; do
    local id
    id=$(echo "$line" | grep -oE 'AOP-CLAIM-[A-Z0-9-]+' | head -1)
    [[ -n "$id" ]] || continue
    if [[ -z "${marker_surfaces[$id]+x}" ]]; then
      marker_surfaces[$id]="$rel"
    elif [[ "${marker_surfaces[$id]}" != *"$rel"* ]]; then
      marker_surfaces[$id]="${marker_surfaces[$id]},$rel"
    fi
  done < <(grep -n 'agentops:claim:AOP-CLAIM-' "$abs" 2>/dev/null || true)
}

should_skip_path() {
  local p="$1"
  [[ "$p" == .agents/* || "$p" == _beads/* ]] && return 0
  [[ "$p" == *"_test.go" ]] && return 0
  [[ "$p" == *"/testdata/"* ]] && return 0
  [[ "$p" == "docs/contracts/claim-registry.yaml" ]] && return 0
  [[ "$p" == "docs/contracts/claim-eval-promote.md" ]] && return 0
  return 1
}

if git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1; then
  while IFS= read -r rel; do
    [[ -n "$rel" ]] || continue
    should_skip_path "$rel" && continue
    case "$rel" in
      *.md|*.yaml|*.yml|*.go) collect_from_file "$rel" ;;
    esac
  done < <(git -C "$ROOT" ls-files --cached --others --exclude-standard 2>/dev/null || true)
else
  while IFS= read -r abs; do
    rel="${abs#"$ROOT"/}"
    should_skip_path "$rel" && continue
    case "$rel" in
      *.md|*.yaml|*.yml|*.go) collect_from_file "$rel" ;;
    esac
  done < <(find "$ROOT" -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' -o -name '*.go' \) 2>/dev/null || true)
fi

missing=()
if [[ -f "$REGISTRY" ]]; then
  for id in $(printf '%s\n' "${!marker_surfaces[@]}" | sort); do
    if ! grep -q "^  $id:" "$REGISTRY" 2>/dev/null; then
      missing+=("$id")
    fi
  done
else
  for id in $(printf '%s\n' "${!marker_surfaces[@]}" | sort); do
    missing+=("$id")
  done
fi

if [[ "${#missing[@]}" -eq 0 ]]; then
  echo "regen-claim-registry: OK — all ${#marker_surfaces[@]} marker(s) have registry entries"
  exit 0
fi

if [[ "$CHECK" -eq 1 ]]; then
  echo "regen-claim-registry: DRIFT — ${#missing[@]} marker(s) missing from registry:"
  for id in "${missing[@]}"; do
    echo "  $id (in: ${marker_surfaces[$id]})"
  done
  exit 1
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "# Would add ${#missing[@]} UNPROVEN stub(s):"
  for id in "${missing[@]}"; do
    IFS=',' read -ra surfs <<< "${marker_surfaces[$id]}"
    echo "  $id:"
    echo "    tier: UNPROVEN"
    echo "    surfaces:"
    for s in "${surfs[@]}"; do
      echo "      - $s"
    done
  done
  exit 0
fi

if [[ ! -f "$REGISTRY" ]]; then
  mkdir -p "$(dirname "$REGISTRY")"
  cat > "$REGISTRY" <<'HEADER'
version: 1

claims:
HEADER
fi

for id in "${missing[@]}"; do
  IFS=',' read -ra surfs <<< "${marker_surfaces[$id]}"
  {
    echo "  $id:"
    echo "    tier: UNPROVEN"
    echo "    summary: \"\""
    echo "    surfaces:"
    for s in "${surfs[@]}"; do
      echo "      - $s"
    done
    echo "    marker: \"agentops:claim:$id\""
    echo "    eval_binding: \"\""
    echo "    evidence: []"
    echo "    owner: \"\""
  } >> "$REGISTRY"
done

echo "regen-claim-registry: added ${#missing[@]} UNPROVEN stub(s) to $REGISTRY"
