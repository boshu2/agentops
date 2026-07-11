#!/usr/bin/env bats
# Hermetic regression tests for scripts/check-epic-children-closed.sh
# (ag-9gac; tracker-agnostic rewrite age-5w8fd).
#
# The script shells out to `ao beads exec` (the ONE tracker-agnostic entry
# point). We stub `ao` via PATH so the gate is tested without a real bead
# ledger. `jq` is used real.
#
# Stub contract (mirrors the real ao surface):
#   ao beads exec children <epic> --json
#       -> prints $TMP/children verbatim; exits $TMP/children_rc if present.
#          br emits plain child ids one per line; bd (--json forwarded
#          verbatim) emits a JSON array of issue objects. Both shapes are
#          exercised below.
#   ao beads exec show <child> --json
#       -> prints [{"id":...,"status":...}] read from $TMP/status/<child>
#          (the canonical shape ao normalizes both trackers to).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/check-epic-children-closed.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/status"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

stub_ao() {
  cat >"$TMP/bin/ao" <<EOF
#!/usr/bin/env bash
STATUS_DIR="$TMP/status"
if [ "\$1" = "beads" ] && [ "\$2" = "exec" ]; then
  shift 2
  if [ "\$1" = "children" ]; then
    cat "$TMP/children" 2>/dev/null
    if [ -f "$TMP/children_rc" ]; then
      exit "\$(cat "$TMP/children_rc")"
    fi
    exit 0
  fi
  if [ "\$1" = "show" ]; then
    if [ -f "$TMP/show_rc" ]; then
      exit "\$(cat "$TMP/show_rc")"
    fi
    child="\$2"
    if [ -f "\$STATUS_DIR/\$child" ]; then
      st="\$(cat "\$STATUS_DIR/\$child")"
      printf '[{"id":"%s","status":"%s"}]' "\$child" "\$st"
    fi
    exit 0
  fi
fi
exit 0
EOF
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$ORIG_PATH"
}

set_child() { echo "$2" > "$TMP/status/$1"; }

run_gate() { run "$SCRIPT" "$@"; }

# --- br shape: plain child ids, one per line ---

@test "br shape: all children closed: exit 0" {
  printf 'age-c1\nage-c2\n' > "$TMP/children"
  set_child age-c1 closed
  set_child age-c2 closed
  stub_ao
  run_gate age-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"all children of age-epic are closed"* ]]
}

@test "br shape: one open child: exit 1 and names the offender" {
  printf 'age-c1\nage-c2\n' > "$TMP/children"
  set_child age-c1 closed
  set_child age-c2 open
  stub_ao
  run_gate age-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"OPEN-CHILD: age-c2 status=open"* ]]
  [[ "$output" == *"EPIC-GATE FAIL"* ]]
}

@test "br shape: in_progress child: exit 1 and names the offender" {
  printf 'age-c1\n' > "$TMP/children"
  set_child age-c1 in_progress
  stub_ao
  run_gate age-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"OPEN-CHILD: age-c1 status=in_progress"* ]]
}

@test "br shape: no children (empty output): exit 0" {
  : > "$TMP/children"
  stub_ao
  run_gate age-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"no open children"* ]]
}

# --- bd shape: JSON array of issue objects (bd children <epic> --json) ---

@test "bd shape: all children closed: exit 0" {
  printf '%s' '[{"id":"ag-c1","status":"closed"},{"id":"ag-c2","status":"closed"}]' > "$TMP/children"
  set_child ag-c1 closed
  set_child ag-c2 closed
  stub_ao
  run_gate ag-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"all children of ag-epic are closed"* ]]
}

@test "bd shape: one open child: exit 1 and names the offender" {
  printf '%s' '[{"id":"ag-c1","status":"closed"},{"id":"ag-c2","status":"open"}]' > "$TMP/children"
  set_child ag-c1 closed
  set_child ag-c2 open
  stub_ao
  run_gate ag-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"OPEN-CHILD: ag-c2 status=open"* ]]
  [[ "$output" == *"EPIC-GATE FAIL"* ]]
}

@test "bd shape: empty JSON array: exit 0" {
  printf '%s' '[]' > "$TMP/children"
  stub_ao
  run_gate ag-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"no open children"* ]]
}

# --- error surfaces ---

@test "children query non-zero exit is surfaced: exit 4" {
  printf '%s' '' > "$TMP/children"
  echo 1 > "$TMP/children_rc"
  stub_ao
  run_gate age-epic
  [ "$status" -eq 4 ]
  [[ "$output" == *"children query failed"* ]]
}

@test "per-child show failure is fail-closed: offender, exit 1 (not 4)" {
  # Contract: only the CHILDREN-ENUMERATION error is exit 4. A per-child
  # status read failure blocks the close as an offender (fail-closed exit 1)
  # rather than aborting the whole gate.
  printf 'age-c1\n' > "$TMP/children"
  echo 1 > "$TMP/show_rc"
  stub_ao
  run_gate age-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"WARN: could not read status for child age-c1"* ]]
  [[ "$output" == *"EPIC-GATE FAIL"* ]]
}

@test "unreadable child status counts as offender" {
  printf 'age-c1\n' > "$TMP/children"
  # no status file for age-c1 -> stub prints nothing for show
  stub_ao
  run_gate age-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"WARN: could not read status for child age-c1"* ]]
}

@test "missing epic-id exits 4" {
  : > "$TMP/children"
  stub_ao
  run_gate
  [ "$status" -eq 4 ]
  [[ "$output" == *"need an epic-id"* ]]
}
