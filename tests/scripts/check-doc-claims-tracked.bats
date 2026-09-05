#!/usr/bin/env bats
#
# Behavioral spec for scripts/check-doc-claims-tracked.sh — the
# docs.claims-tracked gate.
#
# WHY: on 2026-09-03 a doc sentence called an egress log "published" while the
# repository's `*.log` ignore rule kept that exact file out of the tree, and a
# fresh verifier accepted the absence because nothing checked the sentence
# against the working tree. Four separate judge rounds that day filed the same
# "doc sentence outruns the tree" finding. This spec pins the rule: a
# backtick-quoted path under evals/, docs/evals/, scripts/, or tests/ must be
# tracked by git when it exists, and must exist on disk when its sentence
# claims it is "published", "tracked", "committed", or "lives at".
#
# Each test builds a throwaway git repo (git discovery env scrubbed first — a
# fixture `git init` that inherits a hook-injected GIT_DIR rewrites the
# SHARED .git/config and bricks every linked worktree, .claude/rules/go.md,
# ek8v) with its own evals/ and docs/evals/ trees, so the assertions are about
# the gate's rule, not about the current state of this repository.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    GATE="$REPO_ROOT/scripts/check-doc-claims-tracked.sh"
    export GATE

    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX \
        GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE

    FIX="$BATS_TEST_TMPDIR/fixture-repo"
    mkdir -p "$FIX/evals" "$FIX/docs/evals" "$FIX/scripts" "$FIX/tests"
    git -C "$FIX" init -q
    git -C "$FIX" config user.name "claims-tracked fixture"
    git -C "$FIX" config user.email "claims-tracked@fixture.invalid"
    export FIX
}

@test "an untracked-but-present path named in a doc fails with its line" {
    cat > "$FIX/evals/doc.md" <<'MD'
Line one is prose.
This helper is at `scripts/exists-untracked.sh`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/exists-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/doc.md:2: scripts/exists-untracked.sh untracked"* ]]
}

@test "a tracked path named in a doc passes" {
    cat > "$FIX/evals/doc.md" <<'MD'
This helper is at `scripts/tracked.sh`.
MD
    touch "$FIX/scripts/tracked.sh"
    git -C "$FIX" add evals/doc.md scripts/tracked.sh
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a placeholder path and a glob pattern are ignored" {
    cat > "$FIX/evals/doc.md" <<'MD'
See `<skill>/SKILL.md` for the shape and `scripts/*.sh` for every script.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a published claim for a missing path fails" {
    cat > "$FIX/evals/doc.md" <<'MD'
The egress log is published at `evals/skill-probes/egress.log`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/doc.md:1: evals/skill-probes/egress.log missing"* ]]
}

@test "a missing path with no claim word is not an offender" {
    cat > "$FIX/evals/doc.md" <<'MD'
A future capture might land at `evals/skill-probes/future.log` eventually.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a fenced code block is skipped even when it quotes an untracked path" {
    cat > "$FIX/evals/doc.md" <<'MD'
Run it like this:

```
cat scripts/inside-fence-untracked.sh
```

Everything below the fence is prose again.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/inside-fence-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "docs/evals is scanned alongside evals" {
    cat > "$FIX/docs/evals/report.md" <<'MD'
This report cites `scripts/other-untracked.sh`.
MD
    git -C "$FIX" add docs/evals/report.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/other-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/evals/report.md:1: scripts/other-untracked.sh untracked"* ]]
}

@test "a path outside the four governed trees is never a candidate" {
    cat > "$FIX/evals/doc.md" <<'MD'
Ask a human about `~/notes/private.md` or `/etc/hosts` or `$HOME/config`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "an invalid repository path fails closed" {
    run bash "$GATE" "$BATS_TEST_TMPDIR/definitely-not-a-repository"
    [ "$status" -eq 2 ]
    [[ "$output" == *"is not a git repository"* ]]
    [[ "$output" != *"OK:"* ]]
}

@test "a repository with no evals or docs/evals Markdown passes trivially" {
    EMPTY="$BATS_TEST_TMPDIR/empty-repo"
    mkdir -p "$EMPTY"
    git -C "$EMPTY" init -q

    run bash "$GATE" "$EMPTY"
    [ "$status" -eq 0 ]
    [[ "$output" == *"nothing to check"* ]]
}

@test "the real repository passes" {
    run bash "$GATE" "$REPO_ROOT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}
