#!/usr/bin/env bats
# Tests for scripts/lib/ratchet.sh (age-ratchet-lib-extraction-bv7d.1) — the
# shared shrink-only pinned-list ratchet mechanics extracted from the 7 in-tree
# grandfather/baseline gates.
#
# Pinned contracts:
#   - parse modes reproduce each consumer family's ORIGINAL entry parsing
#     (raw / cr-strip / strip / trailing-comment) — never a universal superset.
#   - growth guard (comm -13 vs base ref) rejects added entries fail-closed;
#     the initial snapshot (no base file) stands alone; RATCHET_GROWTH_GUARD=off
#     is the migrated-gate escape.
#   - intersection authority: an entry protects only when present in BOTH the
#     working pinned file AND the base-ref version (same-diff self-allowlist
#     grants nothing).
#   - stale detection by set-diff and by caller predicate (exists-only family).
#   - regenerate writes header + LC_ALL=C sorted body.
#
# All git-dependent scenarios run in an ISOLATED temp repo (bats_init_repo);
# the real checkout is never mutated. Fixtures round-trip REAL pinned-file
# shapes (f-2026-07-09-regression-fixture-can-dodge-the-failopen-path).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    LIB="$REPO_ROOT/scripts/lib/ratchet.sh"
    source "$REPO_ROOT/lib/bats-common.bash"
    TMP_DIR="$(mktemp -d)"
    bats_init_repo "$TMP_DIR"
    # shellcheck disable=SC1090
    source "$LIB"
}

teardown() {
    rm -rf "$TMP_DIR"
}

seed_commit() {
    ( cd "$TMP_DIR" && git add -A && git commit -qm "${1:-seed}" )
}

# --- parse modes -------------------------------------------------------------

@test "load_pinned raw: real jsonl-grandfather shape round-trips (comments/blanks dropped, entries verbatim)" {
    cat > "$TMP_DIR/pinned" <<'EOF'
# scripts/.jsonl-scanner-grandfather — FILENAME-pinned grandfather list.
#
# The list only SHRINKS.

cli/cmd/ao/alpha.go
cli/internal/beta/reader.go
EOF
    run ratchet_load_pinned "$TMP_DIR/pinned" raw
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "cli/cmd/ao/alpha.go" ]
    [ "${lines[1]}" = "cli/internal/beta/reader.go" ]
    [ "${#lines[@]}" -eq 2 ]
}

@test "load_pinned raw preserves interior whitespace verbatim (no silent trim)" {
    printf '%s\n' "entry-with-trailing-space " > "$TMP_DIR/pinned"
    run ratchet_load_pinned "$TMP_DIR/pinned" raw
    [ "$status" -eq 0 ]
    [ "$output" = "entry-with-trailing-space " ]
}

@test "load_pinned cr-strip: CRLF entries lose the CR only" {
    printf 'docs/alpha.md\r\n# comment\r\ndocs/beta.md\r\n' > "$TMP_DIR/pinned"
    run ratchet_load_pinned "$TMP_DIR/pinned" cr-strip
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "docs/alpha.md" ]
    [ "${lines[1]}" = "docs/beta.md" ]
}

@test "load_pinned strip: leading/trailing whitespace trimmed" {
    printf '  docs/alpha.md  \n\t docs/beta.md\n' > "$TMP_DIR/pinned"
    run ratchet_load_pinned "$TMP_DIR/pinned" strip
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "docs/alpha.md" ]
    [ "${lines[1]}" = "docs/beta.md" ]
}

@test "load_pinned trailing-comment: real skill-refs baseline shape (trailing # stripped, trimmed)" {
    cat > "$TMP_DIR/pinned" <<'EOF'
# baseline header
docs/alpha.md   # dead ref: /retired-skill
docs/beta.md
EOF
    run ratchet_load_pinned "$TMP_DIR/pinned" trailing-comment
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "docs/alpha.md" ]
    [ "${lines[1]}" = "docs/beta.md" ]
}

@test "load_pinned on a missing file emits nothing and succeeds" {
    run ratchet_load_pinned "$TMP_DIR/absent" raw
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# --- set arithmetic ----------------------------------------------------------

@test "new_violations: offenders minus pinned" {
    printf 'a\nb\n' > "$TMP_DIR/pinned"
    run ratchet_new_violations "$TMP_DIR/pinned" raw <<'EOF'
a
c
EOF
    [ "$status" -eq 0 ]
    [ "$output" = "c" ]
}

@test "new_violations with empty/missing pinned file: every offender is new" {
    run ratchet_new_violations "$TMP_DIR/absent" raw <<'EOF'
a
b
EOF
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "a" ]
    [ "${lines[1]}" = "b" ]
}

@test "stale_entries: pinned minus offenders" {
    printf 'a\nb\nc\n' > "$TMP_DIR/pinned"
    run ratchet_stale_entries "$TMP_DIR/pinned" raw <<'EOF'
b
EOF
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "a" ]
    [ "${lines[1]}" = "c" ]
}

@test "stale_entries_by predicate: exists-only family keeps entries whose files exist" {
    ( cd "$TMP_DIR" && mkdir -p d && touch d/kept )
    printf 'd/kept\nd/gone\n' > "$TMP_DIR/pinned"
    _exists_pred() { [ -f "$TMP_DIR/$1" ]; }
    run ratchet_stale_entries_by _exists_pred "$TMP_DIR/pinned" raw
    [ "$status" -eq 0 ]
    [ "$output" = "d/gone" ]
}

# --- growth guard + intersection authority -----------------------------------

@test "assert_shrink_only rejects an added entry vs the base ref and names it" {
    ( cd "$TMP_DIR" && printf '# h\nold-entry\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf '# h\nold-entry\nsneaky-new-entry\n' > pinned )
    ( cd "$TMP_DIR" && ratchet_load_base pinned HEAD )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_assert_shrink_only pinned raw"
    [ "$status" -ne 0 ]
    [[ "$output" == *"sneaky-new-entry"* ]]
}

@test "assert_shrink_only passes when the list only shrinks" {
    ( cd "$TMP_DIR" && printf 'a\nb\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'a\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_assert_shrink_only pinned raw"
    [ "$status" -eq 0 ]
}

@test "assert_shrink_only initial snapshot (no base file) stands alone" {
    ( cd "$TMP_DIR" && echo seed > seed.txt )
    seed_commit "initial"
    ( cd "$TMP_DIR" && printf 'brand-new\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_assert_shrink_only pinned raw"
    [ "$status" -eq 0 ]
}

@test "RATCHET_GROWTH_GUARD=off disables the growth rejection (migrated-gate escape)" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'old\ngrown\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && RATCHET_GROWTH_GUARD=off ratchet_assert_shrink_only pinned raw"
    [ "$status" -eq 0 ]
}

@test "is_pinned intersection authority: same-diff self-allowlist grants NO protection" {
    ( cd "$TMP_DIR" && printf 'blessed\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'blessed\nself-added\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_is_pinned blessed pinned raw"
    [ "$status" -eq 0 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_is_pinned self-added pinned raw"
    [ "$status" -ne 0 ]
}

@test "is_pinned with no base snapshot: the working file is the cutoff and stands alone" {
    ( cd "$TMP_DIR" && echo seed > seed.txt )
    seed_commit "initial"
    ( cd "$TMP_DIR" && printf 'only-working\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_is_pinned only-working pinned raw"
    [ "$status" -eq 0 ]
}

# --- changed-scope collection ------------------------------------------------

@test "changed_files head scope lists the last commit's files" {
    ( cd "$TMP_DIR" && echo seed > seed.txt )
    seed_commit "initial"
    ( cd "$TMP_DIR" && echo x > tracked.txt )
    seed_commit "add tracked"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files head"
    [ "$status" -eq 0 ]
    [[ "$output" == *"tracked.txt"* ]]
}

@test "changed_files worktree scope includes untracked files" {
    ( cd "$TMP_DIR" && echo y > untracked.txt )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files worktree"
    [ "$status" -eq 0 ]
    [[ "$output" == *"untracked.txt"* ]]
}

@test "auto scope with ONLY untracked changes falls back to HEAD (inherited parity edge, pinned)" {
    ( cd "$TMP_DIR" && echo seed > seed.txt )
    seed_commit "initial"
    ( cd "$TMP_DIR" && echo x > tracked.txt )
    seed_commit "add tracked"
    ( cd "$TMP_DIR" && echo y > only-untracked.txt )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files auto"
    [ "$status" -eq 0 ]
    # Inherited from the source gates (preamble :95-104, jsonl :263-272): the
    # untracked-only worktree is NOT listed under auto; use scope=worktree.
    [[ "$output" != *"only-untracked.txt"* ]]
    [[ "$output" == *"tracked.txt"* ]]
}

@test "load_base with an UNRESOLVABLE base ref fails loudly (rc 2) instead of standing down the guard" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'old\ngrown\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned not-a-real-ref"
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not resolve"* ]]
}

@test "load_base fails loudly (rc 2) when the pinned file EXISTS at base but git show fails (corruption class)" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned )
    seed_commit "pin baseline"
    # git shim: rev-parse/ls-tree pass through, show hard-fails (exit 128) —
    # the refuter's repro: a read failure on an EXISTING base path must not be
    # mistaken for a legal initial snapshot.
    mkdir -p "$TMP_DIR/bin"
    cat > "$TMP_DIR/bin/git" <<SHIM
#!/usr/bin/env bash
if [ "\$1" = "show" ]; then echo "fatal: injected read failure" >&2; exit 128; fi
exec /usr/bin/env -u PATH PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin" git "\$@"
SHIM
    chmod +x "$TMP_DIR/bin/git"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && PATH=\"$TMP_DIR/bin:\$PATH\" ratchet_load_base pinned HEAD"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to stand down"* ]]
}

@test "load_base first-parent-of-root (HEAD^ on the initial commit) is a legal no-base state" {
    ( cd "$TMP_DIR" && printf 'first\n' > pinned )
    seed_commit "root"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD^ && ratchet_assert_shrink_only pinned raw"
    [ "$status" -eq 0 ]
}

@test "base_ref maps scopes: head->HEAD^, staged/worktree->HEAD" {
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_base_ref head"
    [ "$output" = "HEAD^" ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_base_ref staged"
    [ "$output" = "HEAD" ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_base_ref worktree"
    [ "$output" = "HEAD" ]
}

# --- added-hunk guard --------------------------------------------------------

@test "added_hunk_matches trips when an added line matches the ERE" {
    ( cd "$TMP_DIR" && printf 'package x\n' > f.go )
    seed_commit "base f.go"
    ( cd "$TMP_DIR" && printf 'package x\nfunc a() { os.Rename(p, q) }\n' > f.go && git add f.go && git commit -qm "add rename" )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_added_hunk_matches head f.go 'os\\.Rename\\('"
    [ "$status" -eq 0 ]
}

@test "added_hunk_matches does NOT trip when the edit adds no matching line" {
    ( cd "$TMP_DIR" && printf 'package x\nfunc a() { os.Rename(p, q) }\n' > f.go )
    seed_commit "base with rename"
    ( cd "$TMP_DIR" && printf 'package x\nfunc a() { os.Rename(p, q) }\n// harmless comment\n' > f.go && git add f.go && git commit -qm "comment only" )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_added_hunk_matches head f.go 'os\\.Rename\\('"
    [ "$status" -ne 0 ]
}

@test "added_hunk_matches treats an untracked file (worktree scope) as entirely added" {
    ( cd "$TMP_DIR" && printf 'os.Rename(a, b)\n' > new.go )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_added_hunk_matches worktree new.go 'os\\.Rename\\('"
    [ "$status" -eq 0 ]
}

@test "added_hunk_matches with a DYING awk is rc 2 (helper failure never reads as no-match)" {
    # Pin for the atomic-write CI false-PASS class (run 29785505667): a scan
    # helper killed under parallel-runner load must be a loud rc 2, not rc 1 —
    # rc 1 lets a consumer gate certify an unchecked diff as clean.
    ( cd "$TMP_DIR" && printf 'package x\n' > f.go )
    seed_commit "base f.go"
    ( cd "$TMP_DIR" && printf 'package x\nfunc a() { os.Rename(p, q) }\n' > f.go && git add f.go && git commit -qm "add rename" )
    mkdir -p "$TMP_DIR/binawk"
    cat > "$TMP_DIR/binawk/awk" <<'SHIM'
#!/usr/bin/env bash
kill -TERM $$
SHIM
    chmod +x "$TMP_DIR/binawk/awk"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && PATH=\"$TMP_DIR/binawk:\$PATH\" ratchet_added_hunk_matches head f.go 'os\\.Rename\\('"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
}

# --- regenerate --------------------------------------------------------------

@test "regenerate writes header + LC_ALL=C sorted body" {
    run bash -c "cd '$TMP_DIR' && source '$LIB' && _hdr() { printf '# header line one\n# regenerate: cmd\n'; } && _entries() { printf 'zeta\nalpha\nMid\n'; } && ratchet_regenerate pinned _hdr _entries && cat pinned"
    [ "$status" -eq 0 ]
    ( cd "$TMP_DIR" && head -1 pinned | grep -q '^# header line one' )
    # LC_ALL=C sort order: uppercase before lowercase
    data="$(cd "$TMP_DIR" && grep -v '^#' pinned)"
    [ "$data" = "$(printf 'Mid\nalpha\nzeta\n')" ]
}

# --- differential harness smoke ----------------------------------------------

@test "difftest harness: identical scripts are parity; divergent stderr is a mismatch" {
    HARNESS="$REPO_ROOT/tests/scripts/lib/ratchet-difftest.bash"
    [ -f "$HARNESS" ]
    source "$HARNESS"
    cat > "$TMP_DIR/old.sh" <<'EOF'
#!/usr/bin/env bash
echo "out"; echo "err" >&2; exit 3
EOF
    cp "$TMP_DIR/old.sh" "$TMP_DIR/new.sh"
    run ratchet_difftest "$TMP_DIR/old.sh" "$TMP_DIR/new.sh" --
    [ "$status" -eq 0 ]
    cat > "$TMP_DIR/new.sh" <<'EOF'
#!/usr/bin/env bash
echo "out"; echo "DIFFERENT" >&2; exit 3
EOF
    run ratchet_difftest "$TMP_DIR/old.sh" "$TMP_DIR/new.sh" --
    [ "$status" -ne 0 ]
    [[ "$output" == *"stderr"* ]]
}

# --- fail-open class sweep (pawl rounds 1-3): invalid inputs are LOUD ----------

@test "invalid parse-mode is rc 2 from every mode-taking guard function (fail-closed class)" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'old\nnew\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_assert_shrink_only pinned bogus"
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown parse-mode"* ]]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && printf 'x\n' | ratchet_new_violations pinned bogus"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && printf 'x\n' | ratchet_stale_entries pinned bogus"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && _p() { true; } && ratchet_stale_entries_by _p pinned bogus"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_is_pinned old pinned bogus"
    [ "$status" -eq 2 ]
}

@test "regenerate with a missing generator fn is rc 2 and leaves the pinned file untouched" {
    ( cd "$TMP_DIR" && printf '# keep\nprecious\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_regenerate pinned no_such_header no_such_entries"
    [ "$status" -eq 2 ]
    [[ "$output" == *"not a function"* ]]
    [ "$(cat "$TMP_DIR/pinned")" = "$(printf '# keep\nprecious')" ]
}

@test "regenerate with a FAILING entries-fn is rc 2 and leaves the pinned file untouched (atomic)" {
    ( cd "$TMP_DIR" && printf '# keep\nprecious\n' > pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && _h() { printf '# hdr\n'; } && _bad() { return 1; } && ratchet_regenerate pinned _h _bad"
    [ "$status" -eq 2 ]
    [[ "$output" == *"left untouched"* ]]
    [ "$(cat "$TMP_DIR/pinned")" = "$(printf '# keep\nprecious')" ]
}

@test "regenerate with zero entries writes header only (no trailing blank data line)" {
    run bash -c "cd '$TMP_DIR' && source '$LIB' && _h() { printf '# hdr\n'; } && _e() { :; } && ratchet_regenerate pinned _h _e && cat pinned"
    [ "$status" -eq 0 ]
    [ "$output" = "# hdr" ]
}

@test "changed_files head scope on a ROOT commit lists its files (--root; no vacuous certify)" {
    ( cd "$TMP_DIR" && echo x > first.txt )
    seed_commit "root"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files head"
    [ "$status" -eq 0 ]
    [[ "$output" == *"first.txt"* ]]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files_status head"
    [ "$status" -eq 0 ]
    [[ "$output" == *"first.txt"* ]]
}

@test "upstream scope with UNRELATED-history upstream fails loudly (rc 2), never certifies empty" {
    ( cd "$TMP_DIR" && echo a > a.txt )
    seed_commit "main line"
    MAIN_BRANCH="$(cd "$TMP_DIR" && git symbolic-ref --short HEAD)"
    # unrelated-history branch to act as upstream
    ( cd "$TMP_DIR" \
      && git checkout -q --orphan unrelated \
      && git rm -rq . 2>/dev/null || true )
    ( cd "$TMP_DIR" && echo b > b.txt && git add -A && git commit -qm "orphan root" )
    ( cd "$TMP_DIR" && git checkout -q "$MAIN_BRANCH" && git branch -q --set-upstream-to=unrelated )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files upstream"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files_status upstream"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_base_ref upstream"
    [ "$status" -eq 2 ]
}

@test "changed_files_status fails loudly when ls-files fails (git shim; no silent partial changeset)" {
    ( cd "$TMP_DIR" && echo a > a.txt )
    seed_commit "base"
    ( cd "$TMP_DIR" && echo mod >> a.txt && echo new > untracked.txt )
    mkdir -p "$TMP_DIR/bin2"
    cat > "$TMP_DIR/bin2/git" <<SHIM
#!/usr/bin/env bash
if [ "\$1" = "ls-files" ]; then echo "fatal: injected ls-files failure" >&2; exit 128; fi
exec /usr/bin/env -u PATH PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin" git "\$@"
SHIM
    chmod +x "$TMP_DIR/bin2/git"
    run bash -c "cd '$TMP_DIR' && source '$LIB' && PATH=\"$TMP_DIR/bin2:\$PATH\" ratchet_changed_files_status worktree"
    [ "$status" -eq 2 ]
    [[ "$output" == *"ls-files failed"* ]]
}

@test "assert_shrink_only fails loudly (rc 2) when the working pinned file is unreadable" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned )
    seed_commit "pin baseline"
    ( cd "$TMP_DIR" && printf 'old\ngrown\n' > pinned && chmod 000 pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_load_base pinned HEAD && ratchet_assert_shrink_only pinned raw"
    ( cd "$TMP_DIR" && chmod 644 pinned )
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
}

@test "new_violations and stale_entries fail loudly (rc 2) on an unreadable pinned file" {
    ( cd "$TMP_DIR" && printf 'old\n' > pinned && chmod 000 pinned )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && printf 'x\n' | ratchet_new_violations pinned raw"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && printf 'x\n' | ratchet_stale_entries pinned raw"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && _p() { true; } && ratchet_stale_entries_by _p pinned raw"
    [ "$status" -eq 2 ]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_is_pinned old pinned raw"
    [ "$status" -eq 2 ]
    ( cd "$TMP_DIR" && chmod 644 pinned )
}

@test "changed_files head scope on a MERGE commit lists first-parent changes (no vacuous certify)" {
    ( cd "$TMP_DIR" && echo a > a.txt )
    seed_commit "base"
    MAIN_BRANCH="$(cd "$TMP_DIR" && git symbolic-ref --short HEAD)"
    ( cd "$TMP_DIR" && git checkout -qb side && echo s > side.txt && git add -A && git commit -qm side )
    ( cd "$TMP_DIR" && git checkout -q "$MAIN_BRANCH" && git merge -q --no-ff -m "merge side" side )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files head"
    [ "$status" -eq 0 ]
    [[ "$output" == *"side.txt"* ]]
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files_status head"
    [ "$status" -eq 0 ]
    [[ "$output" == *"side.txt"* ]]
}

@test "changed_files_status emits untracked as A<TAB>path — byte-exact (POSIX-portable emitter)" {
    ( cd "$TMP_DIR" && echo seed > seed.txt )
    seed_commit "initial"
    ( cd "$TMP_DIR" && echo y > brandnew.txt )
    run bash -c "cd '$TMP_DIR' && source '$LIB' && ratchet_changed_files_status worktree"
    [ "$status" -eq 0 ]
    expected="$(printf 'A\tbrandnew.txt')"
    [[ "$output" == *"$expected"* ]]
    # and NOT the BSD-sed literal-t corruption shape
    [[ "$output" != *"Atbrandnew.txt"* ]]
}
