#!/usr/bin/env bats
#
# Behavioral spec for scripts/check-doc-claims-tracked.sh — the
# docs.claims-tracked gate.
#
# WHY: on 2026-09-03 an eval doc cited `evals/skill-probes/egress.log` as
# published while the repository's `*.log` ignore rule kept that exact file out
# of the tree, and a fresh verifier accepted the absence. This spec pins the
# fact rule: a backtick span or double-quoted string naming a path under
# evals/, docs/evals/, scripts/, or tests/ is an offender when git ignores it,
# or when it exists on disk but is untracked. A tracked path is fine, and so is
# a missing path no ignore rule covers.
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

# The repository's own ignore rule from the 2026-09-03 incident.
ignore_logs() {
    printf '*.log\n' > "$FIX/.gitignore"
    git -C "$FIX" add .gitignore
}

# A `git` on PATH that fails (exit 128) for one subcommand and defers to the
# real git for everything else.
fake_git_failing() {
    local sub="$1"
    mkdir -p "$BATS_TEST_TMPDIR/fakebin"
    cat > "$BATS_TEST_TMPDIR/fakebin/git" <<SH
#!/usr/bin/env bash
if [ "\$*" != "\${*/$sub/}" ]; then
  echo "git exploded" >&2
  exit 128
fi
exec /usr/bin/git "\$@"
SH
    chmod +x "$BATS_TEST_TMPDIR/fakebin/git"
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

@test "a path the repository ignores fails as ignored, with no claim word needed" {
    ignore_logs
    cat > "$FIX/evals/doc.md" <<'MD'
The egress capture: `evals/skill-probes/egress.log`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/doc.md:1: evals/skill-probes/egress.log ignored"* ]]
}

@test "a tracked path wins over an ignore rule that also matches it" {
    ignore_logs
    cat > "$FIX/evals/doc.md" <<'MD'
The egress capture: `evals/skill-probes/egress.log`.
MD
    mkdir -p "$FIX/evals/skill-probes"
    touch "$FIX/evals/skill-probes/egress.log"
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" add -f evals/skill-probes/egress.log
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a missing, non-ignored path is not an offender" {
    cat > "$FIX/evals/doc.md" <<'MD'
A future capture might land at `evals/skill-probes/future.log` eventually.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

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

@test "a text fence naming an untracked path is scanned" {
    cat > "$FIX/evals/doc.md" <<'MD'
The layout:

```text
`scripts/text-fence-untracked.sh`
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/text-fence-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/text-fence-untracked.sh untracked"* ]]
}

@test "trailing sentence punctuation is stripped before the path is resolved" {
    cat > "$FIX/evals/doc.md" <<'MD'
The receipt lives at `scripts/punctuated.sh.`
And the fixture at `tests/fixtures/thing.json,` plus `tests/other.json;` and `tests/third.json:`
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    mkdir -p "$FIX/tests/fixtures"
    touch "$FIX/scripts/punctuated.sh" "$FIX/tests/fixtures/thing.json" \
        "$FIX/tests/other.json" "$FIX/tests/third.json"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/punctuated.sh untracked"* ]]
    [[ "$output" == *"tests/fixtures/thing.json untracked"* ]]
    [[ "$output" == *"tests/other.json untracked"* ]]
    [[ "$output" == *"tests/third.json untracked"* ]]
    # The stripped punctuation never leaks into the reported path.
    [[ "$output" != *"punctuated.sh. untracked"* ]]
}

@test "both offender states are retained in one run" {
    ignore_logs
    cat > "$FIX/evals/doc.md" <<'MD'
Present but untracked: `scripts/untracked.sh`.
The capture: `evals/absent.log`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/doc.md:1: scripts/untracked.sh untracked"* ]]
    [[ "$output" == *"evals/doc.md:2: evals/absent.log ignored"* ]]
}

@test "a git failure during the tracked lookup fails closed rather than reporting untracked" {
    cat > "$FIX/evals/doc.md" <<'MD'
This helper is at `scripts/tracked.sh`.
MD
    touch "$FIX/scripts/tracked.sh"
    git -C "$FIX" add evals/doc.md scripts/tracked.sh
    git -C "$FIX" commit -q -m init

    # A `git` that exits with neither 0 (tracked) nor 1 (untracked) is an
    # unanswered question, not a negative answer.
    fake_git_failing ls-files

    run env PATH="$BATS_TEST_TMPDIR/fakebin:$PATH" bash "$GATE" "$FIX"
    [ "$status" -eq 2 ]
    [[ "$output" == *"git ls-files failed for scripts/tracked.sh (exit 128)"* ]]
    [[ "$output" != *"untracked"* ]]
    [[ "$output" != *"OK:"* ]]
}

@test "a git failure during the ignore lookup fails closed rather than reporting untracked" {
    cat > "$FIX/evals/doc.md" <<'MD'
This helper is at `scripts/present.sh`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/present.sh"

    fake_git_failing check-ignore

    run env PATH="$BATS_TEST_TMPDIR/fakebin:$PATH" bash "$GATE" "$FIX"
    [ "$status" -eq 2 ]
    [[ "$output" == *"git check-ignore failed for scripts/present.sh (exit 128)"* ]]
    [[ "$output" != *"present.sh untracked"* ]]
    [[ "$output" != *"OK:"* ]]
}

@test "an unreadable directory under evals fails closed instead of certifying clean" {
    cat > "$FIX/evals/doc.md" <<'MD'
Nothing controversial here.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    mkdir -p "$FIX/evals/locked"
    chmod 000 "$FIX/evals/locked"

    run bash "$GATE" "$FIX"
    chmod 755 "$FIX/evals/locked"
    [ "$status" -eq 2 ]
    [[ "$output" != *"OK:"* ]]
    [[ "$output" == *"could not enumerate"* ]]
}

# --- double-quoted strings are candidates too -------------------------------

@test "a scorecard-shaped json fence reports an ignored plain-string path" {
    # The incident shape: a JSON artifact string names a .log file the
    # repository ignores, and the claim word sits on another line.
    ignore_logs
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{
  "status": "published",
  "artifact": "evals/skill-probes/absent-egress.log"
}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/doc.md:4: evals/skill-probes/absent-egress.log ignored"* ]]
}

@test "a data fence reports an untracked plain-string path" {
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{"harness": "scripts/quoted-untracked.sh"}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/quoted-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/quoted-untracked.sh untracked"* ]]
}

@test "a quoted token outside the governed trees is still not a candidate" {
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{"home": "~/notes/private.md", "url": "https://example.invalid/a/b", "glob": "scripts/*.sh"}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}
