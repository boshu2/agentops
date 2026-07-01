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
