#!/usr/bin/env bash
# post-land-provenance-emit.sh — feed provenance AFTER commits are on origin/main (age-0tn).
#
# Pre-push emit can record local OIDs that never land on the shared trunk (gate
# rewrites, trailing sensor commits, stale origin/main). This script runs after
# a successful push: fetch trunk, emit only for the just-landed range with
# --trunk-ref origin/main, optionally emit verdict edges, then commit ledger growth.
#
# Non-blocking by default (exit 0). Skip with AGENTOPS_PROVENANCE_EMIT_SKIP=1.
set -uo pipefail

if [[ "${AGENTOPS_PROVENANCE_EMIT_SKIP:-0}" == "1" ]]; then
  exit 0
fi

warn() { echo "post-land-provenance: $*" >&2; }

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { warn "not in a git repo; skipping"; exit 0; }
cd "$ROOT" || exit 0

AO="${AO_BIN:-}"
if [[ -z "$AO" && -x "cli/bin/ao" ]]; then
  AO="$PWD/cli/bin/ao"
fi
if [[ -z "$AO" || ! -x "$AO" ]]; then
  warn "ao binary not found; skipping"
  exit 0
fi

REMOTE="${AGENTOPS_PROVENANCE_TRUNK_REMOTE:-origin}"
BRANCH="${AGENTOPS_PROVENANCE_TRUNK_BRANCH:-main}"
TRUNK="${REMOTE}/${BRANCH}"

git fetch "$REMOTE" "$BRANCH" --quiet 2>/dev/null || {
  warn "could not fetch $TRUNK; skipping"
  exit 0
}

if ! git rev-parse --verify --quiet "$TRUNK" >/dev/null; then
  warn "trunk ref $TRUNK not resolvable; skipping"
  exit 0
fi

# Commits that landed on trunk since the previous remote tip (best-effort).
RANGE=""
if git rev-parse --verify --quiet "${TRUNK}@{1}" >/dev/null 2>&1; then
  old_tip="$(git rev-parse "${TRUNK}@{1}")"
  new_tip="$(git rev-parse "$TRUNK")"
  if [[ "$old_tip" != "$new_tip" ]]; then
    RANGE="${old_tip}..${new_tip}"
  fi
fi
if [[ -z "$RANGE" ]]; then
  # Fallback: last commit on trunk only.
  RANGE="$(git rev-parse "$TRUNK")"
fi

LEDGER="docs/provenance/ledger.jsonl"
before="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"

emit_args=(provenance emit-landed --trunk-ref "$TRUNK")
if [[ "$RANGE" == *".."* ]]; then
  emit_args+=(--range "$RANGE")
else
  emit_args+=(--commit "$RANGE")
fi

if ! "$AO" "${emit_args[@]}" >/dev/null 2>&1; then
  warn "emit-landed failed for range=$RANGE trunk=$TRUNK; skipping"
  exit 0
fi

# Verdict half: emit CONFIRMED pawl verdicts not yet in the ledger.
VDIR="${AGENTOPS_PAWL_VERDICTS_DIR:-$ROOT/.agents/pawl-verdicts}"
if [[ -d "$VDIR" ]]; then
  for vf in "$VDIR"/*.json; do
    [[ -f "$vf" ]] || continue
    if jq -e '.disposition == "CONFIRMED"' "$vf" >/dev/null 2>&1; then
      "$AO" provenance emit-verdict --file "$vf" >/dev/null 2>&1 || true
    fi
  done
fi

after="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"
if [[ "$before" == "$after" ]]; then
  exit 0
fi

if ! git add "$LEDGER" 2>/dev/null; then
  warn "could not stage $LEDGER"
  exit 0
fi
if git commit -m "chore(provenance): post-land sensor edges (age-0tn trunk-bound emit)" >/dev/null 2>&1; then
  echo "post-land-provenance: appended trunk-bound edges for $RANGE (push again to land ledger)"
else
  warn "ledger updated but commit failed; left staged"
fi
exit 0
