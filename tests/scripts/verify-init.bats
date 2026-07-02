#!/usr/bin/env bats
# End-to-end coverage for `ao verify init` — the portable pre-push verdict
# ratchet (age-rk3r.6). Uses the REAL ao binary + a real bare remote so
# `git push` fires the installed hook exactly as it would in a stranger repo.

setup_file() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    AO="$BATS_FILE_TMPDIR/ao"
    ( cd "$ROOT/cli" && go build -o "$AO" ./cmd/ao ) || {
        echo "failed to build ao for the verify-init bats" >&2
        return 1
    }
    export AO
}

setup() {
    AO="$BATS_FILE_TMPDIR/ao"
    REPO="$BATS_TEST_TMPDIR/work"
    REMOTE="$BATS_TEST_TMPDIR/remote.git"
    git init -q --initial-branch=main "$REPO"
    git init -q --bare "$REMOTE"
    git -C "$REPO" remote add origin "$REMOTE"
    git -C "$REPO" config user.email t@example.com
    git -C "$REPO" config user.name Tester
    git -C "$REPO" config commit.gpgsign false
    echo hi > "$REPO/README.md"
    git -C "$REPO" add README.md
    git -C "$REPO" commit -qm "chore: init"
    git -C "$REPO" push -q origin main
    # Every test runs FROM the throwaway repo so `ao verify init` can never
    # target the real checkout (git-common-dir of a worktree is the real .git).
    cd "$REPO"
}

# Fabricate a CONFIRMED verdict edge bound to $1 (a commit sha) and commit the
# ledger as a #trivial provenance-only commit.
bind_verdict() {
    sha="$1"
    bead="$2"
    "$AO" provenance add "$bead@${sha:0:7}" "$sha" \
        --from-type verdict --to-type commit --relation wasDerivedFrom \
        --trust-tier inferred --evidence "pawl-verdict $bead disposition=CONFIRMED" >/dev/null
    git -C "$REPO" add docs/provenance/ledger.jsonl
    git -C "$REPO" commit -qm "chore(provenance): bind verdict for $bead #trivial"
}

@test "init installs an executable, marker-bearing pre-push hook" {
    run "$AO" verify init
    [ "$status" -eq 0 ]
    [ -x "$REPO/.git/hooks/pre-push" ]
    grep -q "AGENTOPS-VERIFY-RATCHET" "$REPO/.git/hooks/pre-push"
    # The install-time ao absolute path is baked in (no unsubstituted placeholder).
    ! grep -q "@@AO_BIN@@" "$REPO/.git/hooks/pre-push"
}

@test "push of a commit with NO verdict is refused, naming ao verify" {
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo change >> README.md
    git commit -qam "feat: a change (age-b1)"
    run git push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"PUSH REFUSED"* ]]
    [[ "$output" == *"ao verify"* ]]
}

@test "push proceeds once a CONFIRMED commit-bound verdict exists" {
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo change >> README.md
    git commit -qam "feat: a change (age-b2)"
    code="$(git rev-parse HEAD)"
    bind_verdict "$code" age-b2
    run git push origin main
    [ "$status" -eq 0 ]
}

@test "an appended-but-UNCOMMITTED ledger edge does not authorize the push" {
    # The cross-family refuter's repro (age-rk3r.6 amend), end-to-end through the
    # real hook: proof is the ledger AS COMMITTED in the pushed tree — a valid
    # edge sitting only in the working tree never reaches the remote.
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo change >> README.md
    git commit -qam "feat: a change (age-b6)"
    code="$(git rev-parse HEAD)"
    # Append the CONFIRMED edge but do NOT git-add or commit the ledger.
    "$AO" provenance add "age-b6@${code:0:7}" "$code" \
        --from-type verdict --to-type commit --relation wasDerivedFrom \
        --trust-tier inferred --evidence "pawl-verdict age-b6 disposition=CONFIRMED" >/dev/null
    run git push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"PUSH REFUSED"* ]]
    [[ "$output" == *"ao verify"* ]]
    # Committing the same edge (the bind commit) then authorizes the push.
    git add docs/provenance/ledger.jsonl
    git commit -qm "chore(provenance): bind verdict for age-b6 #trivial"
    run git push origin main
    [ "$status" -eq 0 ]
}

@test "a FAILED pre-push stdin capture (disk full / ulimit -f) refuses fail-closed, never gating empty input" {
    # Cross-family refuter's class: the hook captures git's ref-line stdin to a
    # temp file; if that write FAILS/truncates (ENOSPC, RLIMIT_FSIZE), an empty
    # capture would make `ao verify pre-push` read a non-hook invocation and exit
    # 0 — a fail-OPEN. The commit here IS verdict-bound, so a WORKING capture
    # would PASS; only the capture failure must flip it to a fail-closed refusal.
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo change >> README.md
    git commit -qam "feat: a change (age-b7)"
    code="$(git rev-parse HEAD)"
    bind_verdict "$code" age-b7
    # Skip where this shell's RLIMIT_FSIZE is not enforced (some CI sandboxes).
    ( ulimit -f 0 2>/dev/null; printf 'xxxx' > "$BATS_TEST_TMPDIR/fsprobe" ) 2>/dev/null || true
    if [ -s "$BATS_TEST_TMPDIR/fsprobe" ]; then skip "RLIMIT_FSIZE not enforced in this environment"; fi
    remote_sha="$(git rev-parse origin/main)"
    # Fire the installed hook exactly as git would (ref line on stdin, remote
    # name + url as args) but with a 0 file-size limit so the internal capture
    # `cat >"$tmp"` write of the ref line fails mid-stream.
    run bash -c 'ulimit -f 0; printf "refs/heads/main %s refs/heads/main %s\n" "$1" "$2" | "$3" origin "$4"' \
        _ "$code" "$remote_sha" "$REPO/.git/hooks/pre-push" "$REMOTE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"failed to capture"* ]]
}

@test "hostile PATH: planted repo-internal git and cat are never executed by hook or gate" {
    # Round-4 refuter's class, end-to-end: an operator PATH carrying a
    # repo-internal entry (direnv-style \$PWD/bin) must not let planted binaries
    # subvert the ratchet — a planted `git` could forge the gate's answers, and
    # a planted `cat` could swallow the hook's stdin capture into a
    # "no pre-push stdin" skip. The hook prepends system dirs for its own
    # utilities and the gate resolves git on a sanitized PATH, so neither
    # planted binary runs and the verdict matches a clean environment.
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo change >> README.md
    git commit -qam "feat: unverified (age-b8)"

    sentinel="$BATS_TEST_TMPDIR/PWNED"
    mkdir -p bin
    printf '#!/bin/sh\necho x >> "%s"\ncase "$*" in *cat-file*) exit 1 ;; esac\nexit 0\n' "$sentinel" > bin/git
    printf '#!/bin/sh\necho x >> "%s"\nexit 0\n' "$sentinel" > bin/cat
    chmod +x bin/git bin/cat

    # Drive the push with the REAL git via absolute path (only the hook + gate
    # see the hostile PATH; the pushing git itself must be genuine).
    real_git="$(command -v git)"
    run env PATH="$REPO/bin:$PATH" "$real_git" push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"PUSH REFUSED"* ]]
    [[ "$output" == *"ao verify"* ]]
    [ ! -e "$sentinel" ]
}

@test "creation push (new remote) checks the whole new history, not just a #trivial tip" {
    # Round-3 refuter's repro end-to-end: pushing a branch the remote does not
    # have (zero remote sha) must check every commit not already on a
    # remote-tracking ref — an unverified code commit cannot ride in under a
    # provenance-only #trivial tip. (The setup's seeded origin/main tracking ref
    # correctly excludes the pre-hook init commit from the checked range.)
    cd "$REPO"
    "$AO" verify init >/dev/null
    echo x > code.txt && git add code.txt
    git commit -qm "feat: unverified code (age-b7)"
    code="$(git rev-parse HEAD)"
    mkdir -p docs/provenance && echo n > docs/provenance/note.txt
    git add docs/provenance/note.txt
    git commit -qm "chore(provenance): note #trivial"

    git init -q --bare "$BATS_TEST_TMPDIR/remote2.git"
    git remote add origin2 "$BATS_TEST_TMPDIR/remote2.git"
    run git push origin2 main
    [ "$status" -ne 0 ]
    [[ "$output" == *"PUSH REFUSED"* ]]
    [[ "$output" == *"${code:0:12}"* ]]
    [[ "$output" == *"ao verify"* ]]

    # Binding a committed CONFIRMED verdict to the code commit unblocks it.
    bind_verdict "$code" age-b7
    run git push origin2 main
    [ "$status" -eq 0 ]
}

@test "a pre-existing pre-push hook is chained and restored byte-identically on --remove" {
    cd "$REPO"
    printf '#!/usr/bin/env sh\necho custom-hook-ran >&2\nexit 0\n' > .git/hooks/pre-push
    chmod +x .git/hooks/pre-push
    cp .git/hooks/pre-push "$BATS_TEST_TMPDIR/orig-copy"

    run "$AO" verify init
    [ "$status" -eq 0 ]
    [[ "$output" == *"chained"* ]]
    grep -q "AGENTOPS-VERIFY-RATCHET" .git/hooks/pre-push
    # The sidecar holds the original byte-identically.
    cmp -s .git/hooks/pre-push.agentops-orig "$BATS_TEST_TMPDIR/orig-copy"

    # The chained original runs FIRST on push (then the gate refuses, no verdict).
    echo change >> README.md
    git commit -qam "feat: c (age-b3)"
    run git push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"custom-hook-ran"* ]]
    [[ "$output" == *"PUSH REFUSED"* ]]

    # Remove restores the original byte-identically; sidecar is gone.
    run "$AO" verify init --remove
    [ "$status" -eq 0 ]
    cmp -s .git/hooks/pre-push "$BATS_TEST_TMPDIR/orig-copy"
    [ ! -e .git/hooks/pre-push.agentops-orig ]
}

@test "re-init is idempotent (no duplicate hook, byte-stable)" {
    cd "$REPO"
    "$AO" verify init >/dev/null
    cp .git/hooks/pre-push "$BATS_TEST_TMPDIR/first"
    run "$AO" verify init
    [ "$status" -eq 0 ]
    [[ "$output" == *"refreshed"* ]]
    cmp -s .git/hooks/pre-push "$BATS_TEST_TMPDIR/first"
    # Exactly one begin marker.
    run grep -c ">>> AGENTOPS-VERIFY-RATCHET" .git/hooks/pre-push
    [ "$output" -eq 1 ]
}

@test "hostile repo: a planted repo-tree pawl-verdict.sh is NEVER executed by the hook" {
    cd "$REPO"
    mkdir -p scripts scripts/lib
    sentinel="$BATS_TEST_TMPDIR/PWNED"
    printf '#!/usr/bin/env sh\ntouch "%s"\nexit 0\n' "$sentinel" > scripts/pawl-verdict.sh
    chmod +x scripts/pawl-verdict.sh
    printf '#!/usr/bin/env sh\ntouch "%s"\nexit 0\n' "$sentinel" > scripts/check-pawl-pre-push.sh
    chmod +x scripts/check-pawl-pre-push.sh
    git add -A && git commit -qm "chore: planted scripts"
    git push -q origin main

    "$AO" verify init >/dev/null
    echo x > code.txt && git add code.txt && git commit -qm "feat: evil (age-b4)"
    run git push origin main
    [ "$status" -ne 0 ]                 # refused (no verdict), via the embedded gate
    [[ "$output" == *"PUSH REFUSED"* ]]
    [ ! -e "$sentinel" ]                # NONE of the planted repo scripts ran
}

@test "install refuses to bake a repo-internal ao path (the swap-the-baked-binary attack)" {
    # Install-side half of the no-repo-internal-absolute invariant: the runtime
    # hook trusts the baked path absolutely, so the baked path must never be
    # inside the repo — a repo-local ao could later be swapped for a fake that
    # fakes the version + exits 0 for `verify pre-push` to authorize an
    # unverified push. `ao verify init` run from a repo-internal ao must REFUSE
    # at install time, so the bad install never happens.
    cd "$REPO"
    mkdir -p bin
    cp "$AO" bin/ao   # a repo-INTERNAL ao

    run "$REPO/bin/ao" verify init
    [ "$status" -ne 0 ]
    [[ "$output" == *"repo-internal ao path"* ]]
    [[ "$output" == *"ao verify init"* ]]
    # The bad install never happened: no hook was written.
    [ ! -e .git/hooks/pre-push ]

    # And a trusted ao OUTSIDE the repo installs normally (happy path unchanged).
    run "$AO" verify init
    [ "$status" -eq 0 ]
    grep -q "AGENTOPS-VERIFY-RATCHET" .git/hooks/pre-push
}

@test "baked ao missing: hook refuses with a reinstall message; a planted ./bin/ao is NEVER consulted" {
    # Convergence-by-deletion: the baked absolute ao path is the ONLY accepted
    # ao — the command -v ao / AGENTOPS_AO_BIN fallback is gone. A stale install
    # (baked binary deleted) must FAIL CLOSED with a reinstall message, never
    # fall back to a repo-planted ./bin/ao.
    cd "$REPO"
    gone="$BATS_TEST_TMPDIR/ao-gone"
    cp "$AO" "$gone"
    "$gone" verify init >/dev/null   # bakes $gone as the hook's only ao
    rm -f "$gone"                    # the baked binary is now missing (stale install)

    sentinel="$BATS_TEST_TMPDIR/PWNED-ao"
    mkdir -p bin
    printf '#!/bin/sh\necho x >> "%s"\nexit 0\n' "$sentinel" > bin/ao
    chmod +x bin/ao
    echo change >> README.md
    git commit -qam "feat: x (age-b9)"

    real_git="$(command -v git)"
    run env PATH="$REPO/bin:.:$PATH" "$real_git" push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"the installed ao binary is missing"* ]]
    [[ "$output" == *"ao verify init"* ]]
    [ ! -e "$sentinel" ]             # the planted ./bin/ao was NEVER consulted
}

@test "version floor: an ao too old to read the ledger refuses with an upgrade message" {
    cd "$REPO"
    # Install with a COPY of ao, then swap a fake "old ao" in at the SAME baked
    # absolute path — the hook trusts EXACTLY the baked path (no env override).
    swap="$BATS_TEST_TMPDIR/ao-swap"
    cp "$AO" "$swap"
    "$swap" verify init >/dev/null
    # Replace the baked binary in place: no ledger-reader-version subcommand
    # (exits non-zero); its verify pre-push would wrongly pass — the floor must
    # stop us reaching it.
    cat > "$swap" <<'EOS'
#!/bin/sh
case "$1 $2" in
  "provenance ledger-reader-version") echo "unknown command" >&2; exit 1 ;;
  "verify pre-push") echo "OLD-AO-GATE-RAN"; exit 0 ;;
esac
exit 0
EOS
    chmod +x "$swap"
    echo change >> README.md
    git commit -qam "feat: x (age-b5)"
    run git push origin main
    [ "$status" -ne 0 ]
    [[ "$output" == *"too old"* ]]
    [[ "$output" == *"upgrade"* ]]
    [[ "$output" != *"OLD-AO-GATE-RAN"* ]]
}
