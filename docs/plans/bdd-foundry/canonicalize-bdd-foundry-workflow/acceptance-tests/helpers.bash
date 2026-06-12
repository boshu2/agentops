#!/usr/bin/env bash
# Shared helpers — acceptance suite for canonicalize-bdd-foundry-workflow (phase 2, ATDD).
# Contract + evidence.env key reference: ../acceptance-tests.md
# These tests are the executable definition of done. They MUST be red until the arc lands.

REPO="/Users/bo/dev/agentops"
PLAN="$REPO/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow"
CANON_REL=".claude/workflows/bdd-foundry.js"
CANON="$REPO/$CANON_REL"
INSTALLED="$HOME/.claude/workflows/bdd-foundry.js"
EVIDENCE_ENV="$PLAN/evidence.env"
PLAN_REL="docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow"

fail() { echo "ACCEPTANCE-FAIL: $*" >&2; return 1; }

require_file() { [ -e "$1" ] || fail "required file missing: $1${2:+ — $2}"; }

# Machine-readable arc evidence. The implementing arc MUST write evidence.env
# (see acceptance-tests.md §Evidence contract). Missing file = red suite.
load_evidence() {
  require_file "$EVIDENCE_ENV" "machine-readable arc evidence (acceptance-tests.md §Evidence contract)" || return 1
  set -a
  # shellcheck disable=SC1090
  . "$EVIDENCE_ENV"
  set +a
}

require_var() {
  local n="$1"
  [ -n "${!n:-}" ] || fail "evidence.env missing required key: $n"
}

snapshot_file() { ls "$PLAN"/source-snapshot-*.js 2>/dev/null | head -1; }

# Parse N from a "bdd-foundry vN" header (first line).
header_version() { head -1 "$1" | grep -Eo 'bdd-foundry v[0-9]+' | grep -Eo '[0-9]+'; }

# Absolute physical path (full symlink resolution + normalization).
resolve_path() { python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$1"; }

# Run a recorded evidence command string with explicit cwd + HOME.
# Contract: INSTALL_CMD / DRIFT_CHECK_CMD / BLOCKING_PARENT_CMD / MARKER_CHECK_CMD
# resolve the repo root from cwd (or git) and the installed dir from $HOME —
# never from hardcoded /Users/bo paths (S11, gap 10).
run_cmd_in() { # $1=cwd $2=HOME $3=command-string
  local cwd="$1" home="$2" cmd="$3"
  ( cd "$cwd" && HOME="$home" eval "$cmd" )
}

# Replicate the real installed follow-mechanism for bdd-foundry into a fixture HOME,
# so fixture runs only fault the surface they mutate (S7/E6/X4).
install_good_fixture() { # $1 = fixture HOME
  mkdir -p "$1/.claude/workflows"
  if [ -L "$INSTALLED" ]; then
    ln -sf "$CANON" "$1/.claude/workflows/bdd-foundry.js"
  else
    cp "$CANON" "$1/.claude/workflows/bdd-foundry.js"
  fi
}

# Copy real repo siblings into a fixture HOME (so sibling absence never faults a fixture).
copy_siblings_into() { # $1 = fixture HOME
  mkdir -p "$1/.claude/workflows"
  cp "$REPO/.claude/workflows/bead-crank.js" "$1/.claude/workflows/" 2>/dev/null || true
  cp "$REPO/.claude/workflows/operating-loop.js" "$1/.claude/workflows/" 2>/dev/null || true
}
