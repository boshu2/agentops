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

REPORT=$(gocyclo -over "$WARN_THRESHOLD" "${CHANGED_FILES[@]}" || true)

if [[ -z "$REPORT" ]]; then
  echo "No functions exceed warning threshold."
  exit 0
fi

echo
echo "Functions over warning threshold ($WARN_THRESHOLD):"
echo "$REPORT"

FAIL_REPORT=$(echo "$REPORT" | awk -v t="$FAIL_THRESHOLD" '$1 >= t')
if [[ -n "$FAIL_REPORT" ]]; then
  echo
  echo "ERROR: functions over failure threshold ($FAIL_THRESHOLD):"
  echo "$FAIL_REPORT"
  exit 1
fi

echo
echo "Complexity warnings present, but no failures."
exit 0
