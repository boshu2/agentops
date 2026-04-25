#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

if ! command -v gocyclo >/dev/null 2>&1; then
  echo "gocyclo not found; install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest" >&2
  exit 2
fi

write_fixture() {
  local dir="$1"
  mkdir -p "$dir"
  cat >"$dir/hot.go" <<'GO'
package sample

func Simple(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func Risky(x int) int {
	total := 0
	if x%2 == 0 {
		total++
	}
	if x%3 == 0 {
		total++
	}
	if x%5 == 0 {
		total++
	}
	if x%7 == 0 {
		total++
	}
	if x%11 == 0 {
		total++
	}
	if x%13 == 0 {
		total++
	}
	if x%17 == 0 {
		total++
	}
	if x%19 == 0 {
		total++
	}
	return total
}
GO
}

run_pass_case() {
  local dir="$TMP_ROOT/pass"
  write_fixture "$dir"
  GOCYCLO_NO_AUTOINSTALL=1 bash "$REPO_ROOT/scripts/check-go-absolute-complexity.sh" --dir "$dir" --threshold 20 \
    | grep -q 'All functions in'
  echo "complexity-pass-ok"
}

run_fail_case() {
  local dir="$TMP_ROOT/fail"
  write_fixture "$dir"
  local out rc
  set +e
  out="$(GOCYCLO_NO_AUTOINSTALL=1 bash "$REPO_ROOT/scripts/check-go-absolute-complexity.sh" --dir "$dir" --threshold 5 2>&1)"
  rc=$?
  set -e
  test "$rc" -eq 1
  grep -q 'ERROR: Functions exceeding complexity 5' <<<"$out"
  grep -q 'Risky' <<<"$out"
  echo "complexity-fail-ok"
}

run_per_file_case() {
  local dir="$TMP_ROOT/per-file"
  write_fixture "$dir"
  local out rc
  set +e
  out="$(GOCYCLO_NO_AUTOINSTALL=1 bash "$REPO_ROOT/scripts/check-go-absolute-complexity.sh" --dir "$dir" --threshold 5 --per-file 2>&1)"
  rc=$?
  set -e
  test "$rc" -eq 1
  grep -q 'Per-file violations (complexity > 5' <<<"$out"
  grep -q 'hot.go: 1 violation(s)' <<<"$out"
  grep -q 'Total: 1 violation(s)' <<<"$out"
  echo "complexity-per-file-ok"
}

case "${1:-all}" in
  pass) run_pass_case ;;
  fail) run_fail_case ;;
  per-file) run_per_file_case ;;
  all)
    run_pass_case
    run_fail_case
    run_per_file_case
    ;;
  *)
    echo "usage: $0 [pass|fail|per-file|all]" >&2
    exit 2
    ;;
esac
