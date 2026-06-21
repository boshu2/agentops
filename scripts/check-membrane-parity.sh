#!/usr/bin/env bash
# check-membrane-parity.sh
#
# Membrane demo parity guard — epic age-cwo, bead age-membrane-memory-arch-tz2s.2.5.
#
# The e2e membrane demo (bead .2.6) must run against an `ao` binary that carries
# the CURRENT source's membrane surface: `membrane recall`, `membrane
# derive-checks`, and the EM-ENF loop behind them. A STALE installed binary
# (built before the membrane shipped) would make the demo silently exercise old
# machinery and emit a FALSE proof — exactly the drift that motivated this bead
# ("source has membrane recall; installed binary does not").
#
# This check derives the required membrane subcommand surface from a FRESH
# SOURCE BUILD and asserts the demo-path binary exposes every one. The expected
# surface is read from source, never hardcoded, so it cannot drift from the CLI.
#
# Usage:
#   bash scripts/check-membrane-parity.sh                 # check `ao` on PATH
#   AO_BIN=/path/to/ao bash scripts/check-membrane-parity.sh
#
# Environment overrides (testing / speed):
#   AO_BIN      Demo-path binary under test (default: `ao` resolved on PATH).
#   AO_SRC_BIN  Pre-built source reference; skips the `go build` when provided.
#
# Exit codes:
#   0 = demo-path binary has full membrane parity with source
#   1 = demo-path binary is stale / missing membrane surface — run `make install`
#   2 = harness error (cannot build or read the source reference)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

fail()  { echo "MEMBRANE_PARITY: $*" >&2; }
note()  { echo "MEMBRANE_PARITY: $*"; }

# ─── Source reference: a binary built from the CURRENT tree ──────────────────
SRC_BIN="${AO_SRC_BIN:-}"
TMPDIR_BUILD=""
# Invoked indirectly via `trap ... EXIT` (shellcheck can't see that → SC2329).
# MUST preserve the triggering exit code: a trap whose final command sets a
# different status overrides the script's real `exit N` (e.g. with AO_SRC_BIN
# set, TMPDIR_BUILD is empty and a bare `[[ -n "" ]]` would mask exit 0/2 as 1).
# Capture $? first, then `exit "$rc"` to force the final code — NOT a hardcoded
# 0, which would swallow genuine error exits (set -u aborts, etc.).
# shellcheck disable=SC2329
cleanup() {
  local rc=$?
  if [[ -n "$TMPDIR_BUILD" ]]; then
    rm -rf "$TMPDIR_BUILD"
  fi
  exit "$rc"
}
trap cleanup EXIT

if [[ -z "$SRC_BIN" ]]; then
  if ! command -v go >/dev/null 2>&1; then
    fail "go toolchain not found; cannot build the source reference (set AO_SRC_BIN to skip)"
    exit 2
  fi
  TMPDIR_BUILD="$(mktemp -d)"
  SRC_BIN="$TMPDIR_BUILD/ao-src"
  if ! ( cd "$REPO_ROOT/cli" && go build -o "$SRC_BIN" ./cmd/ao ) 2>"$TMPDIR_BUILD/build.err"; then
    fail "failed to build source reference from cli/cmd/ao:"
    sed 's/^/  /' "$TMPDIR_BUILD/build.err" >&2
    exit 2
  fi
fi

if [[ ! -x "$SRC_BIN" ]]; then
  fail "source reference is not executable: $SRC_BIN"
  exit 2
fi

# Required surface = the membrane subcommands the source binary actually exposes.
# Parse cobra's "Available Commands:" block (first token per line, up to the
# blank line that closes the block). Read from source, so it never drifts.
required_subs="$(
  "$SRC_BIN" membrane --help 2>/dev/null \
    | awk '/^Available Commands:/{f=1;next} f&&/^[[:space:]]*$/{f=0} f&&NF{print $1}'
)"

if [[ -z "$required_subs" ]]; then
  fail "source reference exposes no 'membrane' subcommands — membrane surface missing from source?"
  exit 2
fi

# ─── Demo-path binary under test ─────────────────────────────────────────────
AO_BIN="${AO_BIN:-$(command -v ao || true)}"
if [[ -z "$AO_BIN" ]]; then
  fail "no demo-path 'ao' found on PATH (and AO_BIN unset)."
  fail "remediation: build + install from source — \`cd cli && make install\`."
  exit 1
fi
if [[ ! -x "$AO_BIN" ]]; then
  fail "demo-path binary is not executable: $AO_BIN"
  exit 1
fi

# local_flags <binary> <sub> — the long-flag NAMES declared in a `membrane <sub>`
# command's own "Flags:" block (NOT "Global Flags:"), sorted + unique. Read from
# `--help` so it tracks whatever the source actually exposes.
#
# Parse cobra's flag COLUMN structurally: take only the FIRST flag token of each
# flag line — `      --run string  desc` or `  -h, --help  desc` — via sed. A
# bare `grep -oE -- '--x'` over the block would also harvest flag-looking tokens
# from DESCRIPTION prose (e.g. "pass --run to newer builds"), miscounting a flag
# that is merely mentioned as one the binary declares — a stale false-pass.
local_flags() {
  "$1" membrane "$2" --help 2>/dev/null \
    | awk '/^Flags:/{f=1;next} /^Global Flags:/{f=0} f&&/^[[:space:]]*$/{f=0} f' \
    | sed -nE 's/^[[:space:]]*(-[A-Za-z],[[:space:]]+)?(--[A-Za-z][A-Za-z0-9-]*).*/\2/p' \
    | sort -u || true
}

# ─── Compare source vs demo binary: subcommands AND their flag surface ────────
# Subcommand-NAME parity alone is a false-pass: a binary built from older source
# can carry `membrane derive-checks` yet lack a flag the demo invokes (e.g.
# `--run`), passing a name-only check while breaking the demo. So for every
# source subcommand we also require every long flag source exposes to exist on
# the demo binary.
missing_subs=()
stale_flags=()
while IFS= read -r sub; do
  [[ -z "$sub" ]] && continue
  if ! "$AO_BIN" membrane "$sub" --help >/dev/null 2>&1; then
    missing_subs+=("membrane $sub")
    continue
  fi
  demo_flags="$(local_flags "$AO_BIN" "$sub")"
  while IFS= read -r flag; do
    [[ -z "$flag" ]] && continue
    grep -qxF -- "$flag" <<< "$demo_flags" || stale_flags+=("membrane $sub $flag")
  done < <(local_flags "$SRC_BIN" "$sub")
done <<< "$required_subs"

if (( ${#missing_subs[@]} > 0 || ${#stale_flags[@]} > 0 )); then
  fail "demo-path binary is STALE — $AO_BIN lacks membrane surface present in source:"
  # Guard each expansion with a count check: under `set -u`, bash 3.2 (macOS
  # /bin/bash) treats "${empty[@]}" as an unbound-variable error.
  if (( ${#missing_subs[@]} > 0 )); then
    for m in "${missing_subs[@]}"; do fail "  - missing subcommand: ao $m"; done
  fi
  if (( ${#stale_flags[@]} > 0 )); then
    for f in "${stale_flags[@]}"; do fail "  - missing flag: ao $f"; done
  fi
  fail "the e2e membrane demo would run against old machinery and produce a false proof."
  fail "remediation: rebuild + install from source — \`cd cli && make install\`."
  exit 1
fi

note "OK — $AO_BIN has full membrane parity with source ($(echo "$required_subs" | tr '\n' ' ' | sed 's/ *$//'))."
exit 0
