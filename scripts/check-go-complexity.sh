#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $0 [--base <git-ref>] [--warn <n>] [--fail <n>]

Checks cyclomatic complexity for changed non-test Go files under cli/.
- Warns when complexity >= warn threshold
- Fails when complexity >= fail threshold
USAGE
}

BASE_REF=""
WARN_THRESHOLD=15
FAIL_THRESHOLD=25

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base)
      BASE_REF="$2"
      shift 2
      ;;
    --warn)
      WARN_THRESHOLD="$2"
      shift 2
      ;;
    --fail)
      FAIL_THRESHOLD="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if ! command -v gocyclo >/dev/null 2>&1; then
  echo "gocyclo not found; install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest" >&2
  exit 2
fi

if [[ -z "$BASE_REF" ]]; then
  if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    BASE_REF="HEAD~1"
  else
    echo "No base ref and no HEAD~1 available; skipping complexity check."
    exit 0
  fi
fi

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  echo "Base ref $BASE_REF not found; skipping complexity check."
  exit 0
fi

# If base resolves to HEAD (possible on main pushes with shallow history), fall back to HEAD~1.
if [[ "$(git rev-parse "$BASE_REF")" == "$(git rev-parse HEAD)" ]]; then
  if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    BASE_REF="HEAD~1"
  else
    echo "Base equals HEAD and HEAD~1 unavailable; skipping complexity check."
    exit 0
  fi
fi

mapfile -t CHANGED_FILES < <(
  git diff --name-only "$BASE_REF"...HEAD -- '*.go' \
    | grep '^cli/' \
    | grep -v '_test\.go$' \
    || true
)

if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
  echo "No changed non-test Go files under cli/."
  exit 0
fi

echo "Complexity check base: $BASE_REF"
echo "Warn threshold: $WARN_THRESHOLD"
echo "Fail threshold: $FAIL_THRESHOLD"
printf 'Changed files:\n'
printf '  - %s\n' "${CHANGED_FILES[@]}"

CURRENT_REPORT=$(gocyclo -over "$WARN_THRESHOLD" "${CHANGED_FILES[@]}" || true)

TMP_DIR="$(mktemp -d)"
CURRENT_FILE="$(mktemp)"
BASE_FILE="$(mktemp)"
trap 'rm -rf "$TMP_DIR" "$CURRENT_FILE" "$BASE_FILE"' EXIT

printf '%s\n' "$CURRENT_REPORT" | sed '/^[[:space:]]*$/d' > "$CURRENT_FILE"

BASE_VERSION_FILES=()
for file in "${CHANGED_FILES[@]}"; do
  if git cat-file -e "$BASE_REF:$file" 2>/dev/null; then
    mkdir -p "$TMP_DIR/$(dirname "$file")"
    git show "$BASE_REF:$file" > "$TMP_DIR/$file"
    BASE_VERSION_FILES+=("$TMP_DIR/$file")
  fi
done

BASE_REPORT=""
if [[ ${#BASE_VERSION_FILES[@]} -gt 0 ]]; then
  BASE_REPORT=$(gocyclo -over "$WARN_THRESHOLD" "${BASE_VERSION_FILES[@]}" || true)
  BASE_REPORT="${BASE_REPORT//"$TMP_DIR/"/}"
fi
printf '%s\n' "$BASE_REPORT" | sed '/^[[:space:]]*$/d' > "$BASE_FILE"

if [[ ! -s "$CURRENT_FILE" ]]; then
  echo "No functions exceed warning threshold."
  exit 0
fi

echo
echo "Functions over warning threshold ($WARN_THRESHOLD):"
cat "$CURRENT_FILE"

NEW_OR_WORSE_FAILS=$(
  awk -v fail="$FAIL_THRESHOLD" '
    FNR==NR {
      if (NF >= 4) {
        path=$4
        sub(/:[0-9]+:[0-9]+$/, "", path)
        key=$2 " " $3 " " path
        prev[key]=$1+0
      }
      next
    }
    NF >= 4 {
      path=$4
      sub(/:[0-9]+:[0-9]+$/, "", path)
      key=$2 " " $3 " " path
      curr=$1+0
      old=(key in prev) ? prev[key] : -1
      if (curr >= fail && (old < 0 || curr > old)) {
        print $0
      }
    }
  ' "$BASE_FILE" "$CURRENT_FILE"
)

NEW_OR_WORSE_WARNINGS=$(
  awk -v warn="$WARN_THRESHOLD" -v fail="$FAIL_THRESHOLD" '
    FNR==NR {
      if (NF >= 4) {
        path=$4
        sub(/:[0-9]+:[0-9]+$/, "", path)
        key=$2 " " $3 " " path
        prev[key]=$1+0
      }
      next
    }
    NF >= 4 {
      path=$4
      sub(/:[0-9]+:[0-9]+$/, "", path)
      key=$2 " " $3 " " path
      curr=$1+0
      old=(key in prev) ? prev[key] : -1
      if (curr >= warn && curr < fail && (old < 0 || curr > old)) {
        print $0
      }
    }
  ' "$BASE_FILE" "$CURRENT_FILE"
)

LEGACY_FAILS=$(
  awk -v fail="$FAIL_THRESHOLD" '
    FNR==NR {
      if (NF >= 4) {
        path=$4
        sub(/:[0-9]+:[0-9]+$/, "", path)
        key=$2 " " $3 " " path
        prev[key]=$1+0
      }
      next
    }
    NF >= 4 {
      path=$4
      sub(/:[0-9]+:[0-9]+$/, "", path)
      key=$2 " " $3 " " path
      curr=$1+0
      old=(key in prev) ? prev[key] : -1
      if (curr >= fail && old >= 0 && curr <= old) {
        print $0
      }
    }
  ' "$BASE_FILE" "$CURRENT_FILE"
)

if [[ -n "$NEW_OR_WORSE_WARNINGS" ]]; then
  echo
  echo "New/worsened complexity warnings:"
  echo "$NEW_OR_WORSE_WARNINGS"
fi

if [[ -n "$LEGACY_FAILS" ]]; then
  echo
  echo "Legacy high-complexity functions touched but not worsened (non-blocking):"
  echo "$LEGACY_FAILS"
fi

if [[ -n "$NEW_OR_WORSE_FAILS" ]]; then
  echo
  echo "ERROR: new/worsened functions over failure threshold ($FAIL_THRESHOLD):"
  echo "$NEW_OR_WORSE_FAILS"
  exit 1
fi

echo
echo "Complexity budget respected for new/changed complexity."
exit 0
