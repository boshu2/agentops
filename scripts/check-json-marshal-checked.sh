#!/usr/bin/env bash
set -euo pipefail

# check-json-marshal-checked.sh — operationalizes planning-rule
# f-2026-04-29-002 (swallowed json.Marshal anti-pattern) for bead
# agentops-tqc.3.
#
# WHY THIS EXISTS (and why golangci-lint's errcheck is NOT enough):
#   The swallow we care about is `b, _ := json.Marshal(...)` /
#   `_ = json.Unmarshal(...)` — the error is discarded via the blank
#   identifier. errcheck only flags blank-discards when run with `-blank`,
#   and golangci-lint's errcheck has check-blank OFF by default. Turning
#   check-blank ON globally in .golangci.yml would flag EVERY `_, _ :=`
#   discard repo-wide (an unrelated backlog the bead explicitly forbids
#   ballooning). errcheck has no per-function blank scoping, so the
#   correctly-scoped enforcement is this dedicated check: run errcheck
#   -blank, then keep ONLY findings on the conventional `json.` selector
#   (Marshal / MarshalIndent / Unmarshal) call sites.
#
# SCOPE — TEXTUAL, NOT SEMANTIC: this is a text-level guard. It keys on the
#   conventional `json.Marshal` / `json.MarshalIndent` / `json.Unmarshal`
#   selector spelling and does NOT resolve import aliases — a file that does
#   `import j "encoding/json"` and calls `j.Marshal` would NOT be matched.
#   Full AST/type resolution is out of scope; the convention in this repo is
#   the plain `json.` selector, which this catches.
#
# JUSTIFIED EXCEPTIONS: a discarded json error is exempted ONLY when the site
#   carries a `//nolint:errcheck` directive FOLLOWED BY a non-empty reason on
#   the same line (Go convention: `//nolint:errcheck // <reason>`). A BARE
#   `//nolint:errcheck` with no reason is NOT accepted and still trips the
#   guard — silencing the check requires an actual justification.
#
# Exit: 0 = clean, 1 = unjustified json swallow found, 2 = tooling error.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI_DIR="$REPO_ROOT/cli"

if ! command -v go >/dev/null 2>&1; then
  echo "check-json-marshal-checked: go not installed" >&2
  exit 2
fi

# Bootstrap errcheck into a cache dir if not already on PATH.
ERRCHECK_BIN="$(command -v errcheck 2>/dev/null || true)"
if [[ -z "$ERRCHECK_BIN" ]]; then
  gobin="$(go env GOPATH)/bin"
  if [[ -x "$gobin/errcheck" ]]; then
    ERRCHECK_BIN="$gobin/errcheck"
  else
    echo "check-json-marshal-checked: installing errcheck..." >&2
    GOBIN="$gobin" go install github.com/kisielk/errcheck@v1.20.0 >&2
    ERRCHECK_BIN="$gobin/errcheck"
  fi
fi

cd "$CLI_DIR"

# errcheck -blank surfaces blank-identifier discards. Filter to encoding/json
# marshal/unmarshal sites only (the bead's scope). -ignoretests keeps test
# fixtures (which legitimately discard) out of scope, matching .golangci.yml.
#
# Fail-closed: errcheck exits 0 (no findings) or 1 (findings present) under
# normal operation; ANY other exit code is a tooling/package-load failure and
# must NOT be allowed to read as a clean pass. Capture stderr and the exit code
# explicitly instead of swallowing both behind `|| true`.
errcheck_stderr="$(mktemp)"
trap 'rm -f "$errcheck_stderr"' EXIT
set +e
errcheck_out="$("$ERRCHECK_BIN" -blank -ignoretests ./... 2>"$errcheck_stderr")"
errcheck_rc=$?
set -e
if [[ "$errcheck_rc" -ne 0 && "$errcheck_rc" -ne 1 ]]; then
  echo "check-json-marshal-checked: errcheck failed (exit $errcheck_rc) — tooling error, not a pass:" >&2
  cat "$errcheck_stderr" >&2
  exit 2
fi

# exit 0 = no findings; exit 1 = findings present. In both cases filter the
# findings (if any) to encoding/json marshal/unmarshal call sites.
raw="$(printf '%s\n' "$errcheck_out" \
  | grep -E 'json\.(Marshal|MarshalIndent|Unmarshal)\b' || true)"

# Drop sites that carry a JUSTIFIED //nolint:errcheck directive — i.e. the
# directive MUST be followed by a non-empty reason (Go convention:
# `//nolint:errcheck // <reason>`). A bare `//nolint:errcheck` with no reason is
# NOT a valid exemption and must still trip the guard, so we only strip lines
# whose directive carries trailing reason text.
violations="$(printf '%s\n' "$raw" \
  | grep -vE '//nolint:errcheck[[:space:]]+//[[:space:]]*[^[:space:]]' \
  | grep -E '[^[:space:]]' || true)"

if [[ -n "$violations" ]]; then
  echo "❌ Unchecked json.Marshal/Unmarshal return value(s) (planning-rule f-2026-04-29-002):" >&2
  printf '%s\n' "$violations" >&2
  echo "" >&2
  echo "Handle the error: propagate (return ... err), log it, or — only when" >&2
  echo "marshaling genuinely cannot fail and there is no error channel — add" >&2
  echo "an explicit '//nolint:errcheck // <why safe>' justification." >&2
  exit 1
fi

echo "✅ json.Marshal/MarshalIndent/Unmarshal returns are all checked or justified"
exit 0
