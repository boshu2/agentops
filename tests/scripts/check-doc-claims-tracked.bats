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

# --- fence handling: skip shells, scan data --------------------------------
#
# The original rule skipped EVERY fenced block, so a `text` or `json` fence
# quoting a real repo path -- the exact shape a scorecard or a probe README
# uses to show its own layout -- was silently exempt. Only a fence that quotes
# a COMMAND is exempt, because a command line is an example invocation, not a
# claim about the tree.

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

@test "a json fence naming an untracked path is scanned" {
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{"harness": "`scripts/json-fence-untracked.sh`"}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/json-fence-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/json-fence-untracked.sh untracked"* ]]
}

@test "shell-info fences stay exempt for every shell spelling" {
    for info in bash sh shell console zsh; do
        cat > "$FIX/evals/doc-$info.md" <<MD
\`\`\`$info
cat \`scripts/shell-fence-$info.sh\`
\`\`\`
MD
        touch "$FIX/scripts/shell-fence-$info.sh"
    done
    git -C "$FIX" add evals
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "an info-less fence whose first line is a prompt or a command stays exempt" {
    cat > "$FIX/evals/prompt.md" <<'MD'
```
$ cat `scripts/prompt-fence-untracked.sh`
```
MD
    cat > "$FIX/evals/command.md" <<'MD'
```
bash `scripts/command-fence-untracked.sh`
```
MD
    touch "$FIX/scripts/prompt-fence-untracked.sh" "$FIX/scripts/command-fence-untracked.sh"
    git -C "$FIX" add evals
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "an info-less fence that is plainly data is scanned" {
    cat > "$FIX/evals/doc.md" <<'MD'
```
{"binds": "`scripts/data-fence-untracked.sh`"}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/data-fence-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/data-fence-untracked.sh untracked"* ]]
}

# --- code-span punctuation -------------------------------------------------

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

@test "a punctuation-stripped path is still held to the claim rule when missing" {
    cat > "$FIX/evals/doc.md" <<'MD'
The egress log is published at `evals/gone.log.`
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/gone.log missing"* ]]
}

# --- enumeration and fail-closed ------------------------------------------

@test "both offender states are retained in one run" {
    cat > "$FIX/evals/doc.md" <<'MD'
Present but untracked: `scripts/untracked.sh`.
The log is published at `evals/absent.log`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/untracked.sh untracked"* ]]
    [[ "$output" == *"evals/absent.log missing"* ]]
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
    mkdir -p "$BATS_TEST_TMPDIR/fakebin"
    cat > "$BATS_TEST_TMPDIR/fakebin/git" <<'SH'
#!/usr/bin/env bash
if [ "$3" = "ls-files" ] || [ "$*" != "${*/ls-files/}" ]; then
  echo "git exploded" >&2
  exit 128
fi
exec /usr/bin/git "$@"
SH
    chmod +x "$BATS_TEST_TMPDIR/fakebin/git"

    run env PATH="$BATS_TEST_TMPDIR/fakebin:$PATH" bash "$GATE" "$FIX"
    [ "$status" -eq 2 ]
    [[ "$output" != *"untracked"* ]]
}
