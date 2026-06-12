#!/usr/bin/env bats
# §B Bead repair — B77 (judge amendment 2).
# Reads the live bead from the MAIN checkout (never a worktree) and proves its
# ACCEPTANCE is a runnable B62 filter, not a prose recipe.

setup() {
  load helpers2
}

@test "B77: ag-d3-fixture-guard-yk7rq carries a runnable B62 acceptance, not a prose recipe" {
  body="$(br_show ag-d3-fixture-guard-yk7rq)"
  [ -n "$body" ]
  acceptance="$(printf '%s\n' "$body" | bead_section ACCEPTANCE)"
  [ -n "$acceptance" ]

  # an executable bats filter command of the pinned form, anchored on ^B62
  cmd_line="$(printf '%s\n' "$acceptance" \
    | grep -E "bats[[:space:]][^[:space:]]*acceptance-tests[^']*-f[[:space:]]+['\"]\^B62" | head -1 || true)"
  [ -n "$cmd_line" ]

  # the old B25-smoke proxy is gone from the done-criterion
  ! printf '%s\n' "$acceptance" | grep -q '\^B25'

  # no manual prose steps as the operative done-criterion
  ! printf '%s\n' "$acceptance" | grep -Eiq '\b(manually|by hand|eyeball)\b'
  ! printf '%s\n' "$acceptance" | grep -Eiq '^[[:space:]]*(Then[[:space:]])?inspect\b'

  # the filtered test exercises the fixture-guard behavior ITSELF: a raw push
  # without a valid LAND_PUSH_NONCE rejected, a push with it accepted
  b62_file="$(grep -l '@test "B62' "$BASE_SUITE_DIR"/*.bats | head -1)"
  [ -n "$b62_file" ]
  b62_body="$(sed -n '/@test "B62/,/^}/p' "$b62_file")"
  printf '%s\n' "$b62_body" | grep -q 'LAND_PUSH_NONCE'
  printf '%s\n' "$b62_body" | grep -q 'git.*push'

  # the exact command is copy-paste runnable from the repo root
  cmd="$(printf '%s\n' "$acceptance" | grep -oE '`[^`]*bats[^`]*\^B62[^`]*`' | head -1 | tr -d '\140' || true)"
  [ -n "$cmd" ] || cmd="$(printf '%s\n' "$cmd_line" \
    | grep -oE "bats[[:space:]][^[:space:]]*acceptance-tests[^']*'[^']*B62[^']*'" | head -1 || true)"
  [ -n "$cmd" ]
  run bash -c "cd '$REAL_REPO_ROOT' && $cmd"
  # only legal outcomes: red-on-assertion (pre-implementation) or green
  # (post-implementation) — never a harness crash, never zero selected tests
  [[ "$output" != *"No such file or directory"* ]]
  printf '%s\n' "$output" | grep -Eq '1\.\.[1-9]|[1-9][0-9]* test'
}
