#!/usr/bin/env bats
# agentops-2pl.11: land lane must never invoke GitHub Actions / PR / merge queue.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  LANE="$REPO_ROOT/scripts/land-lane-run.sh"
  GUARD="$REPO_ROOT/scripts/assert-no-actions.sh"

  [ -x "$SUBMIT" ] || skip "land-submit.sh missing"
  [ -x "$LANE" ] || skip "land-lane-run.sh missing"
  [ -x "$GUARD" ] || skip "assert-no-actions.sh missing"
  command -v jq >/dev/null 2>&1 || skip "jq required"

  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"

  cat >"$TMP/bin/gh" <<EOS
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >>"$TMP/real-gh.log"
if [[ "\${1:-}" == "run" && "\${2:-}" == "list" ]]; then
  exit 0
fi
echo "fake gh: \$*" >&2
exit 0
EOS
  chmod +x "$TMP/bin/gh"
  export PATH="$TMP/bin:$PATH"

  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  QUEUE_DIR="$REPO/.agents/land-queue"
  GATE_LOG="$TMP/gate-runs.log"

  git init --bare --quiet "$ORIGIN"
  git -C "$ORIGIN" symbolic-ref HEAD refs/heads/main

  mkdir -p "$REPO"
  git -C "$REPO" init --quiet
  git -C "$REPO" checkout --quiet -b main
  git -C "$REPO" config user.email "bats@test.local"
  git -C "$REPO" config user.name "bats-fixture"
  git -C "$REPO" config gc.auto 0
  git -C "$REPO" config maintenance.auto false
  printf 'base\n' >"$REPO/README.md"
  printf '.agents/\n' >"$REPO/.gitignore"

  mkdir -p "$REPO/scripts" "$REPO/schemas" "$REPO/.agents/pawl-verdicts" "$REPO/.git/hooks"
  cp "$REPO_ROOT/scripts/pawl-land.sh" "$REPO/scripts/pawl-land.sh"
  cp "$REPO_ROOT/scripts/pawl-verdict.sh" "$REPO/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/check-pawl-pre-push.sh" "$REPO/scripts/check-pawl-pre-push.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$REPO/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$REPO/scripts/"*.sh

  git -C "$REPO" add README.md .gitignore scripts schemas
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main

  cat >"$REPO/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
exec "$(git rev-parse --show-toplevel)/scripts/check-pawl-pre-push.sh"
EOS
  chmod +x "$REPO/.git/hooks/pre-push"

  EVIDENCE="$TMP/evidence.txt"
  printf 'fresh-context review evidence (no actions guard)\n' >"$EVIDENCE"

  cd "$REPO" || return 1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

queue_bead() {
  local bead="$1" file="$2" content="$3"
  git -C "$REPO" checkout --quiet main
  git -C "$REPO" checkout --quiet -B "work-$bead"
  printf '%s\n' "$content" >"$REPO/$file"
  git -C "$REPO" add "$file"
  git -C "$REPO" commit --quiet -m "fix(land-queue): guarded land $bead ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND=file LAND_AUTHOR_FAMILY=operator \
      bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

write_gate() {
  local mode="$1"
  GATE="$TMP/gate-$mode.sh"
  case "$mode" in
    good)
      cat >"$GATE" <<EOS
#!/usr/bin/env bash
set -euo pipefail
bead="\$1"
echo "gate-run \$bead" >>"$GATE_LOG"
head="\$(git -C "$REPO" rev-parse HEAD)"
bash "$REPO/scripts/pawl-verdict.sh" write "\$bead" 0 \\
  --disposition CONFIRMED --head "\$head" \\
  --author-context "author-operator-\$bead" --author-family operator \\
  --refuter "gpt:CONFIRMED:fresh-reviewer-\$bead:$EVIDENCE" \\
  --dir "$REPO/.agents/pawl-verdicts" >/dev/null
bash "$REPO/scripts/pawl-land.sh" "\$bead"
EOS
      ;;
    workflow)
      cat >"$GATE" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
gh workflow run validate.yml
EOS
      ;;
    pr)
      cat >"$GATE" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
gh pr create --title blocked --body blocked --base main
EOS
      ;;
    rerun)
      # The refuted hole (agentops-2pl.11): `gh run rerun` re-INVOKES a workflow
      # run but is NOT enumerated by the old blocklist, so it slipped through as
      # 'allowed'. Default-deny must BLOCK it.
      cat >"$GATE" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
gh run rerun 123
EOS
      ;;
    unknown)
      # An un-enumerated / future `gh run <verb>`: default-deny must BLOCK it so a
      # new verb cannot reopen the hole.
      cat >"$GATE" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
gh run frobnicate 123
EOS
      ;;
    *)
      return 1
      ;;
  esac
  chmod +x "$GATE"
}

run_lane() {
  ( cd "$REPO" && \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    AGENTOPS_NO_ACTIONS_LOG="$QUEUE_DIR/no-actions-guard.jsonl" \
    LAND_LANE_GATE_CMD="bash '$GATE'" \
    LAND_LANE_PAWL_LAND_SCRIPT="$REPO/scripts/pawl-land.sh" \
    bash "$LANE" --drain )
}

gh_run_count() {
  gh run list --limit 5 | wc -l | tr -d '[:space:]'
}

@test "startup guard passes for the real land scripts (no validate.yml config-assertion)" {
  # The guard asserts only that the LAND LANE invokes no Actions verbs. It does
  # NOT police validate.yml — the live workflow legitimately carries
  # pull_request/merge_group triggers for ordinary contributor PR CI, and `check`
  # must pass against it (the agentops-2pl.11 fix).
  run "$GUARD" check
  [ "$status" -eq 0 ]
  [[ "$output" == *"assert-no-actions: PASS"* ]]

  # Sanity: the live validate.yml DOES carry pull_request/merge_group, and the
  # guard tolerates it (regression guard for the dropped config-assertion).
  grep -Eq '^[[:space:]]{2}pull_request:' "$REPO_ROOT/.github/workflows/validate.yml"
  grep -Eq '^[[:space:]]{2}merge_group:' "$REPO_ROOT/.github/workflows/validate.yml"
}

@test "static guard rejects an Actions-invoking gh verb planted in a land script" {
  lane="$TMP/land-lane-run.sh"
  submit="$TMP/land-submit.sh"
  cp "$REPO_ROOT/scripts/land-lane-run.sh" "$lane"
  cp "$REPO_ROOT/scripts/land-submit.sh" "$submit"
  printf '\ngh workflow run validate.yml\n' >>"$lane"

  run "$GUARD" check --land-lane "$lane" --land-submit "$submit"
  [ "$status" -eq 1 ]
  [[ "$output" == *"forbidden gh workflow dispatch"* ]]
}

@test "static guard rejects gh pr create planted in a land script" {
  lane="$TMP/land-lane-run.sh"
  submit="$TMP/land-submit.sh"
  cp "$REPO_ROOT/scripts/land-lane-run.sh" "$lane"
  cp "$REPO_ROOT/scripts/land-submit.sh" "$submit"
  printf '\ngh pr create --title x --body y --base main\n' >>"$submit"

  run "$GUARD" check --land-lane "$lane" --land-submit "$submit"
  [ "$status" -eq 1 ]
  [[ "$output" == *"forbidden gh PR mutation"* ]]
}

@test "guarded local landing leaves gh run list count unchanged" {
  write_gate good
  queue_bead age-noactions-good guarded.txt guarded

  before="$(gh_run_count)"
  run run_lane
  echo "$output"
  [ "$status" -eq 0 ]
  after="$(gh_run_count)"

  [ "$before" = "0" ]
  [ "$after" = "$before" ]
  git -C "$ORIGIN" cat-file -e refs/heads/main:guarded.txt
  [ ! -s "$QUEUE_DIR/no-actions-guard.jsonl" ]
}

@test "runtime gh shim rejects gh workflow run from inside the land path" {
  write_gate workflow
  queue_bead age-noactions-workflow blocked-workflow.txt blocked

  run run_lane
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"BLOCKED (assert-no-actions): gh workflow run validate.yml"* ]]

  [ -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-noactions-workflow") | .status' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "dead-letter" ]

  run jq -r 'select(.status == "blocked") | .argv' "$QUEUE_DIR/no-actions-guard.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "gh workflow run validate.yml" ]
  ! grep -q 'workflow run' "$TMP/real-gh.log"
}

@test "runtime gh shim rejects gh pr create from inside the land path" {
  write_gate pr
  queue_bead age-noactions-pr blocked-pr.txt blocked

  run run_lane
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"BLOCKED (assert-no-actions): gh pr create"* ]]

  [ -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-noactions-pr") | .status' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "dead-letter" ]

  run jq -r 'select(.status == "blocked") | .argv' "$QUEUE_DIR/no-actions-guard.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == gh\ pr\ create* ]]
  ! grep -q 'pr create' "$TMP/real-gh.log"
}

@test "runtime gh shim rejects gh run rerun from inside the land path (the refuted hole)" {
  # agentops-2pl.11 reviewer hole: `gh run rerun 123` re-INVOKES a workflow run
  # (which runs GitHub Actions) yet was logged 'allowed' and delegated to the real
  # gh under the old BLOCKLIST. Default-deny must BLOCK it and never reach real gh.
  write_gate rerun
  queue_bead age-noactions-rerun blocked-rerun.txt blocked

  run run_lane
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"BLOCKED (assert-no-actions): gh run rerun 123"* ]]

  [ -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-noactions-rerun") | .status' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "dead-letter" ]

  run jq -r 'select(.status == "blocked") | .argv' "$QUEUE_DIR/no-actions-guard.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "gh run rerun 123" ]
  # The real gh was NEVER invoked for the rerun (it would have re-run Actions).
  ! grep -q 'run rerun' "$TMP/real-gh.log"
}

@test "runtime gh shim rejects an UNKNOWN gh run subcommand (default-deny posture)" {
  # A future / un-enumerated `gh run <verb>` must be DENIED by default so a new
  # verb cannot reopen the hole. This is the structural guarantee of the flip from
  # blocklist to allowlist.
  write_gate unknown
  queue_bead age-noactions-unknown blocked-unknown.txt blocked

  run run_lane
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"BLOCKED (assert-no-actions): gh run frobnicate 123"* ]]

  run jq -r 'select(.bead == "age-noactions-unknown") | .status' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "dead-letter" ]

  run jq -r 'select(.status == "blocked") | .argv' "$QUEUE_DIR/no-actions-guard.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "gh run frobnicate 123" ]
  ! grep -q 'run frobnicate' "$TMP/real-gh.log"
}

@test "static guard rejects gh run rerun planted in a land script" {
  # Defense-in-depth: the static scan now also knows the Actions-invoking
  # gh run verbs (the runtime default-deny is the real backstop).
  lane="$TMP/land-lane-run.sh"
  submit="$TMP/land-submit.sh"
  cp "$REPO_ROOT/scripts/land-lane-run.sh" "$lane"
  cp "$REPO_ROOT/scripts/land-submit.sh" "$submit"
  printf '\ngh run rerun 123\n' >>"$lane"

  run "$GUARD" check --land-lane "$lane" --land-submit "$submit"
  [ "$status" -eq 1 ]
  [[ "$output" == *"forbidden gh run mutation"* ]]
}

# --------------------------------------------------------------------------- #
# Direct guard-gh allow/block matrix — exercises the DEFAULT-DENY allowlist at
# the unit level (no land lane). real-gh is a delegate stub that prints when
# invoked, so "delegated" (allowed) vs "blocked" is unambiguous.
# --------------------------------------------------------------------------- #
setup_guard_unit() {
  REAL_GH="$TMP/real-gh-stub"
  GUARD_LOG="$TMP/guard-unit.log"
  cat >"$REAL_GH" <<'EOS'
#!/usr/bin/env bash
echo "DELEGATED: $*"
exit 0
EOS
  chmod +x "$REAL_GH"
  : >"$GUARD_LOG"
}

# Assert the given gh args are ALLOWED: delegated to real gh, exit 0, logged allowed.
assert_allowed() {
  run "$GUARD" guard-gh "$GUARD_LOG" "$REAL_GH" -- "$@"
  echo "args: $*"
  echo "out: $output"
  [ "$status" -eq 0 ]
  [[ "$output" == DELEGATED:* ]]
}

# Assert the given gh args are BLOCKED: hard-error exit 2, never delegated, logged blocked.
assert_blocked() {
  run "$GUARD" guard-gh "$GUARD_LOG" "$REAL_GH" -- "$@"
  echo "args: $*"
  echo "out: $output"
  [ "$status" -eq 2 ]
  [[ "$output" == *"BLOCKED (assert-no-actions):"* ]]
  [[ "$output" != DELEGATED:* ]]
}

@test "guard-gh ALLOWS read-only gh subcommands (delegated to real gh)" {
  setup_guard_unit
  assert_allowed run view 123
  assert_allowed run list
  assert_allowed run watch 1
  assert_allowed pr view 5
  assert_allowed pr list
  assert_allowed pr checks 5
  assert_allowed pr diff 5
  assert_allowed api repos/o/r
  assert_allowed api -X GET repos/o/r
  assert_allowed api --method GET repos/o/r
  assert_allowed auth status
  assert_allowed --version
  assert_allowed workflow list
  assert_allowed workflow view validate.yml

  # Every allowed call delegated, none blocked.
  run jq -rs 'map(.status) | unique | join(",")' "$GUARD_LOG"
  [ "$output" = "allowed" ]
}

@test "guard-gh BLOCKS Actions-invoking / mutating gh paths (default-deny)" {
  setup_guard_unit
  # The exact refuted hole, plus the full default-deny matrix.
  assert_blocked run rerun 123
  assert_blocked run cancel 123
  assert_blocked run delete 123
  assert_blocked run madeup 123          # UNKNOWN run verb → denied by default
  assert_blocked workflow run validate.yml
  assert_blocked workflow enable validate.yml
  assert_blocked workflow disable validate.yml
  assert_blocked pr create --title x --base main
  assert_blocked pr merge --auto 5       # merge-queue enqueue
  assert_blocked api -X POST repos/o/r/dispatches
  assert_blocked api --method PUT repos/o/r/x
  assert_blocked api repos/o/r/actions/runs/1/rerun
  assert_blocked api repos/o/r -f field=value   # request body → write
  # The agentops-2pl.11 equals-form hole: body/method flags in `--flag=value` form
  # were delegated to real gh, letting a body-bearing `gh api` POST to a dispatch
  # endpoint. Default-deny must BLOCK them in BOTH the space and equals forms.
  assert_blocked api --field=x=y repos/o/r       # equals-form body flag → write
  assert_blocked api --raw-field=x=y repos/o/r   # equals-form raw body → write
  assert_blocked api --input=f.json repos/o/r    # equals-form input file → write
  assert_blocked api --method=POST repos/o/r     # equals-form method override → write
  # Actions/workflow dispatch endpoint is blocked regardless of method form.
  assert_blocked api repos/o/r/actions/workflows/x/dispatches
  assert_blocked frobnicate the widget    # totally unknown verb → denied

  # Every blocked call logged blocked; the real gh stub was NEVER delegated to.
  run jq -rs 'map(.status) | unique | join(",")' "$GUARD_LOG"
  [ "$output" = "blocked" ]
}
