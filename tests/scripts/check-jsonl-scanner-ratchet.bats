#!/usr/bin/env bats
# Tests for scripts/check-jsonl-scanner-ratchet.sh (age-storage-hardening-roxg.3) —
# the ADVISORY changed-scope grep-ratchet that flags a NEW raw bufio.NewScanner
# over JSONL outside cli/internal/storage and pins the current sites in a
# grandfather list.
#
# The pinned contracts:
#   - a NEW cli/**/*.go file with a raw scanner + a .jsonl mention (added hunk)
#       -> exit non-zero, names storage.ScanJSONL.
#   - the SAME file grandfathered                       -> exit 0 (exempt).
#   - a grandfathered file that no longer trips         -> exit non-zero (prune).
#   - a non-JSONL scanner file (scanner but no .jsonl)  -> exit 0 (passes).
#   - a file under cli/internal/storage/                -> exit 0 (exempt).
#
# Every scenario runs against an ISOLATED temp git repo so the real checkout is
# never mutated; the scope is the last commit (--scope head).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-jsonl-scanner-ratchet.sh"
    TMP_DIR="$(mktemp -d)"
    # Build a minimal repo skeleton with the script (and the shared preamble lib
    # it sources — the lib anchors REPO_ROOT at its own dir, so copying it into
    # the skeleton makes REPO_ROOT resolve to the temp repo) in place at scripts/.
    mkdir -p "$TMP_DIR/scripts/lib" "$TMP_DIR/cli/cmd/ao" "$TMP_DIR/cli/internal/storage"
    cp "$SCRIPT" "$TMP_DIR/scripts/check-jsonl-scanner-ratchet.sh"
    cp "$REPO_ROOT/scripts/lib/preamble.sh" "$TMP_DIR/scripts/lib/preamble.sh"
    # shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.3)
    cp "$REPO_ROOT/scripts/lib/ratchet.sh" "$TMP_DIR/scripts/lib/ratchet.sh"
    chmod +x "$TMP_DIR/scripts/check-jsonl-scanner-ratchet.sh"
    (
        cd "$TMP_DIR"
        git init -q
        git config user.email t@t.t
        git config user.name t
        # Seed an initial commit so HEAD~1 exists for --scope head diffs.
        echo "seed" > seed.txt
        git add -A
        git commit -qm seed
    )
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Write a Go file that trips the heuristic: a raw scanner + a .jsonl mention.
write_tripping_go() {
    local path="$1"
    mkdir -p "$TMP_DIR/$(dirname "$path")"
    cat > "$TMP_DIR/$path" <<'GO'
package ao

import (
	"bufio"
	"os"
)

// readLedger scans the run ledger at _beads/issues.jsonl line by line.
func readLedger() {
	f, _ := os.Open("_beads/issues.jsonl")
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		_ = sc.Text()
	}
}
GO
}

run_gate() {
    ( cd "$TMP_DIR" && bash scripts/check-jsonl-scanner-ratchet.sh --scope head )
}

@test "a NEW cli Go file with a raw scanner + .jsonl mention FAILS and names storage.ScanJSONL" {
    write_tripping_go "cli/cmd/ao/new_reader.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader" )
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"new_reader.go"* ]]
    [[ "$output" == *"storage.ScanJSONL"* ]]
    # It must not read as a pass.
    [[ "$output" != *"PASS"* ]]
}

@test "a file grandfathered at the BASE ref PASSES (exempt)" {
    # The grandfather entry must PRE-EXIST at the base ref (committed before the
    # change under review) — entry in a prior commit, tripping file in HEAD.
    printf '# grandfather\ncli/cmd/ao/new_reader.go\n' > "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "grandfather snapshot" )
    write_tripping_go "cli/cmd/ao/new_reader.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader" )
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a same-commit self-allowlist (new site + grandfather entry in one diff) FAILS" {
    # The bypass the shrink-only rule closes: a change ships a new raw-scanner
    # site AND appends its path to the grandfather in the same commit. The
    # grandfather existed (without the entry) at the base ref, so both the
    # growth guard and the intersection authority must reject it.
    printf '# grandfather\n' > "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "empty grandfather snapshot" )
    write_tripping_go "cli/cmd/ao/new_reader.go"
    printf '# grandfather\ncli/cmd/ao/new_reader.go\n' > "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader + self-allowlist" )
    run run_gate
    [ "$status" -ne 0 ]
    # The growth guard names the rule and the smuggled entry...
    [[ "$output" == *"only SHRINKS"* ]]
    [[ "$output" == *"cli/cmd/ao/new_reader.go"* ]]
    # ...and the intersection authority still flags the site as NEW (the
    # same-diff entry grants no protection).
    [[ "$output" == *"storage.ScanJSONL"* ]]
}

@test "a grandfathered file that no longer trips FAILS demanding a prune" {
    # Grandfather a path, but the file at that path does NOT trip (no scanner).
    mkdir -p "$TMP_DIR/cli/cmd/ao"
    cat > "$TMP_DIR/cli/cmd/ao/cleaned.go" <<'GO'
package ao

// cleaned.go once used a raw scanner over _beads/foo.jsonl; migrated to
// storage.ScanJSONLFile, so it no longer trips the heuristic.
func cleaned() {}
GO
    printf '# grandfather\ncli/cmd/ao/cleaned.go\n' > "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "grandfather a cleaned file" )
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"cleaned.go"* ]]
    [[ "$output" == *"prune"* ]]
}

@test "a non-JSONL scanner file (scanner but no .jsonl) PASSES" {
    mkdir -p "$TMP_DIR/cli/cmd/ao"
    cat > "$TMP_DIR/cli/cmd/ao/plain_reader.go" <<'GO'
package ao

import (
	"bufio"
	"os"
)

// reads a plain .txt log; NO jsonl anywhere.
func readLog() {
	f, _ := os.Open("run.txt")
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		_ = sc.Text()
	}
}
GO
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add plain_reader" )
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a file under cli/internal/storage/ is EXEMPT (blessed impl home)" {
    write_tripping_go "cli/internal/storage/scanjsonl.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add storage scanner" )
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# --- collector fail-closed witnesses (advisory F2) -----------------------------

# `done < <(collect_changed_files | sort -u)` discards the collector's
# fail-closed rc 2 even under `set -euo pipefail`: a dead git read becomes an
# empty loop and the gate certifies. The repository's own defense for this trap
# is the capture pattern documented in check-atomic-write-ratchet.sh. Both
# witnesses seed a REAL violation first, so a swallowed failure would have to
# produce a visibly wrong green. The shim targets `git diff-tree`, the
# head-scope collection command, so the witness encodes the contract, not
# either implementation.
shim_git_diff_tree() {
    local emit_row="${1:-}"
    mkdir -p "$TMP_DIR/bin"
    {
        echo '#!/usr/bin/env bash'
        echo 'if [ "$1" = "diff-tree" ]; then'
        if [[ -n "$emit_row" ]]; then
            printf '  printf "%s"\n' "$emit_row"
        fi
        echo '  echo "fatal: injected diff-tree failure" >&2'
        echo '  exit 128'
        echo 'fi'
        echo 'exec /usr/bin/env -u PATH PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin" git "$@"'
    } > "$TMP_DIR/bin/git"
    chmod +x "$TMP_DIR/bin/git"
}

@test "a failing changed-scope collector exits 2, never a silent PASS" {
    write_tripping_go "cli/cmd/ao/new_reader.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader" )
    shim_git_diff_tree
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --scope head"
    [ "$status" -eq 2 ]
    # The gate's own refusal must be loud, and the library's stderr preserved.
    [[ "$output" == *"refusing to certify"* ]]
    [[ "$output" == *"diff-tree"* ]]
    # A swallowed failure would certify green over an unchecked diff.
    [[ "$output" != *"PASS"* ]]
}

@test "a PARTIAL collector (row emitted, then death) is never certified against" {
    write_tripping_go "cli/cmd/ao/new_reader.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader" )
    # One plausible path row, then death: truncated bytes must not be processed.
    shim_git_diff_tree 'cli/cmd/ao/ghost.go\n'
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --scope head"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
    [[ "$output" != *"ghost.go"* ]]
    [[ "$output" != *"PASS"* ]]
}

# --- per-file scan-chain fail-closed witnesses (scan-chain successor intent) ---

# The lib's added-hunk matcher is deliberately tri-state (1 = no added match,
# 2 = scan helper failed, loud) and its own comment warns that `|| continue`
# callers still skip on rc 2. These witnesses distinguish the two classes:
# a helper DEATH must refuse (exit 2), a genuine no-match must skip (rc 1
# semantics unchanged), and a matcher-internal git death must keep flagging
# via the whole-file degradation (fail-safe, not false-green). Each witness is
# baseline-first: the sane run must flag the seeded violation before any
# hostility is applied, so a later green cannot be blamed on the fixture.

seed_violation_and_expect_baseline() {
    write_tripping_go "cli/cmd/ao/new_reader.go"
    ( cd "$TMP_DIR" && git add -A && git commit -qm "add new_reader" )
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"new_reader.go"* ]]
}

@test "an added-hunk scan-helper death exits 2, never a skip into green" {
    seed_violation_and_expect_baseline
    # awk shim: dies ONLY when the lib's added-hunk ERE rides the environment
    # (the lib documents that the pattern travels via RATCHET_HUNK_ERE);
    # every other awk call is untouched.
    mkdir -p "$TMP_DIR/bin"
    cat > "$TMP_DIR/bin/awk" <<'SHIM'
#!/usr/bin/env bash
if [ -n "${RATCHET_HUNK_ERE:-}" ]; then echo "awk: injected scan death" >&2; exit 3; fi
exec /usr/bin/awk "$@"
SHIM
    chmod +x "$TMP_DIR/bin/awk"
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --scope head"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
    # The lib's own loud stderr must be preserved.
    [[ "$output" == *"added-hunk scan failed"* ]]
    [[ "$output" != *"PASS"* ]]
}

@test "a whole-file scan death in file_trips exits 2, never a skip into green" {
    seed_violation_and_expect_baseline
    # grep shim: dies ONLY on the gate's \.jsonl whole-file pattern; every
    # other grep call is untouched.
    mkdir -p "$TMP_DIR/bin"
    cat > "$TMP_DIR/bin/grep" <<'SHIM'
#!/usr/bin/env bash
for arg in "$@"; do
    if [ "$arg" = '\.jsonl' ]; then echo "grep: injected scan death" >&2; exit 3; fi
done
exec /usr/bin/grep "$@"
SHIM
    chmod +x "$TMP_DIR/bin/grep"
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --scope head"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
    [[ "$output" != *"PASS"* ]]
}

@test "a mid-loop dead git diff still FLAGS via whole-file degradation (fail-safe, not false-green)" {
    seed_violation_and_expect_baseline
    # git shim: kills plain `git diff` (the matcher's per-file command) while
    # rev-list and diff-tree — the collection commands — pass through. The lib
    # swallows the death into an empty diff and degrades to a whole-file scan,
    # which for a file that already passed file_trips MATCHES: the violation
    # must still be named at rc 1. This pins the degraded path as over-report,
    # the safe direction — the fix may not satisfy the suite by reporting less.
    mkdir -p "$TMP_DIR/bin"
    cat > "$TMP_DIR/bin/git" <<'SHIM'
#!/usr/bin/env bash
if [ "$1" = "diff" ]; then echo "fatal: injected diff failure" >&2; exit 128; fi
exec /usr/bin/env -u PATH PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin" git "$@"
SHIM
    chmod +x "$TMP_DIR/bin/git"
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --scope head"
    [ "$status" -eq 1 ]
    [[ "$output" == *"new_reader.go"* ]]
    [[ "$output" != *"PASS"* ]]
}

# --- regenerate fail-closed witnesses (regenerate successor intent) ------------

# `--regenerate` is a MUTATING success: its exit 0 rewrites the grandfather.
# The enumeration behind it swallowed helper death (`grep -rl … || true` and a
# per-file `grep -q || continue`), so a dead grep rewrote a POPULATED snapshot
# to header-only at exit 0 — silent mass un-pinning that no prune guard ever
# surfaces (vanished entries are legal shrinkage). Each witness is
# baseline-first: it proves the sane regenerate writes real entries on its own
# fixture, snapshots those bytes, applies ONE hostility, and requires nonzero
# exit + byte-identical grandfather afterward.

regenerate_baseline() {
    write_tripping_go "cli/cmd/ao/a_reader.go"
    write_tripping_go "cli/cmd/ao/b_reader.go"
    run bash -c "cd '$TMP_DIR' && bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    [ "$status" -eq 0 ]
    grep -q 'cli/cmd/ao/a_reader.go' "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    grep -q 'cli/cmd/ao/b_reader.go' "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    cp "$TMP_DIR/scripts/.jsonl-scanner-grandfather" "$TMP_DIR/grandfather.before"
}

grep_shim() {
    # Die when the FIRST argument matches the trigger; pass everything else to
    # the real grep. "-rl" selects candidate enumeration; a '\.jsonl' first
    # pattern arg is matched via the trigger appearing anywhere in "$@".
    local trigger="$1" emit="${2:-}"
    mkdir -p "$TMP_DIR/bin"
    {
        echo '#!/usr/bin/env bash'
        echo "trigger='$trigger'"
        echo 'for arg in "$@"; do'
        echo '  if [ "$arg" = "$trigger" ]; then'
        if [[ -n "$emit" ]]; then
            printf '    printf "%s"\n' "$emit"
        fi
        echo '    echo "grep: injected enumeration death" >&2'
        echo '    exit 3'
        echo '  fi'
        echo 'done'
        echo 'exec /usr/bin/grep "$@"'
    } > "$TMP_DIR/bin/grep"
    chmod +x "$TMP_DIR/bin/grep"
}

assert_regenerate_refused() {
    [ "$status" -ne 0 ]
    [[ "$output" == *"refusing to regenerate"* ]]
    [[ "$output" != *"regenerated"* ]]
    cmp -s "$TMP_DIR/grandfather.before" "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
}

@test "--regenerate: a dead candidate enumeration exits nonzero and preserves the snapshot bytes" {
    regenerate_baseline
    grep_shim "-rl"
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    assert_regenerate_refused
}

@test "--regenerate: a PARTIAL enumeration (path emitted, then death) never reaches the snapshot" {
    regenerate_baseline
    grep_shim "-rl" 'cli/cmd/ao/a_reader.go\n'
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    assert_regenerate_refused
}

@test "--regenerate: a dead per-file whole-file scan exits nonzero and preserves the snapshot bytes" {
    regenerate_baseline
    grep_shim '\.jsonl'
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    assert_regenerate_refused
}

@test "--regenerate rewrites the grandfather list from the current tree" {
    # Two tripping files present; --regenerate should list both, sorted, with a header.
    write_tripping_go "cli/cmd/ao/a_reader.go"
    write_tripping_go "cli/cmd/ao/b_reader.go"
    run bash -c "cd '$TMP_DIR' && bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    [ "$status" -eq 0 ]
    [ -f "$TMP_DIR/scripts/.jsonl-scanner-grandfather" ]
    grep -q 'cli/cmd/ao/a_reader.go' "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    grep -q 'cli/cmd/ao/b_reader.go' "$TMP_DIR/scripts/.jsonl-scanner-grandfather"
    # header comment present
    head -1 "$TMP_DIR/scripts/.jsonl-scanner-grandfather" | grep -q '^#'
}
