#!/usr/bin/env bats
# Unit tests for planner intake's MECHANICAL half — membrane/scaffold-quest.sh +
# quests/_template/. Proves the scaffold is correct-by-construction WITHOUT a live
# planner drill (that is bead .4's job): a good slug yields a well-formed,
# default-FAIL quest; a malformed/ambiguous ask path is refused (BLOCKED), never
# guessed into a bad scaffold.
#
# What "well-formed" means here (the contract the planner relies on):
#   * CONTRACT.md carries >=2 numbered default-FAIL clauses (`^N. [ ]`)
#   * test.sh is executable and exits NONZERO against the placeholder impl
#   * no residual {{TEMPLATE}} tokens leak into an instantiated quest
#   * the quest is a git repo whose `main` carries CONTRACT.md (the ruler the
#     close gate reads via `git show main:CONTRACT.md`)

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  SCAFFOLD="$PACK/membrane/scaffold-quest.sh"
  ROOT="$BATS_TEST_TMPDIR/quests"
  mkdir -p "$ROOT"
  chmod +x "$SCAFFOLD" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Well-formed scaffold (the happy path)
# ---------------------------------------------------------------------------

@test "scaffold: good slug produces a well-formed quest dir (exit 0)" {
  run "$SCAFFOLD" foo --root "$ROOT" --ask "foo must add and max"
  [ "$status" -eq 0 ]
  [[ "$output" == SCAFFOLDED* ]]
  [ -f "$ROOT/foo/CONTRACT.md" ]
  [ -f "$ROOT/foo/test.sh" ]
  [ -f "$ROOT/foo/impl.sh" ]
  # the template's own README must NOT be carried into an instantiated quest
  [ ! -f "$ROOT/foo/README.md" ]
}

@test "scaffold: CONTRACT.md carries >=2 numbered default-FAIL clauses" {
  "$SCAFFOLD" foo --root "$ROOT" >/dev/null
  local n
  n="$(grep -cE '^[0-9]+\. \[ \]' "$ROOT/foo/CONTRACT.md")"
  [ "$n" -ge 2 ]
}

@test "scaffold: test.sh is executable and exits NONZERO on the placeholder impl" {
  "$SCAFFOLD" foo --root "$ROOT" >/dev/null
  [ -x "$ROOT/foo/test.sh" ]
  [ -x "$ROOT/foo/impl.sh" ]
  run bash -c "cd '$ROOT/foo' && ./test.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *FAILING* ]]
}

@test "scaffold: no residual template tokens leak into the quest" {
  "$SCAFFOLD" foo --root "$ROOT" --ask "some ask" >/dev/null
  run grep -REn '\{\{(QUEST|SLUG|ASK)\}\}' "$ROOT/foo"
  [ "$status" -ne 0 ]   # grep finds nothing -> nonzero
}

@test "scaffold: --ask text is substituted verbatim into CONTRACT.md" {
  "$SCAFFOLD" foo --root "$ROOT" --ask "calc.sh must support add and max" >/dev/null
  run grep -F "calc.sh must support add and max" "$ROOT/foo/CONTRACT.md"
  [ "$status" -eq 0 ]
}

@test "scaffold: quest is a git repo whose main carries CONTRACT.md (the ruler)" {
  "$SCAFFOLD" foo --root "$ROOT" >/dev/null
  [ "$(git -C "$ROOT/foo" rev-parse --abbrev-ref HEAD)" = "main" ]
  run git -C "$ROOT/foo" show main:CONTRACT.md
  [ "$status" -eq 0 ]
  [[ "$output" == *"Acceptance Contract"* ]]
}

@test "scaffold: --no-git yields the files without a git repo" {
  run "$SCAFFOLD" foo --root "$ROOT" --no-git
  [ "$status" -eq 0 ]
  [ -f "$ROOT/foo/CONTRACT.md" ]
  [ ! -d "$ROOT/foo/.git" ]
}

# ---------------------------------------------------------------------------
# Malformed / ambiguous ask path is REFUSED (BLOCKED), never guessed
# ---------------------------------------------------------------------------

@test "malformed: a bad slug is BLOCKED, not scaffolded (exit 3)" {
  run "$SCAFFOLD" "Bad_Slug" --root "$ROOT"
  [ "$status" -eq 3 ]
  [[ "$output" == BLOCKED*bad_slug* ]]
  [ ! -d "$ROOT/Bad_Slug" ]
}

@test "malformed: the reserved _template slug is BLOCKED (exit 3)" {
  run "$SCAFFOLD" _template --root "$ROOT"
  [ "$status" -eq 3 ]
  [[ "$output" == BLOCKED* ]]
}

@test "malformed: no slug at all is a usage error (exit 2)" {
  run "$SCAFFOLD" --root "$ROOT"
  [ "$status" -eq 2 ]
  [[ "$output" == *usage* ]]
}

@test "idempotency: scaffolding over an existing quest is fail-closed (exit 3), impl untouched" {
  "$SCAFFOLD" foo --root "$ROOT" >/dev/null
  # simulate a builder having implemented impl.sh
  printf '#!/usr/bin/env bash\necho REAL_WORK\n' > "$ROOT/foo/impl.sh"
  run "$SCAFFOLD" foo --root "$ROOT"
  [ "$status" -eq 3 ]
  [[ "$output" == BLOCKED*quest_exists* ]]
  # the existing impl must be untouched — scaffold NEVER edits impl code
  run grep -F "REAL_WORK" "$ROOT/foo/impl.sh"
  [ "$status" -eq 0 ]
}
