#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/evolve-capture-baseline.sh --label <slug> [options]

Capture an immutable evolve baseline snapshot for the current goal era.
By default this writes:
  .agents/evolve/fitness-baselines/<label>/<timestamp>.json

Required:
  --label <slug>              Baseline label (letters, numbers, ., _, -)

Options:
  --repo-root <path>          Repo root (default: current directory)
  --output-dir <path>         Baseline root directory (default: .agents/evolve/fitness-baselines)
  --legacy-path <path>        Optional compatibility mirror path
  --active-path <path>        Optional active baseline pointer path
  --index-path <path>         Optional baseline index path
  --timeout <seconds>         ao goals measure timeout (default: 60)
  --force                     Allow another snapshot in an existing label directory
  -h, --help                  Show help
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

iso_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -Iseconds
}

snapshot_timestamp() {
  date -u +"%Y-%m-%dT%H-%M-%S.000"
}

resolve_path() {
  local base="$1"
  local path="$2"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
  else
    printf '%s/%s\n' "$base" "$path"
  fi
}

relative_to_repo() {
  local repo_root="$1"
  local path="$2"
  case "$path" in
    "$repo_root"/*)
      printf '%s\n' "${path#"$repo_root"/}"
      ;;
    *)
      printf '%s\n' "$path"
      ;;
  esac
}

LABEL=""
REPO_ROOT="$(pwd)"
OUTPUT_DIR=".agents/evolve/fitness-baselines"
LEGACY_PATH=""
ACTIVE_PATH=""
INDEX_PATH=""
TIMEOUT="60"
FORCE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --label)
      LABEL="${2:-}"
      shift 2
      ;;
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --legacy-path)
      LEGACY_PATH="${2:-}"
      shift 2
      ;;
    --active-path)
      ACTIVE_PATH="${2:-}"
      shift 2
      ;;
    --index-path)
      INDEX_PATH="${2:-}"
      shift 2
      ;;
    --timeout)
      TIMEOUT="${2:-}"
      shift 2
      ;;
    --force)
      FORCE=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

require_cmd ao
require_cmd jq

[[ -n "$LABEL" ]] || die "--label is required"
[[ "$LABEL" =~ ^[A-Za-z0-9._-]+$ ]] || die "--label must contain only letters, numbers, ., _, or -"
[[ "$TIMEOUT" =~ ^[0-9]+$ ]] || die "--timeout must be numeric"
if [[ -n "$INDEX_PATH" ]]; then
  require_cmd git
fi

OUTPUT_DIR_PATH="$(resolve_path "$REPO_ROOT" "$OUTPUT_DIR")"
ERA_DIR_PATH="${OUTPUT_DIR_PATH}/${LABEL}"
SNAPSHOT_STEM="$(snapshot_timestamp)"
OUTPUT_PATH="${ERA_DIR_PATH}/${SNAPSHOT_STEM}.json"

mkdir -p "$ERA_DIR_PATH"

if [[ -n "$LEGACY_PATH" ]]; then
  LEGACY_PATH_ABS="$(resolve_path "$REPO_ROOT" "$LEGACY_PATH")"
  mkdir -p "$(dirname "$LEGACY_PATH_ABS")"
fi
if [[ -n "$ACTIVE_PATH" ]]; then
  ACTIVE_PATH_ABS="$(resolve_path "$REPO_ROOT" "$ACTIVE_PATH")"
  mkdir -p "$(dirname "$ACTIVE_PATH_ABS")"
fi
if [[ -n "$INDEX_PATH" ]]; then
  INDEX_PATH_ABS="$(resolve_path "$REPO_ROOT" "$INDEX_PATH")"
  mkdir -p "$(dirname "$INDEX_PATH_ABS")"
fi

if [[ -d "$ERA_DIR_PATH" && "$FORCE" != true ]]; then
  EXISTING_SNAPSHOT="$(find "$ERA_DIR_PATH" -maxdepth 1 -type f -name '*.json' -print -quit)"
else
  EXISTING_SNAPSHOT=""
fi
if [[ -n "$EXISTING_SNAPSHOT" ]]; then
  die "baseline already exists for label: $LABEL"
fi

SNAPSHOT_SUFFIX=1
while [[ -f "$OUTPUT_PATH" ]]; do
  OUTPUT_PATH="${ERA_DIR_PATH}/${SNAPSHOT_STEM}-${SNAPSHOT_SUFFIX}.json"
  SNAPSHOT_SUFFIX=$((SNAPSHOT_SUFFIX + 1))
done

TMP_FILE="$(mktemp "${TMPDIR:-/tmp}/evolve-baseline.XXXXXX")"
trap 'rm -f "$TMP_FILE"' EXIT

(
  cd "$REPO_ROOT"
  ao goals measure --json --timeout "$TIMEOUT" > "$TMP_FILE"
)

jq -e '.goals | type == "array"' "$TMP_FILE" >/dev/null 2>&1 || die "baseline snapshot is not valid goals JSON"

GOALS_TOTAL="$(jq -r '.goals | length' "$TMP_FILE")"
TIMESTAMP="$(iso_timestamp)"
HEAD_SHA="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
RELATIVE_OUTPUT="$(relative_to_repo "$REPO_ROOT" "$OUTPUT_PATH")"

mv "$TMP_FILE" "$OUTPUT_PATH"
trap - EXIT
if [[ -n "$LEGACY_PATH" ]]; then
  cp "$OUTPUT_PATH" "$LEGACY_PATH_ABS"
fi
if [[ -n "$ACTIVE_PATH" ]]; then
  printf '%s\n' "$RELATIVE_OUTPUT" > "$ACTIVE_PATH_ABS"
fi

if [[ -n "$INDEX_PATH" ]]; then
  HEAD_SHA="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
  INDEX_ENTRY="$(
    jq -cn \
      --arg label "$LABEL" \
      --arg path "$RELATIVE_OUTPUT" \
      --arg captured_at "$TIMESTAMP" \
      --arg sha "$HEAD_SHA" \
      --argjson goals_total "$GOALS_TOTAL" \
      '{
        label: $label,
        path: $path,
        captured_at: $captured_at,
        sha: $sha,
        goals_total: $goals_total
      }'
  )"
  printf '%s\n' "$INDEX_ENTRY" >> "$INDEX_PATH_ABS"
fi
printf '%s\n' "$RELATIVE_OUTPUT"
