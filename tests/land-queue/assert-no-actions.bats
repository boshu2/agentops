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
