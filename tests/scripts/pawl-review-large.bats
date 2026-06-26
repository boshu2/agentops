#!/usr/bin/env bats
# build_review_body (age-mwhj): the pawl review packet inlines the full diff, which times the
# review out + fails closed on large diffs (a 41KB packet timed an opus pane out; a 62KB cold
# codex was killed). Above a byte cap, the body switches to READ-FILES-NOT-INLINE (size note +
# git --stat + changed-file absolute paths) so the reviewer reads the files instead of choking.
# build_review_body is pure (caller passes the stat + file list), so the threshold logic is
# locked here without running codex.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl-review.sh"   # source-guard returns before the codex flow
}

@test "build_review_body: a diff at/below the cap is returned verbatim (inline, unchanged)" {
  run build_review_body "diff --git a/x b/x
+small change" 24576 "STAT" "x" "/repo"
  [ "$status" -eq 0 ]
  [ "$output" = "diff --git a/x b/x
+small change" ]
}

@test "build_review_body: a diff over the cap -> read-files body (note + stat + abs paths), NOT the inline diff" {
  # A realistic large diff: a removed line + a context line, then 200 ADDED lines (to be elided).
  added="$(printf '+added line %s\n' $(seq 1 200))"
  diff="$(printf 'diff --git a/x b/x\n@@ -1,2 +1,200 @@\n-removed old line\n context kept\n%s' "$added")"
  run build_review_body "$diff" 10 "STATBLOCK-xyz" "scripts/pawl.sh
tests/scripts/pawl.bats" "/abs/repo"
  [ "$status" -eq 0 ]
  # switched to read-files: note + stat + the absolute file paths
  [[ "$output" == *"NOT inlined"* ]]
  [[ "$output" == *"READ THE CHANGED FILES DIRECTLY"* ]]
  [[ "$output" == *"STATBLOCK-xyz"* ]]
  [[ "$output" == *"/abs/repo/scripts/pawl.sh"* ]]
  [[ "$output" == *"/abs/repo/tests/scripts/pawl.bats"* ]]
  # DELETIONS + structure are PRESERVED (cannot be recovered by reading the current file)
  [[ "$output" == *"-removed old line"* ]]
  [[ "$output" == *"@@ -1,2 +1,200 @@"* ]]
  # ADDED content is ELIDED (the reviewer reads it from the files)
  [[ "$output" != *"+added line 1"* ]]
  [[ "$output" != *"+added line 200"* ]]
  # far smaller than inlining all 200 added lines
  [ "${#output}" -lt "${#diff}" ]
}

@test "build_review_body: read-files body lists EVERY changed file (none dropped)" {
  big="$(printf 'x%.0s' $(seq 1 100))"
  run build_review_body "$big" 10 "stat" "a.go
b.go
c.go" "/r"
  for f in a.go b.go c.go; do
    [[ "$output" == *"/r/$f"* ]]
  done
}

@test "build_review_body: an empty file list over the cap still emits the note + stat (no crash)" {
  big="$(printf 'x%.0s' $(seq 1 100))"
  run build_review_body "$big" 10 "STATONLY" "" "/r"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NOT inlined"* ]]
  [[ "$output" == *"STATONLY"* ]]
}

# --- pawl_tier_note (age-bb5l): the achieved tier surfaced honestly, no hardcoded "opus+codex" ---

@test "pawl_tier_note: multi-model -> cross-family phrase" {
  run pawl_tier_note multi-model
  [ "$output" = "multi-model cross-family" ]
}

@test "pawl_tier_note: fresh-context -> single-family WEAKER nudge (add a 2nd family)" {
  run pawl_tier_note fresh-context
  [[ "$output" == *"SINGLE family"* ]]
  [[ "$output" == *"WEAKER"* ]]
  [[ "$output" == *"add codex or agy"* ]]
  # the lie this fixes: a fresh route must NOT be described as the opus+codex duel
  [[ "$output" != *"opus+codex"* ]]
}

@test "pawl_tier_note: unknown/empty mode -> a safe non-empty label (never claims cross-family)" {
  [ "$(pawl_tier_note '')" = "unknown-tier" ]
  [[ "$(pawl_tier_note multi-model)" != *"fresh"* ]]
}
