#!/usr/bin/env bats
# age-l3xj (D5): the route lease is an ATOMIC EXCLUSIVE primitive (a lock directory with
# owner metadata), not a check-then-write timestamp file. Exactly one route owns the
# service at a time; a second route fails closed BEFORE sending to any pane or writing
# evidence; down/reap serialize against the same lease (no check-then-kill TOCTOU); stale
# (crashed-route) locks are reclaimed within a bounded window. Pure/mockable.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  for b in atm tmux; do printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/$b"; chmod +x "$TMP/bin/$b"; done
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  ROUTE_LOCK="$TMP/route.lock"; ROUTE_TIMEOUT=320
  ROOT="$TMP"; STATE_DIR="pawl"; EVID_DIR="$TMP/evidence"
  log() { :; }
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

# --- the atomic primitive ---

@test "_route_lock_acquire: first caller wins and records owner metadata" {
  run _route_lock_acquire
  [ "$status" -eq 0 ]
  [ -d "$ROUTE_LOCK" ]
  grep -q '"pid"' "$ROUTE_LOCK/owner"
  grep -q '"started"' "$ROUTE_LOCK/owner"
}

@test "_route_lock_acquire: second caller FAILS while the lease is fresh (exclusive)" {
  _route_lock_acquire
  run _route_lock_acquire
  [ "$status" -ne 0 ]
}

@test "_route_lock_acquire: a STALE lease (crashed route) is reclaimed — bounded recovery" {
  mkdir -p "$ROUTE_LOCK"
  printf '{"pid":99999,"started":%s,"session":"x"}\n' "$(( $(date +%s) - 100000 ))" > "$ROUTE_LOCK/owner"
  run _route_lock_acquire
  [ "$status" -eq 0 ]
}

@test "_route_lock_release: releases so the next acquire succeeds" {
  _route_lock_acquire
  _route_lock_release
  run _route_lock_acquire
  [ "$status" -eq 0 ]
}

# --- _route_in_progress over the new primitive ---

@test "_route_in_progress: fresh lease -> in progress (0)" {
  _route_lock_acquire
  run _route_in_progress
  [ "$status" -eq 0 ]
}

@test "_route_in_progress: stale lease -> cleaned + not in progress" {
  mkdir -p "$ROUTE_LOCK"
  printf '{"pid":1,"started":%s,"session":"x"}\n' "$(( $(date +%s) - 100000 ))" > "$ROUTE_LOCK/owner"
  run _route_in_progress
  [ "$status" -ne 0 ]
  [ ! -e "$ROUTE_LOCK" ]
}

@test "_route_in_progress: corrupt owner + OLD dir mtime -> stale (never a permanent wedge)" {
  mkdir -p "$ROUTE_LOCK"
  echo "garbage" > "$ROUTE_LOCK/owner"
  touch -t 199901010000 "$ROUTE_LOCK"    # ancient lease dir => stale even with unreadable owner
  run _route_in_progress
  [ "$status" -ne 0 ]
}

@test "_route_in_progress: corrupt owner + FRESH dir mtime -> in progress (publication window)" {
  mkdir -p "$ROUTE_LOCK"
  echo "garbage" > "$ROUTE_LOCK/owner"   # a half-written owner from a route that just started
  run _route_in_progress
  [ "$status" -eq 0 ]                    # must NOT be mistaken for stale and torn down
}

@test "_route_in_progress: legacy timestamp FILE lock still honored (fresh) + stale-cleaned" {
  date +%s > "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -eq 0 ]
  rm -f "$ROUTE_LOCK"
  echo $(( $(date +%s) - 100000 )) > "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -ne 0 ]
  [ ! -e "$ROUTE_LOCK" ]
}

# --- second route fails closed before evidence/sends ---

@test "cmd_route: fails closed when another route holds the lease — no evidence written" {
  _route_lock_acquire
  load_session() { ENABLED="cc cod"; TIER="multi"; CC_PANE=1; COD_PANE=2; }
  session_exists() { return 0; }
  PKT="$TMP/p.md"; echo x > "$PKT"
  run cmd_route "age-ok" "$PKT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"route lease"* ]]
  [ ! -e "$EVID_DIR" ]
}

# --- down/reap serialize on the same lease ---

@test "cmd_down: refuses (exit 3) while a route holds the lease" {
  _route_lock_acquire
  session_exists() { return 0; }
  run cmd_down
  [ "$status" -eq 3 ]
}

@test "cmd_down: acquires the lease itself (a route starting mid-teardown fails closed) and releases it" {
  session_exists() { return 1; }
  run cmd_down
  [ "$status" -ne 3 ]
  [ ! -e "$ROUTE_LOCK" ]   # released after teardown — no leaked lease
}

@test "cmd_down --force: overrides even a fresh lease" {
  _route_lock_acquire
  session_exists() { return 1; }
  run cmd_down --force
  [ "$status" -ne 3 ]
}

# --- age-l3xj: the sibling verdict writer is SCRIPT-RELATIVE, never $ROOT-relative ---
# Cross-family refuter catch: `bash "$ROOT/scripts/pawl-verdict.sh"` executes the CALLER's
# repo code on the embedded/stranger path ($ROOT = the untrusted repo under review, cwd),
# re-opening the exact RCE the Go trust split closes — and in an ordinary repo the file
# does not exist at all, so `route` would hard-fail there. It must resolve from
# $PAWL_SCRIPT_DIR (repo/scripts in-checkout; the extracted trusted bundle when embedded).

@test "verdict writer resolves script-relative (PAWL_SCRIPT_DIR), not from \$ROOT" {
  [ "$PAWL_VERDICT_SH" = "$PAWL_SCRIPT_DIR/pawl-verdict.sh" ]
  [ -f "$PAWL_VERDICT_SH" ]
}

@test "verdict writer path does NOT follow \$ROOT (a hostile caller repo cannot inject it)" {
  ROOT="/tmp/hostile-caller-repo"
  # Re-source to prove the binding is script-relative, not $ROOT-derived.
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  [[ "$PAWL_VERDICT_SH" != /tmp/hostile-caller-repo/* ]]
  [ "$PAWL_VERDICT_SH" = "$REPO_ROOT/scripts/pawl-verdict.sh" ]
}

@test "no \$ROOT-relative exec of a sibling script remains in pawl.sh (grep guard)" {
  ! grep -qE 'bash "\$ROOT/scripts/' "$REPO_ROOT/scripts/pawl.sh"
  ! grep -qE '"\$ROOT/scripts/pawl-verdict\.sh"' "$REPO_ROOT/scripts/pawl.sh"
}

# --- age-l3xj round-2 refuter catches ---

# R2: the verdict WRITER is trusted-bundle code, but the verdict DATA must land in the
# CALLER's repo. pawl-verdict.sh defaults its dir to $SCRIPT_DIR/../.agents/pawl-verdicts —
# the extracted temp bundle on the embedded path, which Go deletes. Both route write sites
# must pass --dir, and it must resolve to the caller repo (AGENTOPS_REPO_ROOT, else ROOT).
@test "route verdict writes pass --dir pointing at the CALLER repo, not the script bundle" {
  local n
  n="$(grep -cF 'bash "$PAWL_VERDICT_SH" write "$bead" "$pr" --dir "$PAWL_VERDICT_DIR"' "$REPO_ROOT/scripts/pawl.sh")"
  [ "$n" -eq 2 ]
}

@test "PAWL_VERDICT_DIR defaults under the caller's git toplevel (ROOT), not the script dir" {
  # Re-source in a subshell so the derived default is computed fresh, uncontaminated by setup.
  run bash -c 'cd "$1" && unset AGENTOPS_REPO_ROOT PAWL_VERDICT_DIR && source "$1/scripts/pawl.sh" && printf "%s" "$PAWL_VERDICT_DIR"' _ "$REPO_ROOT"
  [ "$output" = "$REPO_ROOT/.agents/pawl-verdicts" ]
  [[ "$output" != "$REPO_ROOT/scripts/"* ]]
}

@test "PAWL_VERDICT_DIR honors AGENTOPS_REPO_ROOT (the embedded-path caller-repo seam)" {
  run bash -c 'unset PAWL_VERDICT_DIR; export AGENTOPS_REPO_ROOT=/tmp/caller-repo; source "$1/scripts/pawl.sh" && printf "%s" "$PAWL_VERDICT_DIR"' _ "$REPO_ROOT"
  [ "$output" = "/tmp/caller-repo/.agents/pawl-verdicts" ]
}

# R3: the lease protects the GLOBAL tmux session, so it must be keyed by SESSION, not by repo —
# otherwise two repos driving one session each take "their own" lock and both route.
@test "the lease is keyed by SESSION and lives outside any repo (two repos, one session, one lock)" {
  local one two
  one="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION=shared-sess; source "$1/scripts/pawl.sh"; ROOT=/repo/one; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  two="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION=shared-sess; source "$1/scripts/pawl.sh"; ROOT=/repo/two; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  [ "$one" = "$two" ]              # SAME session => SAME lease, regardless of repo
  [[ "$one" != /repo/* ]]          # and not inside either repo
  [[ "$one" == *shared-sess* ]]    # keyed by the session name
}

@test "different sessions get different leases (isolation preserved)" {
  local a b
  a="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION=sess-a; source "$1/scripts/pawl.sh"; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  b="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION=sess-b; source "$1/scripts/pawl.sh"; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  [ "$a" != "$b" ]
}

# R12 (refuter round 12): the lease is session-keyed, but per-route packet/evidence files are named
# by BEAD under EVID_DIR. A GLOBAL EVID_DIR let two sessions (two repos) routing the SAME bead id
# write identical evidence paths — one overwrites the other, and a verdict could be validated
# against the wrong review. EVID_DIR must be session-scoped too.
@test "different sessions get different evidence dirs (same bead id cannot collide across repos)" {
  local a b
  a="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION=sess-a; source "$1/scripts/pawl.sh"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  b="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION=sess-b; source "$1/scripts/pawl.sh"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  [ "$a" != "$b" ]
  [[ "$a" == *sess-a* ]]
  [[ "$b" == *sess-b* ]]
  # The bead-named paths under each are therefore distinct even for an identical bead id.
  [ "$a/age-same.packet.md" != "$b/age-same.packet.md" ]
}

@test "PAWL_EVID_DIR still overrides the session-scoped default" {
  local e
  e="$(bash -c 'export PAWL_EVID_DIR=/custom/evid; source "$1/scripts/pawl.sh"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  [ "$e" = "/custom/evid" ]
}

# R17 (refuter round 17): the cross-repo contract lets TWO repos target one existing PAWL_SESSION.
# Evidence scoped by session ALONE then let both repos' same-bead routes share one packet path. It
# must be scoped by session AND repo, so two repos under one session get distinct evidence dirs.
@test "same session, two repos → distinct evidence dirs (no cross-repo overwrite)" {
  local a b
  a="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION=shared; source "$1/scripts/pawl.sh"; ROOT=/repo/one; EVID_DIR=""; EVID_DIR="${TMPDIR:-/tmp}/pawl-evidence-$(_pawl_lease_slug "$SESSION")-$(_pawl_hex "$ROOT")"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  b="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION=shared; source "$1/scripts/pawl.sh"; ROOT=/repo/two; EVID_DIR=""; EVID_DIR="${TMPDIR:-/tmp}/pawl-evidence-$(_pawl_lease_slug "$SESSION")-$(_pawl_hex "$ROOT")"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  [ "$a" != "$b" ]
  [ "$a/age-same.packet.md" != "$b/age-same.packet.md" ]
}

# R7 (refuter round 3): PROJECT was hardcoded "agentops", so `up` from another repo spawned atm
# panes in the wrong project. It must default to the resolved repo (basename of ROOT).
@test "PROJECT defaults to the resolved repo basename, not a hardcoded 'agentops'" {
  local p
  p="$(bash -c 'unset PAWL_PROJECT; source "$1/scripts/pawl.sh"; printf "%s" "$PROJECT"' _ "$REPO_ROOT")"
  [ "$p" = "$(basename "$REPO_ROOT")" ]
}

@test "PAWL_PROJECT still overrides the derived default" {
  local p
  p="$(bash -c 'export PAWL_PROJECT=custom-proj; source "$1/scripts/pawl.sh"; printf "%s" "$PROJECT"' _ "$REPO_ROOT")"
  [ "$p" = "custom-proj" ]
}

# R7b (refuter round 7): SESSION must be the name atm actually creates — ${PROJECT}--${LABEL} —
# so spawn, readiness, route, health, and teardown all target ONE session. Hardcoding it while
# PROJECT derived from the repo spawned '<repo>--pawl-service' but tracked 'agentops--pawl-service'.
@test "SESSION defaults to PROJECT--LABEL (matches what atm spawn creates)" {
  local s
  s="$(bash -c 'unset PAWL_SESSION PAWL_PROJECT; export PAWL_PROJECT=personal-site; source "$1/scripts/pawl.sh"; printf "%s" "$SESSION"' _ "$REPO_ROOT")"
  [ "$s" = "personal-site--pawl-service" ]
}

@test "from the agentops checkout SESSION is unchanged (operator's live session preserved)" {
  local s
  s="$(bash -c 'unset PAWL_SESSION PAWL_PROJECT; source "$1/scripts/pawl.sh"; printf "%s" "$SESSION"' _ "$REPO_ROOT")"
  [ "$s" = "$(basename "$REPO_ROOT")--pawl-service" ]
}

@test "PAWL_SESSION overrides the derived session name" {
  local s
  s="$(bash -c 'export PAWL_SESSION=my--custom; source "$1/scripts/pawl.sh"; printf "%s" "$SESSION"' _ "$REPO_ROOT")"
  [ "$s" = "my--custom" ]
}

# R4: publication window. Between another process's `mkdir lock` and its `owner` write, the
# lease exists with NO owner. Reading that as epoch 0 (=> instantly stale) let a contender
# reclaim a lease someone else had just taken. The dir's own mtime (atomic at mkdir) is the
# fallback, so an owner-less fresh lease reads FRESH and the contender fails closed.
@test "publication window: a lease dir with NO owner file yet is FRESH (contender fails closed)" {
  mkdir -p "$ROUTE_LOCK"                 # simulate: mkdir won, owner not yet written
  [ ! -e "$ROUTE_LOCK/owner" ]
  run _route_lock_fresh
  [ "$status" -eq 0 ]
  run _route_lock_acquire
  [ "$status" -ne 0 ]                    # must NOT steal it
  [ -d "$ROUTE_LOCK" ]
}

@test "an OLD owner-less lease dir is still reclaimable (stale by mtime, no permanent wedge)" {
  mkdir -p "$ROUTE_LOCK"
  touch -t 199901010000 "$ROUTE_LOCK"    # ancient mtime => stale
  run _route_lock_acquire
  [ "$status" -eq 0 ]
  [ -f "$ROUTE_LOCK/owner" ]
}

# R5: PAWL_ROUTE_LOCK is a supported override, and the old code `rm -rf`'d it unvalidated —
# pointing it at any existing directory without a fresh owner deleted that directory's
# contents. A lease dir holds nothing but `owner`; anything else must be refused, not deleted.
@test "a NON-lease directory at PAWL_ROUTE_LOCK is refused, never recursively deleted" {
  ROUTE_LOCK="$TMP/precious"
  mkdir -p "$ROUTE_LOCK"
  echo "important" > "$ROUTE_LOCK/data.txt"
  touch -t 199901010000 "$ROUTE_LOCK"    # look stale, to reach the reclaim path
  run _route_lock_acquire
  [ "$status" -ne 0 ]
  [ -f "$ROUTE_LOCK/data.txt" ]          # the operator's data survives
  [[ "$output" == *"not a pawl lease directory"* ]]
}

@test "_route_lock_release never recursively deletes a non-lease directory" {
  ROUTE_LOCK="$TMP/precious2"
  mkdir -p "$ROUTE_LOCK"
  echo keep > "$ROUTE_LOCK/data.txt"
  run _route_lock_release
  [ -f "$ROUTE_LOCK/data.txt" ]
}

# R14 (refuter round 14): a SYMLINK at ROUTE_LOCK is never our lease — [ -d ]/mv/rmdir follow it,
# so reclaiming it would delete the TARGET's owner (data loss). is_lease must refuse a symlink,
# and release/break must be no-ops against it.
@test "a SYMLINK at ROUTE_LOCK is not a lease; its target's contents are never deleted" {
  local target="$TMP/real-dir"
  mkdir -p "$target"; echo important > "$target/owner"; echo data > "$target/data.txt"
  ROUTE_LOCK="$TMP/link"; ln -s "$target" "$ROUTE_LOCK"
  run _route_lock_is_lease
  [ "$status" -ne 0 ]                    # a symlink is refused
  _route_lock_release                    # must be a no-op
  [ -f "$target/owner" ]                 # target's owner SURVIVES
  [ -f "$target/data.txt" ]
  # break_stale must also refuse (die), never follow the link to delete the target's owner.
  touch -t 199901010000 "$ROUTE_LOCK" 2>/dev/null || true
  run _route_lock_break_stale 1
  [ -f "$target/owner" ]                 # still there regardless of break_stale's exit
}

@test "no unvalidated 'rm -rf \$ROUTE_LOCK' remains in pawl.sh (grep guard)" {
  ! grep -qE 'rm -rf "\$ROUTE_LOCK"' "$REPO_ROOT/scripts/pawl.sh"
}

# R16a/R17 (refuter rounds 16+17): the session→slug map must be INJECTIVE. The sanitized charset
# collides (`sess+a`/`sess=a` → `sess_a`), and a cksum does NOT fix it (the refuter CONSTRUCTED two
# sessions with the same CRC). A reversible HEX encoding is the only guarantee — distinct inputs
# ALWAYS give distinct output.
@test "lease slug is path-distinct for sessions that sanitize to the same charset" {
  local a b
  a="$(bash -c 'source "$1/scripts/pawl.sh"; _pawl_lease_slug "sess+a"' _ "$REPO_ROOT")"
  b="$(bash -c 'source "$1/scripts/pawl.sh"; _pawl_lease_slug "sess=a"' _ "$REPO_ROOT")"
  [ "$a" != "$b" ]
  local a2; a2="$(bash -c 'source "$1/scripts/pawl.sh"; _pawl_lease_slug "sess+a"' _ "$REPO_ROOT")"
  [ "$a" = "$a2" ]                        # deterministic
}

@test "the refuter's constructed cksum-collision pair now maps to DISTINCT slugs (hex injective)" {
  # These two distinct strings share cksum 3694045643 (the round-17 refuter's collision).
  local s1='++==++=+==+=++++===+++==++=++=+==++++==+'
  local s2='+=+====++=++=++=+=++=+=++====++++===+=+='
  local a b
  a="$(bash -c 'source "$1/scripts/pawl.sh"; _pawl_lease_slug "$2"' _ "$REPO_ROOT" "$s1")"
  b="$(bash -c 'source "$1/scripts/pawl.sh"; _pawl_lease_slug "$2"' _ "$REPO_ROOT" "$s2")"
  [ "$a" != "$b" ]                        # cksum would have collided; hex does not
}

@test "distinct sessions that sanitize alike get distinct lease AND evidence paths end to end" {
  local la lb ea eb
  la="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION="sess+a"; source "$1/scripts/pawl.sh"; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  lb="$(bash -c 'unset PAWL_ROUTE_LOCK; export PAWL_SESSION="sess=a"; source "$1/scripts/pawl.sh"; printf "%s" "$ROUTE_LOCK"' _ "$REPO_ROOT")"
  ea="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION="sess+a"; source "$1/scripts/pawl.sh"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  eb="$(bash -c 'unset PAWL_EVID_DIR; export PAWL_SESSION="sess=a"; source "$1/scripts/pawl.sh"; printf "%s" "$EVID_DIR"' _ "$REPO_ROOT")"
  [ "$la" != "$lb" ]
  [ "$ea" != "$eb" ]
}

# R16b (refuter round 16): stale reclaim must NOT rm an unrelated regular file that PAWL_ROUTE_LOCK
# points at (the default is now a predictable /tmp path). A legacy lock file holds ONLY a numeric
# epoch; anything else (an operator's real file) is refused, never deleted.
@test "a regular NON-lock file at ROUTE_LOCK is refused, never deleted by reclaim" {
  ROUTE_LOCK="$TMP/precious.conf"
  printf 'important config\nkeep me\n' > "$ROUTE_LOCK"
  run _route_lock_is_lease
  [ "$status" -ne 0 ]                    # non-numeric content => not a legacy lock
  run _route_lock_break_stale 0
  [ -f "$ROUTE_LOCK" ]                    # the file SURVIVES regardless of break_stale's exit
  grep -q "important config" "$ROUTE_LOCK"
}

@test "a genuine legacy lock file (numeric epoch) IS still recognized + reclaimable" {
  ROUTE_LOCK="$TMP/legacy.lock"
  echo "$(( $(date +%s) - 100000 ))" > "$ROUTE_LOCK"   # old numeric epoch => stale legacy lock
  run _route_lock_is_lease
  [ "$status" -eq 0 ]                    # numeric content => a legacy lock
  _route_lock_break_stale "$(cat "$ROUTE_LOCK")"
  [ ! -e "$ROUTE_LOCK" ]                 # a real stale legacy lock is still cleaned
}

# age-l3xj: the routed-REFUTE structured membrane-catch emitter (_route_emit_catch, age-2yh2)
# must stay present + reachable. A wholesale-file rebuild of pawl.sh from a stale base silently
# reverted it once (the pawl-review-refuted-see-evidence escape reappeared; the cross-family
# refuter caught it). This is that deterministic audit as a standing guard: define AND call.
@test "routed-REFUTE membrane-catch emitter is defined and called in pawl.sh (no silent revert)" {
  grep -qE '^_route_emit_catch\(\)' "$REPO_ROOT/scripts/pawl.sh"
  grep -qE '^_refuting_evidence\(\)' "$REPO_ROOT/scripts/pawl.sh"
  # A reachable call site (not just the definition).
  grep -qE '^\s*_route_emit_catch ' "$REPO_ROOT/scripts/pawl.sh"
}

@test "release removes the lease it owns (rmdir semantics, dir gone)" {
  _route_lock_acquire
  [ -d "$ROUTE_LOCK" ]
  _route_lock_release
  [ ! -e "$ROUTE_LOCK" ]
}

# R9 (refuter round 9): a long route can outlive its lease TTL; a successor reclaims it; then the
# ORIGINAL route's RETURN trap must NOT delete the successor's lease. Release is ownership-checked.
@test "release is a no-op when a SUCCESSOR (different pid) owns the lease" {
  # A lease owned by some other pid (the successor).
  mkdir -p "$ROUTE_LOCK"
  printf '{"pid":999999,"started":%s,"session":"s"}\n' "$(date +%s)" > "$ROUTE_LOCK/owner"
  _route_lock_release                    # our ($$) release must not touch a lease pid 999999 owns
  [ -d "$ROUTE_LOCK" ]                    # successor's lease SURVIVES
  [ "$(_route_lock_owner_pid)" = "999999" ]
}

@test "release removes only OUR lease; owner pid is recorded as this process" {
  _route_lock_acquire
  [ "$(_route_lock_owner_pid)" = "$$" ]
  _route_lock_release
  [ ! -e "$ROUTE_LOCK" ]
}

# Heartbeat: a route touches its own lease to keep it fresh past the TTL, so it is never reclaimed
# while alive. touch only refreshes a lease WE own (a successor's lease is never bumped).
@test "touch refreshes OUR lease's started epoch (heartbeat keeps a long route's lease fresh)" {
  _route_lock_acquire
  # Backdate the owner to look nearly stale.
  printf '{"pid":%s,"started":%s,"session":"s"}\n' "$$" "$(( $(date +%s) - (ROUTE_TIMEOUT + 30) ))" > "$ROUTE_LOCK/owner"
  local before; before="$(_route_lock_started)"
  _route_lock_touch
  local after; after="$(_route_lock_started)"
  [ "$after" -gt "$before" ]            # heartbeat advanced the freshness clock
  _route_lock_fresh                      # and the lease is fresh again
}

@test "touch does NOT refresh a lease owned by another pid" {
  mkdir -p "$ROUTE_LOCK"
  local old; old="$(( $(date +%s) - 100000 ))"
  printf '{"pid":999999,"started":%s,"session":"s"}\n' "$old" > "$ROUTE_LOCK/owner"
  _route_lock_touch
  [ "$(_route_lock_started)" = "$old" ] # unchanged — we never heartbeat someone else's lease
}

# R10 (refuter round 10): a static TTL cannot cover the pre-poll send/respawn phase (~700s+); the
# BACKGROUND heartbeat, tied to the route process lifetime, keeps the lease fresh across ALL phases.
# Deterministic + fast: tiny ROUTE_TIMEOUT (window = 2*ROUTE_TIMEOUT = 4s) + a 1s heartbeat, held
# LONGER than the window, must stay fresh and reject a successor the whole time.
# R15 (refuter round 15b): a supported PAWL_ROUTE_TIMEOUT=0 must NOT collapse the freshness window
# to 0 (which marked every live lease instantly stale). The window is floored to 2.
@test "PAWL_ROUTE_TIMEOUT=0 does not make a live lease instantly stale (window floored)" {
  run bash -c 'source "$1/scripts/pawl.sh"; ROUTE_TIMEOUT=0; printf "%s" "$(_route_lock_window)"' _ "$REPO_ROOT"
  [ "$output" -ge 2 ]
  # A just-started lease is FRESH even at ROUTE_TIMEOUT=0.
  ROUTE_TIMEOUT=0
  _route_lock_started() { date +%s; }
  mkdir -p "$ROUTE_LOCK"; printf '{"pid":%s,"started":%s}\n' "$$" "$(date +%s)" > "$ROUTE_LOCK/owner"
  run _route_lock_fresh
  [ "$status" -eq 0 ]
}

# R11 (refuter round 11): the heartbeat interval must ALWAYS be strictly less than the freshness
# window (2*ROUTE_TIMEOUT), with margin — else a live owner's lease expires between beats. It is
# now derived from the window, and any env override is hard-clamped. Assert the invariant holds
# across a range of ROUTE_TIMEOUT values AND a hostile override.
@test "heartbeat interval is always < freshness window (derived, with margin)" {
  for rt in 1 2 5 30 320; do
    local iv window
    iv="$(bash -c 'unset PAWL_HEARTBEAT_INTERVAL; source "$1/scripts/pawl.sh"; ROUTE_TIMEOUT='"$rt"'; _route_heartbeat_interval' _ "$REPO_ROOT")"
    window=$(( rt * 2 ))
    if [ "$window" -lt 2 ]; then window=2; fi
    [ "$iv" -ge 1 ]
    [ "$iv" -lt "$window" ]                 # strict invariant
    [ "$iv" -le "$(( window / 2 ))" ]       # >= 2 beats per window
  done
}

@test "a hostile PAWL_HEARTBEAT_INTERVAL override is clamped below the window" {
  local iv
  iv="$(bash -c 'export PAWL_HEARTBEAT_INTERVAL=9999; source "$1/scripts/pawl.sh"; ROUTE_TIMEOUT=2; _route_heartbeat_interval' _ "$REPO_ROOT")"
  [ "$iv" -lt 4 ]                            # window=4 → clamped to <= window/2 = 2
  # A non-numeric override falls back to the derived default, never breaks the invariant.
  iv="$(bash -c 'export PAWL_HEARTBEAT_INTERVAL=abc; source "$1/scripts/pawl.sh"; ROUTE_TIMEOUT=2; _route_heartbeat_interval' _ "$REPO_ROOT")"
  [ "$iv" -ge 1 ]
  [ "$iv" -lt 4 ]
}

@test "background heartbeat keeps a lease fresh past the TTL for the process lifetime" {
  ROUTE_TIMEOUT=2                          # freshness window = 4s
  PAWL_HEARTBEAT_INTERVAL=1
  # A worker process acquires + heartbeats for ~6s (> the 4s window), writing liveness ticks.
  bash -c '
    source "$2/scripts/pawl.sh"; ROUTE_LOCK="$1"; log(){ :; }
    ROUTE_TIMEOUT=2; PAWL_HEARTBEAT_INTERVAL=1
    _route_lock_acquire || exit 7
    _route_lock_heartbeat_start
    printf "%s" "$$" > "$3.pid"            # the owner pid
    sleep 6
    _route_lock_heartbeat_stop
    _route_lock_release
  ' _ "$ROUTE_LOCK" "$REPO_ROOT" "$TMP/hb" &
  local worker=$!
  sleep 5                                   # well past the 4s window
  # While the worker still heartbeats: the lease is FRESH and a successor FAILS CLOSED.
  run bash -c "source '$REPO_ROOT/scripts/pawl.sh'; ROUTE_LOCK='$ROUTE_LOCK'; ROUTE_TIMEOUT=2; log(){ :; }; if _route_lock_fresh; then echo fresh; else echo stale; fi"
  [ "$output" = "fresh" ]
  run bash -c "source '$REPO_ROOT/scripts/pawl.sh'; ROUTE_LOCK='$ROUTE_LOCK'; ROUTE_TIMEOUT=2; log(){ :; }; if _route_lock_acquire; then echo won; else echo lost; fi"
  [ "$output" = "lost" ]                    # successor cannot steal a live, heartbeating lease
  wait "$worker"
  # After the worker exits + releases, the lease is gone (or ages out).
  [ ! -d "$ROUTE_LOCK" ] || ! bash -c "source '$REPO_ROOT/scripts/pawl.sh'; ROUTE_LOCK='$ROUTE_LOCK'; ROUTE_TIMEOUT=2; _route_lock_fresh"
}

# R23 (refuter round 23): the heartbeat's ownership-check and owner-write are not atomic, so a
# delayed heartbeat could, after release + successor-acquire, overwrite the successor's owner.
# _route_lock_heartbeat_stop must WAIT for the heartbeat to die BEFORE release, so no in-flight
# write survives into a successor's dir.
@test "heartbeat_stop synchronously terminates the heartbeat (dead before release)" {
  ROUTE_TIMEOUT=2; PAWL_HEARTBEAT_INTERVAL=1
  _route_lock_acquire
  _route_lock_heartbeat_start
  local hb="$_ROUTE_HEARTBEAT_PID"
  kill -0 "$hb"                          # the heartbeat is alive
  _route_lock_heartbeat_stop
  run kill -0 "$hb"
  [ "$status" -ne 0 ]                    # after stop it is DEAD — stop WAITED (not fire-and-forget)
}

@test "no stale heartbeat write reaches a successor's lease after release" {
  ROUTE_TIMEOUT=2; PAWL_HEARTBEAT_INTERVAL=1
  _route_lock_acquire
  _route_lock_heartbeat_start
  _route_lock_heartbeat_stop            # synchronous: heartbeat is now dead
  _route_lock_release                   # our lease gone
  _route_lock_acquire                   # successor acquires a FRESH lease (this test process)
  local owner_before; owner_before="$(cat "$ROUTE_LOCK/owner")"
  sleep 2                               # > the old heartbeat interval; a live stale hb would fire here
  [ "$(cat "$ROUTE_LOCK/owner")" = "$owner_before" ]   # successor owner UNCHANGED (no stale write)
}

@test "a CRASHED route (heartbeat dies with the process) lets the lease go stale — no orphan wedge" {
  ROUTE_TIMEOUT=2
  PAWL_HEARTBEAT_INTERVAL=1
  # Worker acquires + heartbeats, then is KILLED (SIGKILL) — the heartbeat must die with it.
  bash -c '
    source "$2/scripts/pawl.sh"; ROUTE_LOCK="$1"; log(){ :; }
    ROUTE_TIMEOUT=2; PAWL_HEARTBEAT_INTERVAL=1
    _route_lock_acquire; _route_lock_heartbeat_start
    sleep 60
  ' _ "$ROUTE_LOCK" "$REPO_ROOT" &
  local worker=$!
  sleep 2
  kill -9 "$worker" 2>/dev/null || true     # simulate a crash: no release, no trap
  wait "$worker" 2>/dev/null || true
  sleep 5                                     # > 2*ROUTE_TIMEOUT (4s) with NO heartbeat now
  run bash -c "source '$REPO_ROOT/scripts/pawl.sh'; ROUTE_LOCK='$ROUTE_LOCK'; ROUTE_TIMEOUT=2; log(){ :; }; if _route_lock_fresh; then echo fresh; else echo stale; fi"
  [ "$output" = "stale" ]                     # the orphaned heartbeat did NOT keep it alive
  run bash -c "source '$REPO_ROOT/scripts/pawl.sh'; ROUTE_LOCK='$ROUTE_LOCK'; ROUTE_TIMEOUT=2; log(){ :; }; if _route_lock_acquire; then echo won; else echo lost; fi"
  [ "$output" = "won" ]                       # a new route can reclaim it
}

# The full refuter-9 interleaving, deterministic: A acquires; A's lease goes stale (TTL) and B
# reclaims it (fresh, B's pid); A finishes and releases — B's lease must survive and A must not
# have concurrently held it once B reclaimed.
@test "long-route interleaving: A's release cannot dispossess B's reclaimed lease (refuter round-9)" {
  _route_lock_acquire                    # A ($$) holds the lease
  local a_pid; a_pid="$(_route_lock_owner_pid)"; [ "$a_pid" = "$$" ]
  # A's lease goes stale (simulate A running past its TTL without heartbeat).
  printf '{"pid":%s,"started":%s,"session":"s"}\n' "$$" "$(( $(date +%s) - 100000 ))" > "$ROUTE_LOCK/owner"
  # B reclaims the stale lease in a separate process (distinct pid).
  bash -c 'source "$2/scripts/pawl.sh"; ROUTE_LOCK="$1"; log(){ :; }; _route_lock_acquire && printf "%s" "$(_route_lock_owner_pid)" > "$3"' _ "$ROUTE_LOCK" "$REPO_ROOT" "$TMP/bpid"
  local b_pid; b_pid="$(cat "$TMP/bpid")"
  [ -n "$b_pid" ] && [ "$b_pid" != "$a_pid" ]   # B owns a fresh lease now
  # A finishes and releases — must be a no-op (A no longer owns it).
  _route_lock_release
  [ -d "$ROUTE_LOCK" ]                    # B's lease SURVIVES
  [ "$(_route_lock_owner_pid)" = "$b_pid" ]
}

# R6 (refuter round 3): stale-lease reclaim must be SINGLE-WINNER. The earlier reclaim renamed
# unconditionally and returned success even on a no-op, so two contenders racing one stale lease
# both "acquired" it (acquire_results=0,0). mkdir is now the sole ownership barrier and the
# rename the sole reclaim arbiter. This is the deterministic barrier harness the refuter used.
@test "concurrent acquire on a STALE lease yields exactly ONE winner (no 0,0 double-hold)" {
  # Seed a stale lease.
  mkdir -p "$ROUTE_LOCK"; printf '{"pid":1,"started":%s,"session":"x"}\n' "$(( $(date +%s) - 100000 ))" > "$ROUTE_LOCK/owner"
  local barrier="$TMP/go"
  run_one() {
    # Wait on a shared barrier file so both processes hit acquire at ~the same instant.
    while [ ! -e "$barrier" ]; do :; done
    if _route_lock_acquire; then echo won > "$TMP/res.$1"; else echo lost > "$TMP/res.$1"; fi
  }
  run_one A & local pa=$!
  run_one B & local pb=$!
  sleep 0.2; : > "$barrier"        # release both
  wait "$pa"; wait "$pb"
  local a b; a="$(cat "$TMP/res.A")"; b="$(cat "$TMP/res.B")"
  # Exactly one won.
  local wins=0; [ "$a" = won ] && wins=$((wins+1)); [ "$b" = won ] && wins=$((wins+1))
  [ "$wins" -eq 1 ]
  [ -d "$ROUTE_LOCK" ]             # the winner holds a real lease
  [ -f "$ROUTE_LOCK/owner" ]
}

# The EXACT interleaving refuter round 5 named: A finishes acquisition (holds a FRESH lease); B,
# still carrying the OLD stale generation it observed, then runs the break. B must NOT dispossess
# A's fresh lease (different generation) and must fail closed. The old rename-based reclaim renamed
# A's fresh lease aside and both routed (two winners); the generation token refuses it.
@test "stale generation B cannot dispossess A's FRESH lease (refuter round-5 interleaving)" {
  # Seed a stale lease of a known OLD generation.
  local oldgen=1000000000
  mkdir -p "$ROUTE_LOCK"; printf '{"pid":1,"started":%s}\n' "$oldgen" > "$ROUTE_LOCK/owner"
  touch -t 199901010000 "$ROUTE_LOCK"
  # A fully acquires -> replaces the stale lease with a FRESH one (a new generation).
  _route_lock_acquire
  [ -d "$ROUTE_LOCK" ]; _route_lock_fresh
  local a_started; a_started="$(_route_lock_started)"
  [ "$a_started" != "$oldgen" ]        # A's lease is a new generation
  # B, holding the OLD generation, runs the break — it must refuse to touch A's fresh lease.
  _route_lock_break_stale "$oldgen"
  [ -d "$ROUTE_LOCK" ]                  # A's lease survives
  _route_lock_fresh                     # still fresh
  [ "$(_route_lock_started)" = "$a_started" ]   # SAME lease, untouched
  # And B's re-race of the mkdir fails => B fails closed, no double-hold.
  run bash -c "mkdir '$ROUTE_LOCK' 2>/dev/null"
  [ "$status" -ne 0 ]
}

@test "break-token serializes breakers of one generation; a crashed token ages out (no wedge)" {
  local gen=1000000000
  local btok="${ROUTE_LOCK}.break.${gen}"
  mkdir -p "$ROUTE_LOCK"; printf '{"pid":1,"started":%s}\n' "$gen" > "$ROUTE_LOCK/owner"
  touch -t 199901010000 "$ROUTE_LOCK"
  # A LIVE break-token (fresh) blocks a second breaker of the same generation.
  mkdir "$btok"
  run _route_lock_break_stale "$gen"
  [ "$status" -ne 0 ]                   # refused: a live breaker holds the token
  [ -d "$ROUTE_LOCK" ]                  # the stale lease is left for that breaker
  # An AGED break-token (crashed breaker) is reclaimed, and the break proceeds.
  touch -t 199901010000 "$btok"
  run _route_lock_break_stale "$gen"
  [ "$status" -eq 0 ]
  [ ! -e "$ROUTE_LOCK" ]               # the stale lease was finally broken
}
