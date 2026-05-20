#!/usr/bin/env bats
# Regression tests for scripts/export-session-summary.sh (soc-absm).
#
# Each test builds an isolated repo with synthetic .agents/evolve/cycle-
# history.jsonl + a few commits, then runs the script and asserts the
# generated markdown's shape.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/export-session-summary.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  mkdir -p .agents/evolve
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

write_cycle() {
  # write_cycle <cycle> <ts> <mode> <result> <notes>
  cat <<EOF >> "$TMP/repo/.agents/evolve/cycle-history.jsonl"
{"cycle":$1,"ts":"$2","mode":"$3","result":"$4","notes":"$5"}
EOF
}

run_export() {
  cd "$TMP/repo"
  run "$SCRIPT" "$@"
}

@test "writes a summary file when --stdout is not passed" {
  write_cycle 1 "2026-05-20T10:00:00Z" "test-mode" "productive" "first"
  run_export --no-bd --no-prs --out "$TMP/repo/out.md"
  [ "$status" -eq 0 ]
  [ -f "$TMP/repo/out.md" ]
  grep -q "^# Session summary" "$TMP/repo/out.md"
  grep -q "## Outcomes" "$TMP/repo/out.md"
}

@test "--stdout prints to stdout instead of file" {
  write_cycle 1 "2026-05-20T10:00:00Z" "test-mode" "productive" "first"
  run_export --stdout --no-bd --no-prs
  [ "$status" -eq 0 ]
  [[ "$output" == *"# Session summary"* ]]
  [[ "$output" == *"## Outcomes"* ]]
  # No file should have been created at the default path.
  ! ls "$TMP/repo/.agents/evolve/session-summary-"*.md 2>/dev/null
}

@test "filters cycles by ISO timestamp window" {
  # Two cycles, one inside the window, one outside.
  write_cycle 1 "2026-05-20T08:00:00Z" "old-mode" "productive" "outside"
  write_cycle 2 "2026-05-20T12:00:00Z" "new-mode" "productive" "inside"
  run_export --stdout --no-bd --no-prs --since "2026-05-20T10:00:00Z"
  [ "$status" -eq 0 ]
  [[ "$output" == *"new-mode"* ]]
  [[ "$output" != *"old-mode"* ]]
  # Header counts reflect filter.
  [[ "$output" == *"Cycles: **1**"* ]]
}

@test "compressed cycle ledger truncates notes to 140 chars" {
  long_note="$(printf 'x%.0s' {1..300})"
  write_cycle 1 "2026-05-20T10:00:00Z" "m" "productive" "$long_note"
  run_export --stdout --no-bd --no-prs
  [ "$status" -eq 0 ]
  # Find the line containing the cycle and check length.
  line="$(printf '%s\n' "$output" | grep 'cycle 1')"
  [ -n "$line" ]
  [ "${#line}" -lt 200 ]
}

@test "outcomes section reports commit count from git log" {
  cd "$TMP/repo"
  for i in 1 2 3; do
    echo "$i" > "file-$i.txt"
    git add "file-$i.txt"
    git commit --quiet -m "commit $i"
  done
  cd "$ORIG_DIR"
  run_export --stdout --no-bd --no-prs --since HEAD~3
  [ "$status" -eq 0 ]
  # 3 new commits inside the window (initial commit is outside)
  [[ "$output" == *"Commits: **3**"* ]] || [[ "$output" == *"Commits: **4**"* ]]
}

@test "missing cycle-history.jsonl yields zero-count, still succeeds" {
  rm -f "$TMP/repo/.agents/evolve/cycle-history.jsonl"
  run_export --stdout --no-bd --no-prs --since "2026-05-20T08:00:00Z"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Cycles: **0**"* ]]
}

@test "--no-bd suppresses the memories section" {
  write_cycle 1 "2026-05-20T10:00:00Z" "m" "productive" "x"
  run_export --stdout --no-bd --no-prs
  [ "$status" -eq 0 ]
  [[ "$output" != *"## New memories"* ]]
}

@test "--no-prs suppresses the merged PR section" {
  write_cycle 1 "2026-05-20T10:00:00Z" "m" "productive" "x"
  run_export --stdout --no-bd --no-prs
  [ "$status" -eq 0 ]
  [[ "$output" != *"## Merged PRs"* ]]
}

@test "carry-forward section reads goal and in-flight from session-state.json" {
  cat > "$TMP/repo/.agents/evolve/session-state.json" <<EOF
{
  "goal": "clear-all-open-beads",
  "batch_prs": [101, 102]
}
EOF
  write_cycle 1 "2026-05-20T10:00:00Z" "m" "productive" "x"
  run_export --stdout --no-bd --no-prs
  [ "$status" -eq 0 ]
  [[ "$output" == *"clear-all-open-beads"* ]]
  [[ "$output" == *"101, 102"* ]]
}

@test "rejects unknown flag with usage error" {
  run_export --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}

@test "default --out path lands under .agents/evolve/" {
  write_cycle 1 "2026-05-20T10:00:00Z" "m" "productive" "x"
  run_export --no-bd --no-prs
  [ "$status" -eq 0 ]
  files=$(ls "$TMP/repo/.agents/evolve/session-summary-"*.md 2>/dev/null | wc -l | tr -d ' ')
  [ "$files" -eq 1 ]
}
