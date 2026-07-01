#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/ao-snippet-resolve.sh — sourced library: shared `ao` snippet
# resolution machinery for the skills-tree gate and the docs-tree gate.
#
# Source it (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/ao-snippet-resolve.sh"
#   OR (from a script that has already absolutized its own dir):
#     . "$SCRIPT_DIR/lib/ao-snippet-resolve.sh"
#
# Extracted from scripts/validate-skill-cli-snippets.sh (age-gate-the-ungated-egwt.4)
# so both snippet gates resolve `ao` commands against the SAME live cobra tree
# from one place ("share the core, don't fork it"). The Python resolution core
# lives beside this file as scripts/lib/ao_snippet_resolve.py; this bash lib owns
# the build machinery + exposes the module directory so callers' inline Python
# can import it.
#
# Behavior-preserving contract: this lib does NOT `set -euo pipefail` on behalf
# of its callers (the extracting script sets strict mode itself). The functions
# are pure/idempotent under either mode.

# ao_snippet_lib_dir — absolute path to this lib's directory (holds the Python
# module). Resolved via BASH_SOURCE at source time; safe even if the caller cd's
# afterward, because we absolutize immediately.
ao_snippet_lib_dir() {
  # shellcheck disable=SC1007  # `CDPATH=` clears CDPATH for this one cd (env-prefix, not an assignment)
  CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd
}

# AO_SNIPPET_LIB_DIR — module dir, computed once at source time (pre-cd-safe).
AO_SNIPPET_LIB_DIR="$(ao_snippet_lib_dir)"
export AO_SNIPPET_LIB_DIR

# ao_snippet_resolve_bin [REPO_ROOT] — echo the path to an `ao` binary suitable
# for snippet resolution and export it as AO_BIN.
#
# Honors an already-set AGENTOPS_AO_BIN (fast path for CI / tests). Otherwise
# builds `ao` from <REPO_ROOT>/cli with the ADR-0012 archive tags
# (`-tags "flywheel legacy"`) so snippets that document archived-but-revivable
# commands (e.g. `ao harvest`, `ao forge`, behind //go:build flywheel|legacy)
# still resolve — the default spine build omits them and would false-fail those
# (two prior escapes). The built binary lands in a mktemp dir; the CALLER is
# responsible for its own EXIT trap cleanup if it wants the temp removed.
#
# REPO_ROOT defaults to the parent of this lib's dir's parent (…/scripts/lib →
# repo root), matching the historical layout.
ao_snippet_resolve_bin() {
  local repo_root="${1:-}"
  if [[ -z "$repo_root" ]]; then
    # shellcheck disable=SC1007  # `CDPATH=` clears CDPATH for this one cd (env-prefix, not an assignment)
    repo_root="$(CDPATH= cd "$AO_SNIPPET_LIB_DIR/../.." && pwd)"
  fi

  local ao_bin="${AGENTOPS_AO_BIN:-}"
  if [[ -z "$ao_bin" ]]; then
    local tmp_dir
    tmp_dir="$(mktemp -d)"
    ao_bin="$tmp_dir/ao"
    (
      cd "$repo_root/cli" || exit 1
      go build -tags "flywheel legacy" -o "$ao_bin" ./cmd/ao
    )
    # Surface the temp dir so the caller can trap-clean it.
    AO_SNIPPET_TMP_DIR="$tmp_dir"
    export AO_SNIPPET_TMP_DIR
  fi

  [[ -x "$ao_bin" ]] || {
    echo "Missing or non-executable ao binary: $ao_bin" >&2
    return 1
  }
  AO_BIN="$ao_bin"
  export AO_BIN
  printf '%s\n' "$ao_bin"
}
