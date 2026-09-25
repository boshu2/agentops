#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/docs-scope.sh — sourced library: shared LIVE-doc scope resolution
# and the "is this doc historical-by-design" exemption test.
#
# Source it (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/docs-scope.sh"
#
# Shared LIVE-doc scope helper (age-gate-the-ungated-egwt.1)
# so any docs-scoped gate resolves the SAME live-doc set and the SAME
# historical-exemption rule from one place.
#
# IMPORTANT — behavior-preserving contract: this lib does NOT `set -euo pipefail`
# on behalf of its callers; forcing it here could change a caller's behavior.
# The functions below are pure/idempotent and safe under either mode.
#
# Scope resolution is anchored at DOCS_ROOT (default: the current directory).
# Callers that `cd` to the repo root before use get the historical relative
# `docs/...` paths unchanged; tests inject DOCS_ROOT to point at a fixture tree.

# docs_scope_live_files — emit the LIVE docs/**/*.md set (one path per line),
# sorted. Paths are emitted relative to DOCS_ROOT and always begin with `docs/`.
# The exclude list is the set of dated/historical-archive directories.
docs_scope_live_files() {
  local root="${DOCS_ROOT:-.}"
  ( cd "$root" && find docs -name '*.md' \
      -not -path 'docs/adr/*' \
      -not -path 'docs/audits/*' -not -path 'docs/plans/*' -not -path 'docs/brainstorms/*' \
      -not -path 'docs/council*/*' -not -path 'docs/handoffs/*' -not -path 'docs/learnings/*' \
      -not -path 'docs/evidence/*' -not -path 'docs/releases/*' -not -path 'docs/convergence/*' \
      -not -path 'docs/rescope/*' -not -path 'docs/reduction/*' -not -path 'docs/migration-trackers/*' \
      -not -path 'docs/sovereignty-proof/*' -not -path 'docs/rfcs/*' -not -path 'docs/code-map/*' \
      | sort )
}

# docs_scope_is_exempt FILE — return 0 (exempt / historical-by-design) if the
# doc opts out of live-staleness scanning by EITHER:
#   - a migration/upgrade/retirement/closeout/index filename glob
#   - a YAML front-matter block (the file's first line is `---`) that declares
#     the top-level key `status: historical`
# Otherwise return 1 (a live doc, in scope). Prose wording never exempts a doc:
# a page that is historical by design says so in its front matter.
#
# FILE is resolved relative to DOCS_ROOT when it is not already readable as-is,
# so callers that pass a `docs/...` path after `cd`-ing to the root keep working
# unchanged, while a test can point DOCS_ROOT at a fixture tree.
docs_scope_is_exempt() {
  local f="$1"
  case "$f" in
    *-migration*|*-retirement*|*-sunset*|*-closeout*|*CHANGELOG*) return 0 ;;
    *MIGRATION*|*UPGRADING*|*documentation-index*) return 0 ;;
  esac
  local doc="$f"
  if [ ! -r "$doc" ] && [ -n "${DOCS_ROOT:-}" ] && [ -r "${DOCS_ROOT%/}/$f" ]; then
    doc="${DOCS_ROOT%/}/$f"
  fi
  [ -r "$doc" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[ \t\r]*$/) exit; next }
    /^(---|\.\.\.)[ \t\r]*$/ { exit }
    /^status:[ \t]*["'\'']?historical["'\'']?[ \t\r]*(#.*)?$/ { found = 1; exit }
    END { exit found ? 0 : 1 }
  ' "$doc"
}
