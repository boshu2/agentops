#!/usr/bin/env bats
# Hermetic regression tests for scripts/check-epic-children-closed.sh (ag-9gac).
#
# The script shells out to `bd`. We stub it via PATH so the gate is tested
# without a real bead database. `jq` is used real.
#
# Stub contract:
#   bd dep list <epic> --direction=up -t parent-child --json
#       -> prints $TMP/children (a JSON array of {id})
#   bd show <child> --json
#       -> prints [{"id":...,"status":...}] read from $TMP/status/<child>

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

stub_bd() {
  cat >"$TMP/bin/bd" <<EOF
#!/usr/bin/env bash
STATUS_DIR="$TMP/status"
if [ "\$1" = "dep" ] && [ "\$2" = "list" ]; then
  cat "$TMP/children"
  exit 0
fi
if [ "\$1" = "show" ]; then
  child="\$2"
  if [ -f "\$STATUS_DIR/\$child" ]; then
    st="\$(cat "\$STATUS_DIR/\$child")"
    printf '[{"id":"%s","status":"%s"}]' "\$child" "\$st"
  fi
  exit 0
fi
exit 0
EOF
  chmod +x "$TMP/bin/bd"
  export PATH="$TMP/bin:$ORIG_PATH"
}

set_child() { echo "$2" > "$TMP/status/$1"; }

run_gate() { run "$SCRIPT" "$@"; }

@test "all children closed: exit 0" {
  printf '%s' '[{"id":"ag-c1"},{"id":"ag-c2"}]' > "$TMP/children"
  set_child ag-c1 closed
  set_child ag-c2 closed
  stub_bd
  run_gate ag-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"all children of ag-epic are closed"* ]]
}

@test "one open child: exit 1 and names the offender" {
  printf '%s' '[{"id":"ag-c1"},{"id":"ag-c2"}]' > "$TMP/children"
  set_child ag-c1 closed
  set_child ag-c2 open
  stub_bd
  run_gate ag-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"OPEN-CHILD: ag-c2 status=open"* ]]
  [[ "$output" == *"EPIC-GATE FAIL"* ]]
}

@test "in_progress child: exit 1 and names the offender" {
  printf '%s' '[{"id":"ag-c1"}]' > "$TMP/children"
  set_child ag-c1 in_progress
  stub_bd
  run_gate ag-epic
  [ "$status" -eq 1 ]
  [[ "$output" == *"OPEN-CHILD: ag-c1 status=in_progress"* ]]
}

@test "no children: exit 0" {
  printf '%s' '[]' > "$TMP/children"
  stub_bd
  run_gate ag-epic
  [ "$status" -eq 0 ]
  [[ "$output" == *"no open children"* ]]
}

@test "bd dep list error is surfaced: exit 4" {
  printf '%s' '{"error":"schema drift","schema_version":1}' > "$TMP/children"
  stub_bd
  run_gate ag-epic
  [ "$status" -eq 4 ]
  [[ "$output" == *"bd dep list failed"* ]]
}

@test "missing epic-id exits 4" {
  printf '%s' '[]' > "$TMP/children"
  stub_bd
  run_gate
  [ "$status" -eq 4 ]
  [[ "$output" == *"need an epic-id"* ]]
}
