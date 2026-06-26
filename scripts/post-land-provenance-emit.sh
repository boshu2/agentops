#!/usr/bin/env bash
# post-land-provenance-emit.sh — feed provenance AFTER commits are on origin/main (age-0tn),
# race-proofed against a hot trunk (age-lyev).
#
# Pre-push emit can record local OIDs that never land on the shared trunk (gate
# rewrites, trailing sensor commits, stale origin/main). This script runs after
# a successful push and records the just-landed range with --trunk-ref origin/main.
#
# WHY A WORKTREE (age-lyev): the old version committed the ledger growth to the
# CANONICAL checkout and printed "(push again to land ledger)" — that deferred
# push raced on a busy trunk and stranded an orphan commit (e.g. 3f6347baf),
# which is the primary driver of the canonical checkout drifting behind origin.
# We instead emit + commit + push from a DISPOSABLE worktree off origin/main:
#   - the canonical checkout is never committed to or left dirty (no orphan, no churn);
#   - `git reset --hard` is safe (the worktree owns nothing);
#   - on a push race we RE-EMIT onto the fresh trunk (the ledger is a prev_hash
#     chain, deliberately NOT union-merged — rebase-replay would break it; re-emit
#     re-chains onto the current tail and dedups by edge identity);
#   - the commit is marked `#trivial` so check-pawl-pre-push.sh waives the
#     provenance-only commit instead of demanding a bead verdict.
#
# Non-blocking by default (exit 0). Skip with AGENTOPS_PROVENANCE_EMIT_SKIP=1.
# Set AGENTOPS_PROVENANCE_EMIT_STRICT=1 when a caller intends to close work on
# successful provenance reconciliation; strict mode turns skipped/failed emit
# preconditions into non-zero exits.
set -uo pipefail

warn() { echo "post-land-provenance: $*" >&2; }

truthy() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

STRICT=0
truthy "${AGENTOPS_PROVENANCE_EMIT_STRICT:-0}" && STRICT=1

skip_or_fail() {
  warn "$1"
  [[ "$STRICT" -eq 1 ]] && exit 1
  exit 0
}

if truthy "${AGENTOPS_PROVENANCE_EMIT_SKIP:-0}"; then
  skip_or_fail "skip requested via AGENTOPS_PROVENANCE_EMIT_SKIP"
fi

# Scrub inherited git environment FIRST (cross-family REFUTE): this script may be
# invoked from inside git's pre-push hook (AGENTOPS_PROVENANCE_EMIT_POST_LAND=1 in
# scripts/hooks/pre-push.local), where git exports GIT_DIR/GIT_WORK_TREE/etc. Those
# env vars OVERRIDE cwd-based discovery, so without scrubbing them every `git` call
# below — including the ones we `cd "$WT"` for — would target the pushing CANONICAL
# checkout instead of the disposable worktree, defeating the isolation and dirtying
# the canonical tree. Unset them so all git ops resolve by cwd.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX \
  GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_NAMESPACE GIT_QUARANTINE_PATH

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || skip_or_fail "not in a git repo; skipping"
cd "$ROOT" || skip_or_fail "cannot cd into repo root; skipping"

# Resolve the ao binary to an ABSOLUTE path: the loop cd's into the worktree, so a
# relative binary path would not resolve there.
AO="${AO_BIN:-}"
if [[ -z "$AO" && -x "$ROOT/cli/bin/ao" ]]; then
  AO="$ROOT/cli/bin/ao"
fi
if [[ -z "$AO" || ! -x "$AO" ]]; then
  skip_or_fail "ao binary not found; skipping"
fi
case "$AO" in /*) ;; *) AO="$ROOT/$AO" ;; esac

REMOTE="${AGENTOPS_PROVENANCE_TRUNK_REMOTE:-origin}"
BRANCH="${AGENTOPS_PROVENANCE_TRUNK_BRANCH:-main}"
TRUNK="${REMOTE}/${BRANCH}"
RETRIES="${AGENTOPS_PROVENANCE_EMIT_RETRIES:-5}"
# Sanitize caller-supplied retry count: a non-numeric value would make the
# `for (( ))` arithmetic error out. Fall back to the default rather than misbehave.
[[ "$RETRIES" =~ ^[1-9][0-9]*$ ]] || RETRIES=5
LEDGER="docs/provenance/ledger.jsonl"
# Verdict files live in the canonical (gitignored) .agents dir — read by ABSOLUTE
# path, since they are absent inside the worktree. The ledger WRITE still resolves
# from the worktree cwd, keeping landed + verdict edges in one chain.
VDIR="${AGENTOPS_PAWL_VERDICTS_DIR:-$ROOT/.agents/pawl-verdicts}"
REQUIRED_VERDICT_BEAD="${AGENTOPS_PROVENANCE_REQUIRED_VERDICT_BEAD:-}"
REQUIRED_VERDICT_HEAD="${AGENTOPS_PROVENANCE_REQUIRED_VERDICT_HEAD:-}"

validate_required_verdict() {
  [[ -n "$REQUIRED_VERDICT_BEAD" ]] || return 0
  required_vf="$VDIR/$REQUIRED_VERDICT_BEAD.json"
  [[ -f "$required_vf" ]] || skip_or_fail "required pawl verdict missing: $required_vf"
  if ! jq -e --arg bead "$REQUIRED_VERDICT_BEAD" \
      '.bead_id == $bead and .disposition == "CONFIRMED"' "$required_vf" >/dev/null 2>&1; then
    skip_or_fail "required pawl verdict is not CONFIRMED for $REQUIRED_VERDICT_BEAD: $required_vf"
  fi
  if [[ -n "$REQUIRED_VERDICT_HEAD" ]]; then
    verdict_head="$(jq -r '.head_sha // ""' "$required_vf" 2>/dev/null || true)"
    [[ "$verdict_head" == "$REQUIRED_VERDICT_HEAD" ]] \
      || skip_or_fail "required pawl verdict is stale: ${verdict_head:-missing} != $REQUIRED_VERDICT_HEAD"
  fi
}

validate_required_verdict

git fetch "$REMOTE" "$BRANCH" --quiet 2>/dev/null || skip_or_fail "could not fetch $TRUNK; skipping"
git rev-parse --verify --quiet "$TRUNK" >/dev/null || skip_or_fail "trunk ref $TRUNK not resolvable; skipping"

# Fixed-SHA range for the just-landed commits, computed ONCE so retries don't drift
# (a ref-relative range would shift each time the trunk moves). emit-landed dedups
# by edge identity, so a re-emit over the same range is idempotent.
NEW_TIP="$(git rev-parse "$TRUNK")"
RANGE_ARGS=(--commit "$NEW_TIP")
if git rev-parse --verify --quiet "${TRUNK}@{1}" >/dev/null 2>&1; then
  OLD_TIP="$(git rev-parse "${TRUNK}@{1}")"
  if [[ "$OLD_TIP" != "$NEW_TIP" ]]; then
    RANGE_ARGS=(--range "${OLD_TIP}..${NEW_TIP}")
  fi
fi

# Reap leaked provenance-emit worktrees from prior SIGKILL'd runs — but ONLY
# those whose owning process is dead. Each worktree path embeds the creating
# PID (agentops-prov-emit.<pid>.XXXXXX); a glob-wide --force sweep would delete a
# CONCURRENT live run's worktree (cross-family REFUTE), so we gate every removal
# on `kill -0 <pid>` failing. Then prune dangling registrations.
while IFS= read -r w; do
  case "$w" in
    *"/agentops-prov-emit."*)
      wpid="$(basename "$w" | sed -n 's/^agentops-prov-emit\.\([0-9][0-9]*\)\..*/\1/p')"
      # Reap only when we can identify a PID that is NOT currently running.
      if [[ -n "$wpid" ]] && ! kill -0 "$wpid" 2>/dev/null; then
        git worktree remove --force "$w" 2>/dev/null || true
      fi
      ;;
  esac
done < <(git worktree list --porcelain 2>/dev/null | awk '/^worktree /{print $2}')
git worktree prune 2>/dev/null || true

WT="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prov-emit.$$.XXXXXX")" || skip_or_fail "mktemp failed; skipping"
rmdir "$WT" 2>/dev/null || true
# shellcheck disable=SC2329 # invoked by trap below
cleanup() { cd "$ROOT" 2>/dev/null || true; git worktree remove --force "$WT" 2>/dev/null || true; git worktree prune 2>/dev/null || true; }
trap cleanup EXIT INT TERM

if ! git worktree add --detach "$WT" "$TRUNK" --quiet 2>/dev/null; then
  skip_or_fail "could not create worktree at $WT; skipping"
fi

for ((attempt = 1; attempt <= RETRIES; attempt++)); do
  cd "$WT" || skip_or_fail "cannot cd into worktree; skipping"
  git fetch "$REMOTE" "$BRANCH" --quiet 2>/dev/null || true
  git reset --hard "$TRUNK" --quiet 2>/dev/null || true   # disposable worktree — safe

  before="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"

  # Landed-edge half + verdict half BOTH run with cwd=$WT so every ledger write
  # resolves to the worktree ledger (no canonical-vs-worktree split chain).
  if ! "$AO" provenance emit-landed --trunk-ref "$TRUNK" "${RANGE_ARGS[@]}" >/dev/null 2>&1; then
    warn "emit-landed failed (attempt $attempt); retrying"
    continue
  fi
  verdict_files=()
  if [[ -n "$REQUIRED_VERDICT_BEAD" ]]; then
    required_vf="$VDIR/$REQUIRED_VERDICT_BEAD.json"
    [[ -f "$required_vf" ]] || skip_or_fail "required pawl verdict missing: $required_vf"
    verdict_files=("$required_vf")
  elif [[ -d "$VDIR" ]]; then
    shopt -s nullglob
    verdict_files=("$VDIR"/*.json)
    shopt -u nullglob
  elif [[ "$STRICT" -eq 1 ]]; then
    skip_or_fail "pawl verdict directory missing: $VDIR"
  fi

  if [[ "${#verdict_files[@]}" -gt 0 ]]; then
    for vf in "${verdict_files[@]}"; do
      if jq -e '.disposition == "CONFIRMED"' "$vf" >/dev/null 2>&1; then
        if ! "$AO" provenance emit-verdict --file "$vf" >/dev/null 2>&1; then
          warn "emit-verdict failed for $vf (attempt $attempt)"
          [[ "$STRICT" -eq 1 ]] && exit 1
        fi
      elif [[ "$STRICT" -eq 1 && -n "$REQUIRED_VERDICT_BEAD" ]]; then
        skip_or_fail "required pawl verdict is not CONFIRMED: $vf"
      fi
    done
  fi

  after="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"
  if [[ "$before" == "$after" ]]; then
    exit 0   # nothing new to record — clean exit, no commit (cleanup trap fires)
  fi

  git add "$LEDGER" 2>/dev/null || skip_or_fail "could not stage $LEDGER"
  if ! git commit -m "chore(provenance): post-land sensor edges (age-0tn trunk-bound emit) #trivial" --quiet 2>/dev/null; then
    warn "commit failed (attempt $attempt); retrying"
    continue
  fi
  # --no-verify is REQUIRED (cross-family REFUTE): this script may run from inside
  # git's pre-push hook, which holds a serial push lock. Re-running the pre-push
  # hook here would deadlock on that lock (and needlessly re-run the whole gate).
  # The commit is provenance-only + #trivial — the gate would waive it anyway —
  # so skipping the local hook is safe and breaks the recursion.
  if git push --no-verify "$REMOTE" "HEAD:$BRANCH" --quiet 2>/dev/null; then
    echo "post-land-provenance: landed trunk-bound edges (attempt $attempt)"
    exit 0
  fi
  warn "push raced (attempt $attempt); re-syncing trunk and re-emitting"
done

warn "could not land provenance after $RETRIES attempts; nothing stranded (worktree discarded)"
[[ "$STRICT" -eq 1 ]] && exit 1
exit 0
