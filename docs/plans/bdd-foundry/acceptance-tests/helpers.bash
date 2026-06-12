#!/usr/bin/env bash
# helpers.bash — shared harness for the land.sh acceptance suite.
# bdd-foundry Phase 2 (ATDD): every test here is the executable definition of
# done for one frozen scenario in docs/plans/bdd-foundry/behaviors.md.
# The suite is RED by design until scripts/land.sh exists and satisfies it.
#
# ───────────────────────── PINNED CONTRACT ─────────────────────────
# Everything an implementation must expose to be observable by this suite is
# pinned HERE, in one place, so the spec phase can renegotiate a pin by editing
# this file only (never by weakening a scenario — that re-opens Phase 1).
#
# SUT:            scripts/land.sh (single entry point, committed in the repo)
# Subcommands:    --status [--json] · --abort · --dry-run · --install
#                 --help · --version
#                 --verify-generated-json (B47 standalone strict-JSON verifier)
#                 --check-counts          (B48 standalone count checker)
# Config knobs (CLI flag > env > repo config land.* > built-in default — B23):
#   LAND_LOCK_DIR            / --lock-dir        lock storage root (shared)
#   LAND_STALE_TTL           / --stale-ttl       seconds (default 900)
#   LAND_WAIT_TIMEOUT        / --wait-timeout    seconds (0 = fail if held)
#   LAND_HEARTBEAT_INTERVAL  / --heartbeat-interval seconds (default 30)
#   LAND_GATE_TIMEOUT        / --gate-timeout    seconds per gate check
#   LAND_MAX_REBASE_ATTEMPTS / --max-rebase-attempts (default 2)
#   LAND_REMOTE / LAND_BASE_BRANCH               remote + base branch names
# Lock storage layout under $LAND_LOCK_DIR:
#   lock.json   — holder record: {"id","pid","start_time","heartbeat","nonce"}
#                 id = host:worktree:pid:start_time (B28)
#   queue/      — one JSON entry per waiter
#   audit.jsonl — append-only JSONL audit log (acquire/release/stale-takeover/
#                 corrupt-lock/abort entries; correlation ids — B68)
# Test seams (active only when LAND_TEST_MODE=1; hermetic-harness contract):
#   sandbox marker: a file named "land-sandbox" inside the bare remote's git
#     dir; in test mode land.sh refuses to push to any remote lacking it (B25)
#   LAND_TEST_CRASH_AFTER=<phase> — kill -9 self after phase ∈
#     {rebase,regen-write,regen-commit,gate,push,pre-release} (B57)
#   LAND_TEST_AFTER_GATE_CMD=<cmd> — run <cmd> immediately after a gate pass
#     (used to inject out-of-band remote churn — B12/B56, push faults — B53)
#   LAND_TEST_GATE_SLEEP=<sec> — artificially lengthen this invocation's gate
#     phase (B3/B5/B31)
# Fixture repo seams (the sandbox models the real repo's regen system):
#   scripts/regen-all.sh             mini generator battery (+ --check)
#   scripts/generators/*.sh          one generator per derived surface
#   scripts/regen-manifest.txt       the declared generator-owned write set (B42)
#   scripts/count-docs.txt           manifest of count-bearing docs (B48)
#   scripts/gate.d/*.sh              additional gate checks (exit nonzero = fail)
#   .github/workflows/validate.yml   carries "# land-gate-families: ..." for
#                                    the B49 CI-parity check
# Exit-code taxonomy (B67; pinned numbers — renegotiate here only):
#   0 success / no-op · 10 preflight refusal · 11 lock wait timeout
#   12 source conflict · 13 gate failure · 14 push failure · 20 internal error
#   3 dry-run "would be blocked" (B71)
# ─────────────────────────────────────────────────────────────────────

set -o pipefail

EXIT_OK=0
EXIT_PREFLIGHT=10
EXIT_LOCK_TIMEOUT=11
EXIT_CONFLICT=12
EXIT_GATE=13
EXIT_PUSH=14
EXIT_INTERNAL=20
EXIT_DRYRUN_BLOCKED=3

# Repo under test: docs/plans/bdd-foundry/acceptance-tests → 4 levels up.
repo_under_test() {
  cd "$BATS_TEST_DIRNAME/../../../.." && pwd
}

# ── sandbox lifecycle ────────────────────────────────────────────────

sandbox_setup() {
  REPO_ROOT="$(repo_under_test)"
  SANDBOX="$(mktemp -d "${BATS_TEST_TMPDIR:-${TMPDIR:-/tmp}}/land-sbx.XXXXXX")"
  LOCKDIR="$SANDBOX/landlock"
  mkdir -p "$LOCKDIR" "$SANDBOX/lanes" "$SANDBOX/out"
  : > "$SANDBOX/pids"

  export LAND_LOCK_DIR="$LOCKDIR"
  export LAND_TEST_MODE=1
  # Fast suite defaults; individual tests override.
  export LAND_STALE_TTL=5
  export LAND_WAIT_TIMEOUT=15
  export LAND_HEARTBEAT_INTERVAL=1
  unset LAND_GATE_TIMEOUT LAND_MAX_REBASE_ATTEMPTS LAND_TEST_CRASH_AFTER \
        LAND_TEST_AFTER_GATE_CMD LAND_TEST_GATE_SLEEP 2>/dev/null || true

  # Hermetic git identity/config (B73: zero writes outside fixture roots).
  export GIT_CONFIG_GLOBAL="$SANDBOX/gitconfig"
  export GIT_CONFIG_SYSTEM=/dev/null
  git config --file "$GIT_CONFIG_GLOBAL" user.name  "land-acceptance"
  git config --file "$GIT_CONFIG_GLOBAL" user.email "land-acceptance@local"
  git config --file "$GIT_CONFIG_GLOBAL" init.defaultBranch main
  git config --file "$GIT_CONFIG_GLOBAL" advice.detachedHead false

  REMOTE="$SANDBOX/origin.git"
  make_bare_remote "$REMOTE"
  seed_fixture
  AUDIT="$LAND_LOCK_DIR/audit.jsonl"
}

sandbox_teardown() {
  if [ -f "$SANDBOX/pids" ]; then
    while read -r p; do kill -9 "$p" 2>/dev/null; done < "$SANDBOX/pids"
  fi
  wait 2>/dev/null
  chmod -R u+rwX "$SANDBOX" 2>/dev/null
  rm -rf "$SANDBOX"
}

make_bare_remote() {
  git init -q --bare "$1"
  : > "$1/land-sandbox"   # B25 sandbox marker
}

# ── fixture seeding ──────────────────────────────────────────────────

seed_fixture() {
  SEED="$SANDBOX/seed"
  git init -q "$SEED"
  git -C "$SEED" checkout -qb main

  mkdir -p "$SEED/scripts/generators" "$SEED/scripts/gate.d" "$SEED/docs" \
           "$SEED/skills/seed-skill" "$SEED/skills/crank" "$SEED/.github/workflows"

  # Source-owned files
  cat > "$SEED/skills/seed-skill/SKILL.md" <<'EOF'
---
name: seed-skill
---
# seed-skill
seed body
EOF
  cat > "$SEED/skills/crank/SKILL.md" <<'EOF'
---
name: crank
---
# crank
line-1
line-2
line-3
EOF
  echo "# hand-authored doc" > "$SEED/docs/HAND.md"
  cat > "$SEED/docs/COUNTS.md" <<'EOF'
# Counts
This repo has <!-- count:skills -->0<!-- /count --> skills.
EOF
  printf '_beads/\n' > "$SEED/.gitignore"

  # Regen manifest (B42) + count-docs manifest (B48)
  cat > "$SEED/scripts/regen-manifest.txt" <<'EOF'
registry.json
docs/context-map.md
docs/SKILL-TIERS.md
skills-codex/
EOF
  printf 'docs/COUNTS.md\n' > "$SEED/scripts/count-docs.txt"

  # Mini generators — deterministic
  cat > "$SEED/scripts/generators/10-registry.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
ls skills | sort | jq -R . | jq -s '{skills: map({name:.})}' > registry.json
EOF
  cat > "$SEED/scripts/generators/20-context-map.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
{ echo "# Context map"; for d in skills/*/; do echo "- $(basename "$d")"; done; } > docs/context-map.md
EOF
  cat > "$SEED/scripts/generators/30-tiers.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
{ echo "# SKILL TIERS"; for d in skills/*/; do echo "| $(basename "$d") | T1 |"; done; } > docs/SKILL-TIERS.md
EOF
  cat > "$SEED/scripts/generators/40-codex.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
mkdir -p skills-codex
for c in skills-codex/*/; do
  [ -d "$c" ] || continue
  [ -d "skills/$(basename "$c")" ] || rm -rf "$c"
done
for d in skills/*/; do
  n="$(basename "$d")"
  mkdir -p "skills-codex/$n"
  h="$(git hash-object "skills/$n/SKILL.md")"
  printf '{"generated_hash": "%s"}\n' "$h" > "skills-codex/$n/.agentops-generated.json"
done
ls skills | sort | jq -R . | jq -s '{entries: .}' > skills-codex/.agentops-manifest.json
EOF
  cat > "$SEED/scripts/generators/50-counts.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
n="$(ls skills | wc -l | tr -d ' ')"
while read -r doc; do
  [ -f "$doc" ] || continue
  perl -0pi -e "s|(<!-- count:skills -->).*?(<!-- /count -->)|\${1}${n}\${2}|gs" "$doc"
done < scripts/count-docs.txt
EOF

  cat > "$SEED/scripts/regen-all.sh" <<'EOF'
#!/usr/bin/env bash
# Fixture mini regen battery. --check = regenerate, fail on drift, restore.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
run_all() { for g in scripts/generators/*.sh; do bash "$g"; done; }
if [ "${1:-}" = "--check" ]; then
  run_all
  drift="$(git status --porcelain -- $(tr '\n' ' ' < scripts/regen-manifest.txt))"
  if [ -n "$drift" ]; then
    echo "regen drift:" >&2; echo "$drift" >&2
    git checkout -q -- $(tr '\n' ' ' < scripts/regen-manifest.txt) 2>/dev/null || true
    git clean -qfd -- $(tr '\n' ' ' < scripts/regen-manifest.txt) 2>/dev/null || true
    exit 1
  fi
  exit 0
fi
run_all
EOF
  chmod +x "$SEED"/scripts/regen-all.sh "$SEED"/scripts/generators/*.sh

  # CI gate-family declaration for the B49 parity check.
  cat > "$SEED/.github/workflows/validate.yml" <<'EOF'
# fixture CI gate
# land-gate-families: regen-check json-verify counts gate.d
jobs: {}
EOF

  install_sut "$SEED"

  git -C "$SEED" add -A
  git -C "$SEED" commit -qm "seed: fixture repo"
  ( cd "$SEED" && bash scripts/regen-all.sh )
  git -C "$SEED" add -A
  git -C "$SEED" commit -qm "seed: derived surfaces"
  git -C "$SEED" remote add origin "$REMOTE"
  git -C "$SEED" push -q origin main
}

# Copy the system under test into the fixture. When scripts/land.sh does not
# exist yet (Phase 2 red), a loud not-implemented placeholder is installed so
# every scenario fails with a single unambiguous reason.
install_sut() {
  local dest="$1"
  mkdir -p "$dest/scripts"
  if [ -f "$REPO_ROOT/scripts/land.sh" ]; then
    cp "$REPO_ROOT/scripts/land.sh" "$dest/scripts/land.sh"
  else
    cat > "$dest/scripts/land.sh" <<'EOF'
#!/usr/bin/env bash
echo "land.sh: NOT IMPLEMENTED — bdd-foundry Phase 2 red placeholder" >&2
exit 97
EOF
  fi
  chmod +x "$dest/scripts/land.sh"
}

# ── lanes ────────────────────────────────────────────────────────────

# new_lane <name> [branch] → echoes the lane dir; creates branch off main.
new_lane() {
  local name="$1" branch="${2:-}"
  local lane="$SANDBOX/lanes/$name"
  git clone -q "$REMOTE" "$lane"
  if [ -n "$branch" ]; then git -C "$lane" checkout -qb "$branch"; fi
  echo "$lane"
}

add_skill() { # lane skill-name [msg]
  local lane="$1" n="$2" msg="${3:-feat: add skill $2}"
  mkdir -p "$lane/skills/$n"
  printf -- '---\nname: %s\n---\n# %s\nbody\n' "$n" "$n" > "$lane/skills/$n/SKILL.md"
  git -C "$lane" add -A
  git -C "$lane" commit -qm "$msg"
}

commit_file() { # lane path content msg
  local lane="$1" path="$2" content="$3" msg="$4"
  mkdir -p "$lane/$(dirname "$path")"
  printf '%s\n' "$content" > "$lane/$path"
  git -C "$lane" add -A
  git -C "$lane" commit -qm "$msg"
}

# ── running the SUT ──────────────────────────────────────────────────

land() { # lane [args...] — run scripts/land.sh inside the lane
  local lane="$1"; shift
  ( cd "$lane" && scripts/land.sh "$@" )
}

start_land() { # tag lane [args...] — background land; output in $SANDBOX/out/<tag>
  local tag="$1" lane="$2"; shift 2
  ( cd "$lane" && scripts/land.sh "$@" ) > "$SANDBOX/out/$tag" 2>&1 &
  local pid=$!
  echo "$pid" >> "$SANDBOX/pids"
  eval "PID_$tag=$pid"
}

wait_land() { # tag → sets ST_<tag>
  local tag="$1" pidvar="PID_$1" st=0
  wait "${!pidvar}" || st=$?
  eval "ST_$tag=$st"
}

out_of() { cat "$SANDBOX/out/$1"; }

status_json() { # lane → stdout
  ( cd "$1" && scripts/land.sh --status --json )
}

# ── shared state helpers ─────────────────────────────────────────────

remote_main_sha() { git --git-dir="$REMOTE" rev-parse refs/heads/main 2>/dev/null || echo ABSENT; }

remote_ref_sha() { git --git-dir="$REMOTE" rev-parse "refs/$1" 2>/dev/null || echo ABSENT; }

# fresh clone of the fixture remote (B52 discipline) → echoes dir
fresh_clone() {
  local d
  d="$(mktemp -d "$SANDBOX/fresh.XXXXXX")"
  git clone -q "$REMOTE" "$d/c"
  echo "$d/c"
}

fresh_clone_regen_check() { # exit status of regen-all.sh --check on a fresh clone
  local c; c="$(fresh_clone)"
  ( cd "$c" && bash scripts/regen-all.sh --check )
}

remote_patch_ids() {
  local c; c="$(fresh_clone)"
  git -C "$c" log --format=%H main | while read -r sha; do
    git -C "$c" show "$sha" | git -C "$c" patch-id --stable | awk '{print $1}'
  done
}

audit_entries() { [ -f "$AUDIT" ] && cat "$AUDIT" || true; }

audit_count() { # [type-filter]
  if [ -n "${1:-}" ]; then audit_entries | grep -c "\"$1\"" || true
  else audit_entries | wc -l | tr -d ' '; fi
}

# ── lock fabrication / live holders ──────────────────────────────────

fabricate_lock() { # pid heartbeat_age_seconds [id]
  local pid="$1" age="$2" id="${3:-testhost:/dead/worktree:$1:111}"
  local now; now="$(date +%s)"
  printf '{"id":"%s","pid":%s,"start_time":111,"heartbeat":%s,"nonce":"deadbeef"}\n' \
    "$id" "$pid" "$((now - age))" > "$LAND_LOCK_DIR/lock.json"
}

# A live, heartbeating foreign holder (not a real land — pure lock occupancy).
live_holder_start() {
  (
    while :; do
      now="$(date +%s)"
      printf '{"id":"testhost:/live/worktree:%s:222","pid":%s,"start_time":222,"heartbeat":%s,"nonce":"cafe0001"}\n' \
        "$BASHPID" "$BASHPID" "$now" > "$LAND_LOCK_DIR/lock.json.tmp"
      mv "$LAND_LOCK_DIR/lock.json.tmp" "$LAND_LOCK_DIR/lock.json"
      sleep 0.5
    done
  ) &
  HOLDER_PID=$!
  echo "$HOLDER_PID" >> "$SANDBOX/pids"
  sleep 0.7   # let the first heartbeat land
}

live_holder_stop() {
  kill -9 "$HOLDER_PID" 2>/dev/null || true
  wait "$HOLDER_PID" 2>/dev/null || true
  rm -f "$LAND_LOCK_DIR/lock.json"
}

# ── polling ──────────────────────────────────────────────────────────

wait_until() { # timeout_seconds cmd...
  local t="$1"; shift
  local deadline=$(( $(date +%s) + t ))
  until "$@"; do
    [ "$(date +%s)" -ge "$deadline" ] && return 1
    sleep 0.2
  done
}

lock_state_is() { # lane state
  status_json "$1" 2>/dev/null | jq -e --arg s "$2" '.state == $s' > /dev/null 2>&1
}

queue_len_is() { # lane n
  status_json "$1" 2>/dev/null | jq -e --argjson n "$2" '(.queue | length) == $n' > /dev/null 2>&1
}

# ── misc assertions ──────────────────────────────────────────────────

worktree_clean() { [ -z "$(git -C "$1" status --porcelain)" ]; }

git_dir_state_snapshot() { # lane → snapshot of refs + in-progress markers
  git -C "$1" for-each-ref --format='%(refname) %(objectname)'
  for f in rebase-merge rebase-apply MERGE_HEAD CHERRY_PICK_HEAD; do
    [ -e "$1/.git/$f" ] && echo "present:$f"
  done
  true
}
