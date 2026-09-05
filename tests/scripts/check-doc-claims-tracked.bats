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

# --- enumeration must not certify what it could not read -------------------

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

# --- a claim and its path split across lines inside a data fence -----------
#
# The claim words are matched per LINE. Inside a data fence a scorecard's
# `"status": "published"` and the path it names sit on different lines, so a
# missing path read as a forward reference and the sentence outran the tree
# unchecked. Inside a scanned data fence, a missing candidate path is an
# offender on its own: a data block is a record of what IS, never a plan.

@test "a multiline json fence reports a missing path even when the claim word is on another line" {
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{
  "status": "published",
  "artifact": "`evals/skill-probes/absent-egress.log`"
}
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/skill-probes/absent-egress.log missing"* ]]
}

@test "a multiline yaml fence reports a missing path even with no claim word at all" {
    cat > "$FIX/evals/doc.md" <<'MD'
```yaml
capture:
  harness: `scripts/absent-harness.sh`
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/absent-harness.sh missing"* ]]
}

@test "prose outside a fence keeps the claim-word rule for a missing path" {
    cat > "$FIX/evals/doc.md" <<'MD'
A future capture might land at `evals/skill-probes/future.log` eventually.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a data fence naming a path that exists and is tracked stays clean" {
    cat > "$FIX/evals/doc.md" <<'MD'
```json
{"harness": "`scripts/present.sh`"}
```
MD
    touch "$FIX/scripts/present.sh"
    git -C "$FIX" add evals/doc.md scripts/present.sh
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

# --- data fences: a plain JSON string is a claim, not decoration -----------

@test "a scorecard-shaped json fence reports a missing plain-string path" {
    # The real shape: the claim word and the path are on different lines and
    # neither is a backtick span, so backtick-only candidacy read it as clean.
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
    [[ "$output" == *"evals/skill-probes/absent-egress.log missing"* ]]
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

@test "a shell fence keeps its exemption for quoted tokens too" {
    cat > "$FIX/evals/doc.md" <<'MD'
```bash
cat "scripts/shell-quoted-untracked.sh"
```
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/shell-quoted-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

# --- prose: the claim is the paragraph, not the line ----------------------

@test "a prose claim wrapped across lines still binds its path" {
    cat > "$FIX/evals/doc.md" <<'MD'
The egress log for the low-effort capture is published at the
path `evals/skill-probes/wrapped.log` for anyone to inspect.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"evals/skill-probes/wrapped.log missing"* ]]
}

@test "a claim word in a different paragraph does not bind a later path" {
    cat > "$FIX/evals/doc.md" <<'MD'
The scorecard is published in this repository.

A future capture might land at `evals/skill-probes/future.log` eventually.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

# --- fence closers: character and length both matter ----------------------
#
# CommonMark closes a fence only with the same character, at least as long as
# the opener, and nothing but whitespace after it. Matching on the first
# character alone let a shorter run or an info-bearing line close a block, so
# the scanner's idea of "inside a fence" drifted from the renderer's.

@test "a shorter backtick run does not close a longer fence" {
    cat > "$FIX/evals/doc.md" <<'MD'
````bash
cat `scripts/still-inside.sh`
```
cat `scripts/also-inside.sh`
````

Back in prose.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/still-inside.sh" "$FIX/scripts/also-inside.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a longer backtick run does close a shorter fence" {
    cat > "$FIX/evals/doc.md" <<'MD'
```bash
cat `scripts/inside.sh`
````

This helper is at `scripts/outside-untracked.sh`.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/inside.sh" "$FIX/scripts/outside-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/outside-untracked.sh untracked"* ]]
    [[ "$output" != *"scripts/inside.sh"* ]]
}

@test "a tilde run does not close a backtick fence" {
    cat > "$FIX/evals/doc.md" <<'MD'
```bash
cat `scripts/tilde-inside.sh`
~~~
cat `scripts/tilde-also-inside.sh`
```

Back in prose.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/tilde-inside.sh" "$FIX/scripts/tilde-also-inside.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a backtick run does not close a tilde fence" {
    cat > "$FIX/evals/doc.md" <<'MD'
~~~bash
cat `scripts/backtick-inside.sh`
```
cat `scripts/backtick-also-inside.sh`
~~~

Back in prose.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/backtick-inside.sh" "$FIX/scripts/backtick-also-inside.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a closer carrying an info string does not close the fence" {
    cat > "$FIX/evals/doc.md" <<'MD'
```bash
cat `scripts/info-inside.sh`
``` trailing words
cat `scripts/info-also-inside.sh`
```

Back in prose.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/info-inside.sh" "$FIX/scripts/info-also-inside.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
}

@test "a data fence closed by a longer run resumes prose rules after it" {
    cat > "$FIX/evals/doc.md" <<'MD'
~~~json
{"harness": "scripts/data-inside-untracked.sh"}
~~~~

A future capture might land at `evals/skill-probes/later.log` eventually.
MD
    git -C "$FIX" add evals/doc.md
    git -C "$FIX" commit -q -m init
    touch "$FIX/scripts/data-inside-untracked.sh"

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/data-inside-untracked.sh untracked"* ]]
    [[ "$output" != *"later.log"* ]]
}
