#!/usr/bin/env bats
# Tests for scripts/check-atomic-write-ratchet.sh (age-ratchet-lib-extraction-bv7d.9)
# — the ADVISORY changed-scope ratchet on hand-rolled tmp+rename atomic writes
# outside cli/internal/storage, the first NEW consumer of scripts/lib/ratchet.sh.
#
# The fixture corpus below is PRE-REGISTERED (premortem rounds 2-3): the
# positive shapes are real in-tree writers (scenarioresults/writer.go:124-131,
# config/config.go:694-701, agentworker/quarantine.go:69-81) and the negatives
# are real plain-movers (search/util.go, doctor/engine.go) plus the
# comment-strip exerciser the in-tree doctor/mutate.go could not provide (its
# comment lacks the paren). The detector boundary was decided at plan time,
# not during implementation.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-atomic-write-ratchet.sh"
    TMP_DIR="$(mktemp -d)"
    mkdir -p "$TMP_DIR/scripts/lib" "$TMP_DIR/cli/internal/thing" "$TMP_DIR/cli/internal/storage"
    cp "$SCRIPT" "$TMP_DIR/scripts/check-atomic-write-ratchet.sh"
    cp "$REPO_ROOT/scripts/lib/preamble.sh" "$TMP_DIR/scripts/lib/preamble.sh"
    cp "$REPO_ROOT/scripts/lib/ratchet.sh" "$TMP_DIR/scripts/lib/ratchet.sh"
    chmod +x "$TMP_DIR/scripts/check-atomic-write-ratchet.sh"
    (
        cd "$TMP_DIR"
        git init -q
        git config user.email t@t.t
        git config user.name t
        echo seed > seed.txt
        git add -A
        git commit -qm seed
    )
}

teardown() {
    rm -rf "$TMP_DIR"
}

run_gate() {
    ( cd "$TMP_DIR" && bash scripts/check-atomic-write-ratchet.sh --scope head )
}

commit_all() {
    ( cd "$TMP_DIR" && git add -A && git commit -qm "${1:-change}" )
}

# --- pre-registered POSITIVE corpus -------------------------------------------

write_writer_shape() {  # cli/internal/scenarioresults/writer.go:124-131
    mkdir -p "$TMP_DIR/$(dirname "$1")"
    cat > "$TMP_DIR/$1" <<'GO'
package thing

import "os"

func writeArtifact(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}
GO
}

write_config_shape() {  # cli/internal/config/config.go:694-701
    mkdir -p "$TMP_DIR/$(dirname "$1")"
    cat > "$TMP_DIR/$1" <<'GO'
package thing

import "os"

func saveConfig(path string, data []byte) error {
	// Write atomically: temp file then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
GO
}

write_quarantine_shape() {  # cli/internal/agentworker/quarantine.go:69-81
    mkdir -p "$TMP_DIR/$(dirname "$1")"
    cat > "$TMP_DIR/$1" <<'GO'
package thing

import "os"

func quarantine(path string, body []byte) (string, error) {
	tmp, err := os.CreateTemp("", "quarantine-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(body); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}
GO
}

# --- pre-registered NEGATIVE corpus -------------------------------------------

write_plain_move() {  # search/util.go:29-38 / doctor/engine.go:597-613 shape
    mkdir -p "$TMP_DIR/$(dirname "$1")"
    cat > "$TMP_DIR/$1" <<'GO'
package thing

import "os"

func relocate(src, dst string) error {
	return os.Rename(src, dst)
}
GO
}

@test "positive corpus 1: writer.go shape (\".tmp + WriteFile + Rename) trips as a new hunk" {
    write_writer_shape "cli/internal/thing/writer.go"
    commit_all "add writer"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/thing/writer.go"* ]]
    [[ "$output" == *"storage.AtomicWriteFile"* ]]
}

@test "positive corpus 2: config.go shape trips" {
    write_config_shape "cli/internal/thing/config.go"
    commit_all "add config writer"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/thing/config.go"* ]]
}

@test "positive corpus 3: quarantine.go shape (os.CreateTemp + Rename) trips" {
    write_quarantine_shape "cli/internal/thing/quarantine.go"
    commit_all "add quarantine"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/thing/quarantine.go"* ]]
}

@test "negative corpus: a plain os.Rename move (no temp signal) does NOT trip" {
    write_plain_move "cli/internal/thing/mover.go"
    commit_all "add mover"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "negative corpus: comment-strip exerciser — os.CreateTemp( in a // comment never counts as the second signal" {
    mkdir -p "$TMP_DIR/cli/internal/thing"
    cat > "$TMP_DIR/cli/internal/thing/mutate.go" <<'GO'
package thing

import "os"

// The sibling helper uses os.CreateTemp( under the hood; this one only moves.
func swapIn(src, dst string) error {
	return os.Rename(src, dst)
}
GO
    commit_all "add mutate shape"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "storage/ is exempt and _test.go is exempt" {
    write_writer_shape "cli/internal/storage/blessed.go"
    write_writer_shape "cli/internal/thing/thing_test.go"
    commit_all "add exempt files"
    run run_gate
    [ "$status" -eq 0 ]
}

@test "a grandfathered file edited WITHOUT a new rename hunk passes" {
    write_writer_shape "cli/internal/thing/old.go"
    printf 'cli/internal/thing/old.go\n' > "$TMP_DIR/scripts/.atomic-write-grandfather"
    commit_all "seed grandfathered"
    ( cd "$TMP_DIR" && printf '\n// harmless doc comment\n' >> cli/internal/thing/old.go )
    commit_all "harmless edit"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a grandfathered file adding a NEW os.Rename hunk STILL trips (no grandfather-skips-first)" {
    write_writer_shape "cli/internal/thing/old.go"
    printf 'cli/internal/thing/old.go\n' > "$TMP_DIR/scripts/.atomic-write-grandfather"
    commit_all "seed grandfathered"
    cat >> "$TMP_DIR/cli/internal/thing/old.go" <<'GO'

func writeSecond(path string, data []byte) error {
	tmp := path + ".tmp2"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
GO
    commit_all "add second rename site"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/thing/old.go"* ]]
}

@test "a stale grandfather entry (file no longer trips) FAILS demanding a prune" {
    write_plain_move "cli/internal/thing/cleaned.go"
    printf 'cli/internal/thing/cleaned.go\n' > "$TMP_DIR/scripts/.atomic-write-grandfather"
    commit_all "stale entry"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"prune"* ]]
    [[ "$output" == *"cli/internal/thing/cleaned.go"* ]]
}

@test "a same-diff self-allowlist is rejected (growth guard ON for this new gate)" {
    ( cd "$TMP_DIR" && printf '# gf\n' > scripts/.atomic-write-grandfather )
    commit_all "empty grandfather"
    write_writer_shape "cli/internal/thing/sneaky.go"
    ( cd "$TMP_DIR" && printf 'cli/internal/thing/sneaky.go\n' >> scripts/.atomic-write-grandfather )
    commit_all "sneak in with allowlist"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"only SHRINKS"* ]]
    [[ "$output" == *"sneaky.go"* ]]
}

@test "--regenerate writes header + sorted current offender set atomically" {
    write_writer_shape "cli/internal/thing/zzz.go"
    write_config_shape "cli/internal/thing/aaa.go"
    commit_all "two writers"
    run bash -c "cd '$TMP_DIR' && bash scripts/check-atomic-write-ratchet.sh --regenerate"
    [ "$status" -eq 0 ]
    ( cd "$TMP_DIR" && head -1 scripts/.atomic-write-grandfather | grep -q '^# scripts/.atomic-write-grandfather' )
    data="$(cd "$TMP_DIR" && grep -v '^#' scripts/.atomic-write-grandfather)"
    [ "$data" = "$(printf 'cli/internal/thing/aaa.go\ncli/internal/thing/zzz.go')" ]
}

@test "usage error on a bogus scope is rc 2" {
    run bash -c "cd '$TMP_DIR' && bash scripts/check-atomic-write-ratchet.sh --scope bogus"
    [ "$status" -eq 2 ]
}

@test "committed repo snapshot: the seeded grandfather + clean head passes" {
    # Other Bats files run concurrently and may temporarily mutate the checkout.
    # Seed this fixture from committed blobs so the ratchet sees one stable tree.
    git -C "$REPO_ROOT" show HEAD:scripts/.atomic-write-grandfather \
        > "$TMP_DIR/scripts/.atomic-write-grandfather"
    while IFS= read -r path; do
        [[ -z "$path" || "$path" == \#* ]] && continue
        mkdir -p "$TMP_DIR/$(dirname "$path")"
        git -C "$REPO_ROOT" show "HEAD:$path" > "$TMP_DIR/$path"
    done < "$TMP_DIR/scripts/.atomic-write-grandfather"
    commit_all "seed committed grandfather snapshot"
    ( cd "$TMP_DIR" && echo clean > snapshot-marker.txt )
    commit_all "clean head"

    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "negative corpus: an INLINE // comment tail carrying os.CreateTemp( never counts" {
    mkdir -p "$TMP_DIR/cli/internal/thing"
    cat > "$TMP_DIR/cli/internal/thing/inline.go" <<'GO'
package thing

import "os"

func swap(src, dst string) error { // like os.CreateTemp( but just a move
	return os.Rename(src, dst)
}
GO
    commit_all "inline comment"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "negative corpus: a /* block comment */ carrying the temp signal never counts" {
    mkdir -p "$TMP_DIR/cli/internal/thing"
    cat > "$TMP_DIR/cli/internal/thing/block.go" <<'GO'
package thing

import "os"

/*
The old implementation wrote to a ".tmp file via os.CreateTemp(
before renaming. This one only moves.
*/
func relocateB(src, dst string) error {
	return os.Rename(src, dst)
}
GO
    commit_all "block comment"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a grandfathered file adding only a // comment that mentions os.Rename( does NOT trip" {
    write_writer_shape "cli/internal/thing/old.go"
    printf 'cli/internal/thing/old.go\n' > "$TMP_DIR/scripts/.atomic-write-grandfather"
    commit_all "seed grandfathered"
    ( cd "$TMP_DIR" && printf '\n// consider os.Rename( semantics when refactoring\n' >> cli/internal/thing/old.go )
    commit_all "comment-only edit"
    run run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "adding only the temp-signal half to a file with an EXISTING rename still trips (round-4 contract)" {
    write_plain_move "cli/internal/thing/gainer.go"
    commit_all "plain mover baseline"
    cat > "$TMP_DIR/cli/internal/thing/gainer.go" <<'GO'
package thing

import "os"

func relocate(src, dst string) error {
	return os.Rename(src, dst)
}

func writeViaRelocate(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return relocate(tmp, path)
}
GO
    commit_all "add temp writer reusing existing rename"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/thing/gainer.go"* ]]
}

@test "a dying scan helper is rc 2 refusing to certify — never a silent PASS (CI flake 29785505667)" {
    # Root of the one-off CI false-PASS: a helper (awk/grep) killed or failing
    # under parallel-runner load used to read as "no signal" and the gate
    # printed a clean PASS over an unchecked diff. Pin the tri-state: helper
    # death is loud rc 2, never certification.
    write_writer_shape "cli/internal/thing/writer.go"
    commit_all "add writer"
    mkdir -p "$TMP_DIR/bin"
    cat > "$TMP_DIR/bin/awk" <<'SHIM'
#!/usr/bin/env bash
# Simulate the CI class: the scan helper dies (stray signal / fork pressure).
kill -TERM $$
SHIM
    chmod +x "$TMP_DIR/bin/awk"
    run bash -c "cd '$TMP_DIR' && PATH=\"$TMP_DIR/bin:\$PATH\" bash scripts/check-atomic-write-ratchet.sh --scope head"
    [ "$status" -eq 2 ]
    [[ "$output" == *"refusing to certify"* ]]
    [[ "$output" != *"PASS"* ]]
}
