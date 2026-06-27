#!/usr/bin/env bash
# land-lane-run.sh — the LAND LANE: a single serialized writer that owns `main`
# (agentops-2pl.9).
#
# Agents submit branches to the file-queue (.agents/land-queue/requests.jsonl)
# via scripts/land-submit.sh (slice .8). This lane is the SOLE writer to main:
# it pops the OLDEST request, rebases its branch onto current origin/main, runs
# the gate ONCE, lands it (the .7-patched pawl-land.sh does the post-rebase pawl
# stamp + push HEAD:main), and loops. The gate therefore runs once per LANDING,
# not once per racing agent, and there is exactly one writer to main.
#
# ADR-0009 ("use what exists; no bespoke daemon"): this is a thin foreground
# loop, not a service. It reuses the proven primitives:
#   - the mkdir advisory-lock singleton pattern from scripts/push-serial.sh,
#     timed out by AGENTOPS_PUSH_LOCK_TIMEOUT — so a 2nd lane refuses to start
#     (the single-writer invariant);
#   - scripts/land-queue-next.sh to claim the oldest request;
#   - the existing land path (gate -> pawl-land.sh) to gate + push.
#
# Agent-Mail is in degraded_read_only mode and unreliable, so the lane runs on
# the file-queue + a file-based singleton lock; it does NOT depend on `am`.
#
# Host: a foreground loop on the always-on Mac (NEVER bushido — Wi-Fi SPOF).
#
# Usage:
#   scripts/land-lane-run.sh                 # drain the queue then exit
#   scripts/land-lane-run.sh --drain         # (explicit) drain then exit
#   scripts/land-lane-run.sh --once          # land exactly ONE request then exit
#   scripts/land-lane-run.sh --watch [--poll N]
#                                            # run forever; poll every N seconds
#                                            # (default 10) when the queue empties
#
# Knobs (env):
#   AGENTOPS_LAND_QUEUE_DIR     queue dir          (default <repo>/.agents/land-queue)
#   AGENTOPS_PUSH_LOCK_TIMEOUT  lane-lock wait (s) (default 300; PUSH_LOCK_TIMEOUT also honored)
#   AGENTOPS_LAND_LANE_LOCK     lane lock dir      (default <queue>/.lane.lock)
#   LAND_LANE_GATE_CMD          gate+land command  (default the bundled gate that
#                                drives ship -> pawl-review -> pawl-land). The
#                                command is invoked as: <cmd> <bead> <branch-ref>
#                                from a checked-out land-work branch already
#                                rebased onto origin/main. It MUST run the gate
#                                exactly once and, on success, leave main pushed.
#   LAND_LANE_AUTHOR_FAMILY     author family for the gate (default operator)
#   BR_BIN / bd                 issue tracker close on a green land (best-effort)
#
# Exit codes: 0 clean (queue drained / one landed / watch interrupted);
#             1 could not acquire the singleton lane lock (another lane runs);
#             2 usage error.
set -euo pipefail

# --------------------------------------------------------------------------- #
# Resolve repo + scripts
# --------------------------------------------------------------------------- #
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

NEXT_SCRIPT="${LAND_LANE_NEXT_SCRIPT:-$SCRIPT_DIR/land-queue-next.sh}"
PAWL_LAND_SCRIPT="${LAND_LANE_PAWL_LAND_SCRIPT:-$SCRIPT_DIR/pawl-land.sh}"
PAWL_REVIEW_SCRIPT="${LAND_LANE_PAWL_REVIEW_SCRIPT:-$SCRIPT_DIR/pawl-review.sh}"
SHIP_SCRIPT="${LAND_LANE_SHIP_SCRIPT:-$SCRIPT_DIR/ship.sh}"

QUEUE_DIR="${AGENTOPS_LAND_QUEUE_DIR:-$REPO_ROOT/.agents/land-queue}"
QUEUE_FILE="${AGENTOPS_LAND_QUEUE_FILE:-$QUEUE_DIR/requests.jsonl}"
CLAIMS_FILE="${AGENTOPS_LAND_CLAIMS_FILE:-$QUEUE_DIR/claims.jsonl}"
DEADLETTER_FILE="${AGENTOPS_LAND_DEADLETTER_FILE:-$QUEUE_DIR/dead-letter.jsonl}"
DONE_FILE="${AGENTOPS_LAND_DONE_FILE:-$QUEUE_DIR/done.jsonl}"

LANE_LOCK="${AGENTOPS_LAND_LANE_LOCK:-$QUEUE_DIR/.lane.lock}"
# Honor AGENTOPS_PUSH_LOCK_TIMEOUT (the convention used by pre-push.local), then
# push-serial.sh's PUSH_LOCK_TIMEOUT, then a sane default.
LOCK_TIMEOUT="${AGENTOPS_PUSH_LOCK_TIMEOUT:-${PUSH_LOCK_TIMEOUT:-300}}"

GATE_CMD="${LAND_LANE_GATE_CMD:-}"
AUTHOR_FAMILY="${LAND_LANE_AUTHOR_FAMILY:-operator}"
BR_BIN="${BR_BIN:-$(command -v br 2>/dev/null || command -v bd 2>/dev/null || true)}"

POLL_SECONDS=10
MODE="drain"   # drain | once | watch

die() { echo "land-lane: ERROR: $*" >&2; exit 2; }
log() { echo "land-lane: $*" >&2; }

# --------------------------------------------------------------------------- #
# Args
# --------------------------------------------------------------------------- #
while [[ $# -gt 0 ]]; do
  case "$1" in
    --drain) MODE="drain"; shift ;;
    --once)  MODE="once";  shift ;;
    --watch) MODE="watch"; shift ;;
    --poll)
      [[ -n "${2:-}" ]] || die "--poll requires a value"
      POLL_SECONDS="$2"; shift 2 ;;
    -h|--help) sed -n '2,55p' "$0"; exit 0 ;;
    -*) die "unknown flag: $1" ;;
    *)  die "unexpected argument: $1" ;;
  esac
done

mkdir -p "$QUEUE_DIR"
cd "$REPO_ROOT"

# --------------------------------------------------------------------------- #
# Singleton lane lock (mkdir is atomic + portable; flock is absent on macOS).
# A 2nd land-lane-run.sh blocks until LOCK_TIMEOUT then EXITS 1 — it refuses to
# become a second writer to main. Released on any exit (trap).
# --------------------------------------------------------------------------- #
LANE_LOCK_HELD=0
release_lane_lock() {
  if [[ "$LANE_LOCK_HELD" -eq 1 ]]; then
    # Remove the forensic holder file first so the dir is empty for rmdir (the
    # atomic mkdir-lock contract: an EMPTY dir whose creation was the lock). A
    # non-empty lock dir would survive rmdir and wedge the next lane forever.
    rm -f "$LANE_LOCK/holder" 2>/dev/null || true
    rmdir "$LANE_LOCK" 2>/dev/null || true
    LANE_LOCK_HELD=0
  fi
}
# EXIT trap releases the lock (idempotently — release_lane_lock no-ops if the
# lock was already freed). INT/TERM get a SEPARATE handler that releases the lock
# AND terminates the process: a bare `release_lane_lock` on INT/TERM would free
# the lock but let the `--watch` loop keep running, so a second lane could acquire
# the SAME lane lock while this one is still alive — two concurrent writers to
# `main`, violating the single-writer invariant. Exiting with the conventional
# signal code (130 INT / 143 TERM) re-fires the EXIT trap once more, but
# release_lane_lock is idempotent so the lock is freed exactly once. The lock is
# genuinely gone only AFTER this process has exited, so the next lane can start.
on_signal() {
  local code="$1"
  release_lane_lock
  trap - EXIT INT TERM   # disarm so the imminent `exit` does not re-recurse
  exit "$code"
}
trap release_lane_lock EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

acquire_lane_lock() {
  local waited=0
  while ! mkdir "$LANE_LOCK" 2>/dev/null; do
    if [[ "$waited" -ge "$LOCK_TIMEOUT" ]]; then
      echo "land-lane: another land lane already holds $LANE_LOCK (single-writer invariant); refusing to start a second writer to main." >&2
      exit 1
    fi
    sleep 1
    waited=$((waited + 1))
  done
  LANE_LOCK_HELD=1
  # Record the holder for forensics (pid + host); best-effort. release_lane_lock
  # deletes this file before rmdir so the lock always releases cleanly.
  printf '%s %s %s\n' "$$" "$(hostname 2>/dev/null || echo '?')" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$LANE_LOCK/holder" 2>/dev/null || true
  log "acquired singleton lane lock ($LANE_LOCK, pid $$)"
}

# --------------------------------------------------------------------------- #
# JSON helpers (jq when present; printf fallback otherwise)
# --------------------------------------------------------------------------- #
json_escape() {
  local s="${1:-}"
  s="${s//\\/\\\\}"; s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"; s="${s//$'\r'/\\r}"; s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

# append_record <file> <status> <bead> <branch_ref> <reason>
append_record() {
  local file="$1" status="$2" bead="$3" branch="$4" reason="${5:-}"
  local ts; ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  mkdir -p "$(dirname "$file")"
  if command -v jq >/dev/null 2>&1; then
    jq -nc --arg ts "$ts" --arg status "$status" --arg bead "$bead" \
           --arg branch "$branch" --arg reason "$reason" \
           '{timestamp:$ts, status:$status, bead:$bead, branch_ref:$branch, reason:$reason}' \
      >>"$file"
  else
    printf '{"timestamp":"%s","status":"%s","bead":"%s","branch_ref":"%s","reason":"%s"}\n' \
      "$(json_escape "$ts")" "$(json_escape "$status")" "$(json_escape "$bead")" \
      "$(json_escape "$branch")" "$(json_escape "$reason")" >>"$file"
  fi
}

# already_terminal <bead>: true if the bead is already claimed/done/dead-lettered
# (crash-safety: a re-run / drain must not double-process). Checks the side files,
# which is the durable record of what this lane has touched.
already_terminal() {
  local bead="$1" f
  for f in "$CLAIMS_FILE" "$DONE_FILE" "$DEADLETTER_FILE"; do
    [[ -f "$f" ]] || continue
    if command -v jq >/dev/null 2>&1; then
      if jq -e --arg b "$bead" 'select(.bead == $b)' "$f" >/dev/null 2>&1; then
        return 0
      fi
    else
      grep -q "\"bead\":\"$bead\"" "$f" && return 0
    fi
  done
  return 1
}

# --------------------------------------------------------------------------- #
# Claim the oldest UNCLAIMED request, atomically, under a short request-file lock
# (same mkdir pattern land-submit.sh uses to append). Marking the claim BEFORE
# processing means a crash mid-land never re-pops the same request — it lands in
# claims.jsonl and a re-run skips it (operator inspects + re-queues if needed).
#
# Prints "<bead>\t<branch_ref>" on stdout and returns 0 when a request is claimed;
# returns 1 when the queue is empty / all claimed.
# --------------------------------------------------------------------------- #
REQUEST_LOCK="$QUEUE_DIR/.requests.lock"

# request_lock_acquire / request_lock_release: short-held mkdir lock around the
# read-modify-write of requests.jsonl (same primitive land-submit.sh uses to
# append). Acquire returns 0 on success, 1 on timeout.
request_lock_acquire() {
  local waited=0 timeout="${AGENTOPS_LAND_QUEUE_LOCK_TIMEOUT:-30}"
  while ! mkdir "$REQUEST_LOCK" 2>/dev/null; do
    if [[ "$waited" -ge "$timeout" ]]; then
      log "timed out waiting for request lock $REQUEST_LOCK; skipping this tick"
      return 1
    fi
    sleep 1; waited=$((waited + 1))
  done
  return 0
}
request_lock_release() { rmdir "$REQUEST_LOCK" 2>/dev/null || true; }

# Claim the oldest UNCLAIMED request, atomically. Marking the claim BEFORE
# processing means a crash mid-land never re-pops the same request — it lands in
# claims.jsonl and a re-run skips it (operator inspects + re-queues if needed).
# Iterative (NOT recursive): skips any request that is already terminal in the
# side-files, flipping its queue `claimed` flag so next.sh stops returning it.
#
# Prints "<bead>\t<branch_ref>" on stdout and returns 0 when a request is claimed;
# returns 1 when the queue is empty / all claimed / lock contended.
claim_next() {
  request_lock_acquire || return 1

  local line bead branch
  while :; do
    line="$(AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" AGENTOPS_LAND_QUEUE_FILE="$QUEUE_FILE" \
              "$NEXT_SCRIPT" 2>/dev/null || true)"
    if [[ -z "$line" ]]; then
      request_lock_release
      return 1
    fi
    bead="$(printf '%s' "$line" | cut -f1)"
    branch="$(printf '%s' "$line" | cut -f2)"
    if [[ -z "$bead" || -z "$branch" ]]; then
      request_lock_release
      return 1
    fi

    if already_terminal "$bead"; then
      # Already claimed/done/dead-lettered: flip the queue flag so next.sh stops
      # returning it, then look at the next-oldest. If we cannot flip it (no jq),
      # bail rather than spin forever.
      if ! mark_request_claimed "$bead"; then
        request_lock_release
        return 1
      fi
      continue
    fi

    mark_request_claimed "$bead" || true
    append_record "$CLAIMS_FILE" "claimed" "$bead" "$branch" ""
    request_lock_release
    printf '%s\t%s\n' "$bead" "$branch"
    return 0
  done
}

# Flip the request's `claimed` flag to true in requests.jsonl so land-queue-next.sh
# (which filters `(.claimed // false) | not`) stops returning it. Idempotent.
# Returns 0 if the flag was (or already is) set; 1 if it could not be flipped
# (no jq) — callers must NOT loop on an unflippable request (infinite-spin guard).
mark_request_claimed() {
  local bead="$1" tmp
  [[ -f "$QUEUE_FILE" ]] || return 0
  command -v jq >/dev/null 2>&1 || return 1
  tmp="$(mktemp "${TMPDIR:-/tmp}/land-claim.XXXXXX")"
  if jq -c --arg b "$bead" '
        if (type == "object" and .bead == $b) then .claimed = true else . end
      ' "$QUEUE_FILE" >"$tmp" 2>/dev/null && mv "$tmp" "$QUEUE_FILE"; then
    return 0
  fi
  rm -f "$tmp"
  return 1
}

# --------------------------------------------------------------------------- #
# Default gate+land: drive the existing land path for one claimed bead/branch,
# running the gate EXACTLY ONCE, then push to main via pawl-land.sh.
#
# Overridable wholesale with LAND_LANE_GATE_CMD (invoked: <cmd> <bead> <branch>),
# which the e2e test uses to inject a deterministic, countable gate (the real
# pawl-review needs a live cross-family model and cannot run in CI). The default
# below is the production path.
#
# Precondition (set up by process_one before calling): a `land-work` branch is
# checked out, built from origin/land-queue/<bead> and already rebased onto
# origin/main, with HEAD citing the bead.
# --------------------------------------------------------------------------- #
default_gate_and_land() {
  local bead="$1"
  # 1) Gate ONCE: ship's inventory-aware pre-push gate, then the cross-family
  #    pawl review (writes the CONFIRMED verdict pawl-land requires).
  "$SHIP_SCRIPT"
  "$PAWL_REVIEW_SCRIPT" "$bead" --scope head --author-family "$AUTHOR_FAMILY"
  # 2) Land: the .7-patched pawl-land.sh re-fetches origin/main, rebases, rebinds
  #    the verdict to the post-rebase head, and pushes HEAD:main in one shot.
  "$PAWL_LAND_SCRIPT" "$bead"
}

# --------------------------------------------------------------------------- #
# Process ONE claimed request. Returns 0 on a green land, non-zero on failure
# (the caller dead-letters; the lane never aborts on one bad bead).
# --------------------------------------------------------------------------- #
process_one() {
  local bead="$1" branch="$2"
  local short="${branch#refs/heads/}"
  log "claimed $bead ($branch) — fetching + rebasing onto origin/main"

  # Always start from a known-clean main; abandon any leftover land-work.
  git rebase --abort >/dev/null 2>&1 || true
  git checkout -q main 2>/dev/null || git checkout -q -B main 2>/dev/null || true
  git branch -D land-work >/dev/null 2>&1 || true

  if ! git fetch origin main "$short" --quiet 2>/dev/null; then
    # Fall back to fetching everything; a missing queue ref is a hard fail.
    git fetch origin --quiet 2>/dev/null || true
  fi

  # Materialize the queued branch as land-work from the freshly-fetched ref.
  local src_ref="refs/remotes/origin/${short}"
  if ! git rev-parse --verify "$src_ref" >/dev/null 2>&1; then
    src_ref="$short"
  fi
  if ! git checkout -q -B land-work "$src_ref" 2>/dev/null; then
    log "FAIL $bead: cannot check out queued branch ($branch / $src_ref)"
    return 1
  fi

  # Rebase onto current origin/main. On conflict: abort cleanly, dead-letter,
  # and continue the loop — one bad bead never stops the lane.
  if ! git rebase origin/main >/dev/null 2>&1; then
    git rebase --abort >/dev/null 2>&1 || true
    log "FAIL $bead: rebase onto origin/main conflicted"
    return 1
  fi

  # Gate ONCE + land (push HEAD:main). The gate command is responsible for the
  # single gate run and the push; a non-zero exit dead-letters the request.
  if [[ -n "$GATE_CMD" ]]; then
    if ! eval "$GATE_CMD \"\$bead\" \"\$branch\""; then
      log "FAIL $bead: gate/land command failed"
      return 1
    fi
  else
    if ! default_gate_and_land "$bead"; then
      log "FAIL $bead: default gate/land failed"
      return 1
    fi
  fi
  return 0
}

# --------------------------------------------------------------------------- #
# close_bead: best-effort issue-tracker close on a green land (never fatal).
# --------------------------------------------------------------------------- #
close_bead() {
  local bead="$1"
  [[ -n "$BR_BIN" ]] || return 0
  "$BR_BIN" close "$bead" --reason "Landed via land-lane (agentops-2pl.9)" >/dev/null 2>&1 \
    || "$BR_BIN" close "$bead" >/dev/null 2>&1 || true
}

# --------------------------------------------------------------------------- #
# The loop. One iteration: claim -> process -> done|dead-letter.
# Returns 0 if it processed a request, 1 if the queue was empty.
# --------------------------------------------------------------------------- #
process_tick() {
  local claimed bead branch
  claimed="$(claim_next || true)"
  [[ -n "$claimed" ]] || return 1
  bead="$(printf '%s' "$claimed" | cut -f1)"
  branch="$(printf '%s' "$claimed" | cut -f2)"

  if process_one "$bead" "$branch"; then
    append_record "$DONE_FILE" "done" "$bead" "$branch" "landed"
    close_bead "$bead"
    log "LANDED $bead"
  else
    append_record "$DEADLETTER_FILE" "dead-letter" "$bead" "$branch" "land failed; see lane log"
    log "DEAD-LETTERED $bead — lane continues"
  fi
  # Return to a clean main between requests.
  git rebase --abort >/dev/null 2>&1 || true
  git checkout -q main 2>/dev/null || true
  git branch -D land-work >/dev/null 2>&1 || true
  return 0
}

main() {
  acquire_lane_lock

  case "$MODE" in
    once)
      if ! process_tick; then
        log "queue empty — nothing to land"
      fi
      ;;
    drain)
      local n=0
      while process_tick; do n=$((n + 1)); done
      log "drained queue ($n request(s) processed)"
      ;;
    watch)
      log "watch mode — polling every ${POLL_SECONDS}s (Ctrl-C to stop)"
      while true; do
        while process_tick; do :; done
        sleep "$POLL_SECONDS"
      done
      ;;
  esac
}

main
