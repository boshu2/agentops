#!/usr/bin/env bats
# §D Cutover: doctrine flip — B86.
# Read-only against the operator checkout; the mutating negative runs in a
# disposable clone (B92 hermetic contract).

setup() {
  load helpers2
}

@test "B86: landing doctrine flips repo-wide, not just CLAUDE.md" {
  # CLAUDE.md's Workflow Land phase instructs scripts/land.sh
  phase_line="$(grep -E '^4\. \*\*Land\.' "$REAL_REPO_ROOT/CLAUDE.md" || true)"
  [ -n "$phase_line" ]
  [[ "$phase_line" == *"scripts/land.sh"* ]]

  # ...and the Branch+PR-shape Land row does too
  row="$(grep -E '^\| Land \|' "$REAL_REPO_ROOT/CLAUDE.md" || true)"
  [ -n "$row" ]
  [[ "$row" == *"scripts/land.sh"* ]]

  # a checked-in operator-doc sweep script exists and covers the pinned list
  [ -x "$REAL_REPO_ROOT/$V_DOCTRINE" ]
  for d in CLAUDE.md AGENTS.md AGENTS-WORKFLOW.md AGENTS-CI.md AGENTS-CODEX.md \
           AGENTS-RUNTIME.md README.md docs/agent-workflow-reference.md; do
    grep -q "$d" "$REAL_REPO_ROOT/$V_DOCTRINE"
  done

  # the historical/superseded marker convention is documented in the sweep header
  head -40 "$REAL_REPO_ROOT/$V_DOCTRINE" | grep -Eiq 'historical|superseded'

  # the sweep passes on the cutover commit (no live direct-push instruction
  # survives outside explicitly historical/superseded sections)
  clone="$(real_repo_clone)"
  run bash -c "cd '$clone' && $V_DOCTRINE"
  [ "$status" -eq 0 ]

  # the old operative phrases are gone from CLAUDE.md's live instructions
  ! grep -E '^4\. \*\*Land\.' "$REAL_REPO_ROOT/CLAUDE.md" | grep -q 'Push to `main`'
  ! grep -E '^\| Land \|' "$REAL_REPO_ROOT/CLAUDE.md" | grep -q 'Push to `main`'

  # the pre-push hook description in the swept docs names the chained guard
  # (beads segment + cockpit gate + land guard), matching B82's installed reality
  grep -Eiq 'beads.*(cockpit|pre-push\.local).*guard|chained guard' "$REAL_REPO_ROOT/CLAUDE.md"

  # negative: a live direct-push instruction planted in a sister doc is
  # caught by name (mutates the disposable clone only)
  printf '\nLand: Push to `main` directly; rebase-on-reject (git serializes concurrent pushers).\n' \
    >> "$clone/AGENTS-WORKFLOW.md"
  run bash -c "cd '$clone' && $V_DOCTRINE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"AGENTS-WORKFLOW.md"* ]]
}
