#!/usr/bin/env bats
#
# Tests for the workflow retired-tracker gate (age-u1u6):
# scripts/check-workflow-no-retired-tracker.sh fails if any repo-tracked
# .claude/workflows/*.js instructs running the RETIRED bd/Dolt tracker
# (e.g. `bd ready`, `bd update`, `bd --json`). The tracker is `br`
# (BEADS_DIR="$(ao beads dir)" br ...); bd/Dolt is retired legacy.
#
# WHY: operating-loop.js — the single most-viewed content artifact on the repo
# (GitHub traffic) — shipped a prompt telling agents to run `bd ready`, a
# retired command, with no gate to catch it. This keeps the viewed workflow
# surface from silently re-staling.
#
# The gate derives repo_root from git and scans only .claude/workflows/*.js, so
# the word "bead", "bdd-foundry", or a `br` command never trips it.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GATE="$REPO_ROOT/scripts/check-workflow-no-retired-tracker.sh"
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts" "$FIX/.claude/workflows"
    cp "$GATE" "$FIX/scripts/check-workflow-no-retired-tracker.sh"
    chmod +x "$FIX/scripts/check-workflow-no-retired-tracker.sh"
    git -C "$FIX" init -q
    git -C "$FIX" config user.email t@t.t
    git -C "$FIX" config user.name t
}

write_wf() { printf '%s\n' "$2" > "$FIX/.claude/workflows/$1"; git -C "$FIX" add ".claude/workflows/$1"; }
run_gate() { ( cd "$FIX" && bash scripts/check-workflow-no-retired-tracker.sh ); }

@test "passes when workflows use br and never bd commands" {
    write_wf "ok.js" 'const x = `BEADS_DIR="$(ao beads dir)" br ready`; // pick a bead'
    run run_gate
    [ "$status" -eq 0 ]
}

@test "fails on a bd command in a workflow prompt" {
    write_wf "bad.js" 'const p = `No intent? Run \`bd ready\` and pick the top bead.`'
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"bad.js"* ]]
    [[ "$output" == *"bd ready"* ]]
}

# Each forbidden token is asserted independently — a regression in any single
# token's detection must turn a case red (not be masked by another token in the
# same fixture). pawl/codex catch: the old single fixture passed on `bd update`
# alone, so `bd --json` / `bd close` could silently stop being detected.
@test "catches bd update independently" {
    write_wf "u.js" 'log(`bd update ${id} --claim`)'
    run run_gate; [ "$status" -ne 0 ]
}
@test "catches bd close independently" {
    write_wf "c.js" 'log(`bd close ${id}`)'
    run run_gate; [ "$status" -ne 0 ]
}
@test "catches bd --json independently" {
    write_wf "j.js" 'const s = `bd --json list`'
    run run_gate; [ "$status" -ne 0 ]
}
@test "catches bd show independently" {
    write_wf "s.js" 'log(`bd show ${id}`)'
    run run_gate; [ "$status" -ne 0 ]
}

# Fail CLOSED: a tracked workflow that is unreadable/absent in the worktree must
# fail, never be silently skipped (pawl/codex catch: grep error -> "no match").
@test "fails closed when a tracked workflow is unreadable in the worktree" {
    write_wf "gone.js" 'const x = `br ready`'
    git -C "$FIX" commit -qm wf
    rm "$FIX/.claude/workflows/gone.js"   # tracked but absent in worktree
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"gone.js"* ]]
}

# An empty tracked set is genuinely nothing to protect -> pass (not fail-closed).
@test "passes when no workflows are tracked at all" {
    run run_gate
    [ "$status" -eq 0 ]
}

@test "does not false-positive on the word bead, bdd-foundry, or embed" {
    write_wf "fine.js" 'const meta = { name: "bdd-foundry" }; // embed the bead body; the bead id'
    run run_gate
    [ "$status" -eq 0 ]
}
