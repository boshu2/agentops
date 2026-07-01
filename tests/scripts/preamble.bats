#!/usr/bin/env bats
# age-0dq9.1: scripts/lib/preamble.sh is a SOURCED library (strict mode +
# REPO_ROOT + portable stat/find). These cases prove the two portability
# properties it exists to centralize — `stat -f`/`stat -c` mtime and a `find`
# that does NOT depend on the macOS interactive `bfs` shim — exercised against
# the real system find, plus a case that PROVES portable_find bypasses a broken
# `find` on PATH (the exact bug class: hand-tests hit the shim, scripts hit the
# real find).

setup() {
  LIB="$BATS_TEST_DIRNAME/../../scripts/lib/preamble.sh"
  FIX="$(mktemp -d)"
}

teardown() {
  rm -rf "$FIX"
}

@test "preamble: sources clean and enables strict mode" {
  run bash -c '. "'"$LIB"'"; set -o | grep -E "errexit|nounset" | grep -c "on"'
  [ "$status" -eq 0 ]
  # both errexit and nounset are on
  [ "$output" = "2" ]
}

@test "preamble: REPO_ROOT resolves to the repo toplevel" {
  run bash -c 'cd "'"$BATS_TEST_DIRNAME"'"; . "'"$LIB"'"; echo "$REPO_ROOT"'
  [ "$status" -eq 0 ]
  # the lib lives at <root>/scripts/lib/preamble.sh
  [ -f "$output/scripts/lib/preamble.sh" ]
}

@test "preamble: REPO_ROOT is cwd-independent (lib's repo, not the caller's)" {
  # Make a SEPARATE git repo and cd into it, then source the real lib (which
  # lives in THIS repo). REPO_ROOT must be this repo, not the unrelated one.
  mkdir -p "$FIX/other"
  ( cd "$FIX/other" && git init -q )
  run bash -c 'cd "'"$FIX"'/other"; . "'"$LIB"'"; echo "$REPO_ROOT"'
  [ "$status" -eq 0 ]
  [ -f "$output/scripts/lib/preamble.sh" ]   # resolved to the lib's own repo
  [ "$output" != "$FIX/other" ]              # NOT the caller's checkout
}

@test "preamble: REPO_ROOT falls back outside a git checkout" {
  # Copy the lib into a NON-git temp tree; git rev-parse must fail, fallback wins.
  mkdir -p "$FIX/proj/scripts/lib"
  cp "$LIB" "$FIX/proj/scripts/lib/preamble.sh"
  run bash -c 'cd /tmp; . "'"$FIX"'/proj/scripts/lib/preamble.sh"; echo "$REPO_ROOT"'
  [ "$status" -eq 0 ]
  [ "$output" = "$FIX/proj" ]
}

@test "preamble: portable_mtime returns an epoch number for an existing file" {
  touch "$FIX/a"
  run bash -c '. "'"$LIB"'"; portable_mtime "'"$FIX"'/a"'
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^[0-9]+$ ]]
}

@test "preamble: newest_by_mtime picks the newest of several files" {
  touch -t 202001010000 "$FIX/old"
  touch -t 202501010000 "$FIX/new"
  touch -t 202301010000 "$FIX/mid"
  run bash -c '. "'"$LIB"'"; newest_by_mtime "'"$FIX"'/old" "'"$FIX"'/mid" "'"$FIX"'/new"'
  [ "$status" -eq 0 ]
  [ "$output" = "$FIX/new" ]
}

@test "preamble: newest_by_mtime is empty when no args exist" {
  run bash -c '. "'"$LIB"'"; newest_by_mtime "'"$FIX"'/nope1" "'"$FIX"'/nope2"'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "preamble: newest_in_dir finds the newest matching file via the real find" {
  touch -t 202001010000 "$FIX/x.log"
  touch -t 202501010000 "$FIX/y.log"
  touch -t 202501010000 "$FIX/z.txt"   # newer but wrong glob — must be ignored
  run bash -c '. "'"$LIB"'"; newest_in_dir "'"$FIX"'" "*.log"'
  [ "$status" -eq 0 ]
  [ "$output" = "$FIX/y.log" ]
}

@test "preamble: portable_find bypasses a broken 'find' shadowing it on PATH" {
  # Shadow `find` with a stub that always fails — the exact failure a non-portable
  # script hits. portable_find must still work because it calls /usr/bin/find.
  mkdir -p "$FIX/bin"
  cat > "$FIX/bin/find" <<'EOF'
#!/usr/bin/env bash
echo "BROKEN SHIM find invoked" >&2
exit 3
EOF
  chmod +x "$FIX/bin/find"
  touch "$FIX/target.txt"
  run env PATH="$FIX/bin:$PATH" bash -c '. "'"$LIB"'"; portable_find "'"$FIX"'" -type f -name "target.txt"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"target.txt"* ]]
  [[ "$output" != *"BROKEN SHIM"* ]]
}

@test "preamble: with_tmpdir creates a dir and assigns it to the caller variable" {
  run bash -c '. "'"$LIB"'"; with_tmpdir work; [ -d "$work" ] && echo "$work"'
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  # dir was created under ${TMPDIR:-/tmp} with the default label
  [[ "$output" == *"/agentops."* ]]
}

@test "preamble: with_tmpdir honors a custom label" {
  run bash -c '. "'"$LIB"'"; with_tmpdir cache mylabel; echo "$cache"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"/mylabel."* ]]
}

@test "preamble: with_tmpdir dir is removed by the EXIT trap" {
  # Capture the path the child made, then assert it no longer exists after the
  # child (and its EXIT trap) has finished — proving auto-cleanup.
  run bash -c '. "'"$LIB"'"; with_tmpdir work; printf "%s" "$work" > "'"$FIX"'/path"'
  [ "$status" -eq 0 ]
  made="$(cat "$FIX/path")"
  [ -n "$made" ]
  [ ! -e "$made" ]
}

@test "preamble: with_tmpdir cleans up multiple dirs on exit" {
  run bash -c '. "'"$LIB"'"; with_tmpdir a; with_tmpdir b; printf "%s\n%s\n" "$a" "$b" > "'"$FIX"'/paths"'
  [ "$status" -eq 0 ]
  a="$(sed -n 1p "$FIX/paths")"
  b="$(sed -n 2p "$FIX/paths")"
  [ -n "$a" ] && [ -n "$b" ] && [ "$a" != "$b" ]
  [ ! -e "$a" ]
  [ ! -e "$b" ]
}

@test "preamble: with_tmpdir requires a VARNAME argument" {
  run bash -c '. "'"$LIB"'"; with_tmpdir'
  [ "$status" -ne 0 ]
  [[ "$output" == *"VARNAME required"* ]]
}

@test "preamble: require_cmd succeeds for a command on PATH" {
  # `sh` is guaranteed present in any POSIX environment running bats.
  run bash -c '. "'"$LIB"'"; require_cmd sh && echo OK'
  [ "$status" -eq 0 ]
  [ "$output" = "OK" ]
}

@test "preamble: require_cmd exits 127 with an error for a missing command" {
  run bash -c '. "'"$LIB"'"; require_cmd definitely-not-a-real-command-xyz'
  [ "$status" -eq 127 ]
  [[ "$output" == *"required command not found: definitely-not-a-real-command-xyz"* ]]
}

@test "preamble: require_cmd includes the install hint when given" {
  run bash -c '. "'"$LIB"'"; require_cmd definitely-not-a-real-command-xyz "brew install xyz"'
  [ "$status" -eq 127 ]
  [[ "$output" == *"install with: brew install xyz"* ]]
}

@test "preamble: require_cmd requires a CMD argument" {
  run bash -c '. "'"$LIB"'"; require_cmd'
  [ "$status" -ne 0 ]
  [[ "$output" == *"CMD required"* ]]
}
