#!/usr/bin/env bats
# Regression tests for scripts/audit-bd-memories.sh (soc-lgq4).
#
# The script shells out to `bd memories`. We stub that binary via PATH
# so tests get deterministic input without hitting the real dolt store.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/audit-bd-memories.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# stub_bd <memories-output-file> — write a bd shim that emits the file.
stub_bd() {
  local out_file="$1"
  cat >"$TMP/bin/bd" <<EOF
#!/usr/bin/env bash
if [ "\$1" = "memories" ]; then
  cat "$out_file"
  exit 0
fi
exit 0
EOF
  chmod +x "$TMP/bin/bd"
  export PATH="$TMP/bin:$ORIG_PATH"
}

# Common synthetic memory corpus.
write_corpus_basic() {
  cat >"$TMP/mems.txt" <<'EOF'
Memories (4):

  alpha-one
    The quick brown fox jumps over the lazy dog repeatedly.

  alpha-two
    The quick brown fox jumps over the lazy dog repeatedly.

  beta-distinct
    Completely unrelated content about systemd timers and journald.

  retired-mention
    Old lesson about ollama gemma morai-codex pipelines that no longer apply.
EOF
}

run_audit() {
  cd "$TMP"
  run "$SCRIPT" "$@"
}

@test "exits 3 when no bd memories present" {
  cat >"$TMP/mems.txt" <<'EOF'
Memories (0):

EOF
  stub_bd "$TMP/mems.txt"
  run_audit --json
  [ "$status" -eq 3 ]
}

@test "--json reports counts on a 4-memory corpus" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  run_audit --json
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.total == 4' >/dev/null
  # alpha-one and alpha-two are byte-identical → 1.0 jaccard, definitely above threshold.
  echo "$output" | jq -e '.near_duplicates >= 1' >/dev/null
  # retired-mention contains "ollama" → matches default pattern.
  echo "$output" | jq -e '.retired_candidates >= 1' >/dev/null
}

@test "default markdown output lands under .agents/audits/" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  # We need .agents/ to be writable; the script creates the audit dir.
  run_audit
  [ "$status" -eq 0 ]
  files=$(ls "$TMP/.agents/audits/bd-memories-"*.md 2>/dev/null | wc -l | tr -d ' ')
  [ "$files" -eq 1 ]
}

@test "--stdout emits markdown instead of writing a file" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  run_audit --stdout
  [ "$status" -eq 0 ]
  [[ "$output" == *"# bd memories audit"* ]]
  [[ "$output" == *"## Near-duplicates"* ]]
  [[ "$output" == *"## Retired-surface candidates"* ]]
  # No file should have been written.
  ! ls "$TMP/.agents/audits/bd-memories-"*.md 2>/dev/null
}

@test "near-duplicates table includes the duplicate keys" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  run_audit --stdout
  [ "$status" -eq 0 ]
  [[ "$output" == *"alpha-one"* ]]
  [[ "$output" == *"alpha-two"* ]]
}

@test "--threshold 0.99 raises the bar; identical pairs still pass, near misses don't" {
  cat >"$TMP/mems.txt" <<'EOF'
Memories (2):

  a-mostly-same
    apple banana cherry date elderberry fig grape

  b-mostly-same
    apple banana cherry date elderberry fig pear
EOF
  stub_bd "$TMP/mems.txt"
  run_audit --threshold 0.99 --json
  [ "$status" -eq 0 ]
  # 6 of 8 unique words shared = 0.75 jaccard → below 0.99.
  echo "$output" | jq -e '.near_duplicates == 0' >/dev/null
}

@test "--no-dups suppresses near-duplicate scanning entirely" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  run_audit --stdout --no-dups
  [ "$status" -eq 0 ]
  [[ "$output" != *"## Near-duplicates"* ]]
}

@test "--no-retired suppresses retired-surface section" {
  write_corpus_basic
  stub_bd "$TMP/mems.txt"
  run_audit --stdout --no-retired
  [ "$status" -eq 0 ]
  [[ "$output" != *"## Retired-surface candidates"* ]]
}

@test "--retired <csv> overrides default retired-keyword list" {
  cat >"$TMP/mems.txt" <<'EOF'
Memories (2):

  a-clean
    nothing notable about this one

  b-special
    this memory mentions cobalt-strike very loudly
EOF
  stub_bd "$TMP/mems.txt"
  run_audit --stdout --retired "cobalt-strike"
  [ "$status" -eq 0 ]
  [[ "$output" == *"b-special"* ]]
  [[ "$output" == *"cobalt-strike"* ]]
}

@test "unknown flag exits 2 with usage error" {
  stub_bd "$TMP/mems.txt"
  run_audit --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}

@test "missing bd binary exits 3" {
  # Don't stub bd; ensure it's not on the test PATH while keeping coreutils.
  mkdir -p "$TMP/coreutils-only"
  for cmd in bash sh sed awk grep tr sort comm cat mkdir mv rm cp ls wc dirname basename head tail printf jq mktemp; do
    full="$(command -v "$cmd" 2>/dev/null || true)"
    [ -n "$full" ] && ln -sf "$full" "$TMP/coreutils-only/$cmd"
  done
  export PATH="$TMP/coreutils-only"
  run_audit --json
  [ "$status" -eq 3 ]
}
