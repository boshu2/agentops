#!/usr/bin/env bash
set -euo pipefail

# check-go-lint.sh — release gate for the `go.lint` check (age-gate-the-ungated-egwt.7).
#
# WHY THIS EXISTS:
#   .claude/rules/go.md documents hard lint budgets (gocyclo fail at 25, errcheck
#   mandatory, staticcheck, copyloopvar) and `cd cli && make lint` enforces them
#   locally — but `make lint` was NOT wired into the release gate, so 13 findings
#   drifted onto green main. This script makes the class impossible: it runs the
#   SAME repo-pinned linter path the Makefile's `lint` target uses
#   (scripts/golangci-lint-v2.sh run ./..., config cli/.golangci.yml) and fails
#   the push on ANY finding.
#
# FAIL-CLOSED CONTRACT (explicitly banned: skip-if-absent):
#   If the pinned linter cannot be resolved or bootstrapped, this MUST exit
#   non-zero with the exact install command — a missing linter is NOT a pass. The
#   only clean exit (0) is a genuinely lint-clean tree.
#
# EXIT CODES:
#   0 - lint clean (0 findings)
#   1 - findings present (names file:line + class) OR linter unresolvable/tooling error

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI_DIR="$REPO_ROOT/cli"
LINTER="$REPO_ROOT/scripts/golangci-lint-v2.sh"

# Install command surfaced on any fail-closed path so the operator can recover.
INSTALL_CMD="cd cli && make lint   # bootstraps the repo-pinned golangci-lint (scripts/golangci-lint-v2.sh)"

fail_closed() {
  # $1 = human-readable reason
  echo "❌ check-go-lint: $1" >&2
  echo "   The pinned linter must be resolvable for this gate to pass (skip-if-absent is banned)." >&2
  echo "   Install / bootstrap it with:" >&2
  echo "       $INSTALL_CMD" >&2
  exit 1
}

if [[ ! -x "$LINTER" ]]; then
  fail_closed "pinned linter wrapper not found or not executable at $LINTER"
fi

# The v2 wrapper bootstraps golangci-lint via `go install` when absent; without
# go it cannot resolve the linter at all — that is a fail-closed condition.
if ! command -v go >/dev/null 2>&1; then
  fail_closed "go toolchain not installed — cannot resolve the pinned linter"
fi

cd "$CLI_DIR"

# Run the SAME invocation the Makefile's `lint` target uses. Capture stdout
# (findings, file:line + (class)) and the exit code explicitly; NEVER swallow a
# non-{0,1} exit behind `|| true` (that would read a tooling failure as a pass).
lint_stderr="$(mktemp)"
trap 'rm -f "$lint_stderr"' EXIT
set +e
lint_out="$("$LINTER" run ./... 2>"$lint_stderr")"
lint_rc=$?
set -e

# golangci-lint exits 0 (clean), 1 (findings). Anything else — incl. the v2
# wrapper's 127 when it cannot bootstrap — is a tooling/resolution failure and
# must fail closed, never pass.
if [[ "$lint_rc" -ne 0 && "$lint_rc" -ne 1 ]]; then
  fail_closed "linter exited $lint_rc (tooling/resolution failure, not a clean tree): $(tr '\n' ' ' <"$lint_stderr")"
fi

if [[ "$lint_rc" -eq 1 ]]; then
  echo "❌ check-go-lint: golangci-lint reported findings (budgets in .claude/rules/go.md):" >&2
  # Emit the raw findings, which already carry file:line + (class), plus the
  # linter's own trailing "N issues" summary.
  printf '%s\n' "$lint_out" >&2
  [[ -s "$lint_stderr" ]] && cat "$lint_stderr" >&2
  echo "" >&2
  echo "   Fix or split — see $INSTALL_CMD" >&2
  exit 1
fi

echo "✅ check-go-lint: golangci-lint clean (0 findings) via $LINTER"
exit 0
