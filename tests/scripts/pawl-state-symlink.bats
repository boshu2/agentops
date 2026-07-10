#!/usr/bin/env bats
# age-l3xj (cross-family refuter round 19): the repo-local state dir ($ROOT/$STATE_DIR) is
# written/deleted by up/route/down. On the installed-binary path in an UNTRUSTED repo, that repo
# can commit `.agents` or `.agents/pawl` as a SYMLINK escaping the repo — then session.json/metrics
# writes land outside it and `down` deletes the target's session.json (data loss). Any symlink in
# the ROOT→STATE_DIR chain must be refused before any state write/delete.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  ROOT="$TMP/repo"; mkdir -p "$ROOT"
  STATE_DIR=".agents/pawl"
  # An external target the planted symlink points at, holding an operator sentinel.
  EXT="$TMP/external"; mkdir -p "$EXT"
  echo "operator sentinel" > "$EXT/session.json"
  echo "operator data" > "$EXT/data.txt"
  log() { :; }
}
teardown() { rm -rf "$TMP"; }

@test "_pawl_verify_state_dir: passes for a real in-repo state dir" {
  mkdir -p "$ROOT/.agents/pawl"
  run _pawl_verify_state_dir
  [ "$status" -eq 0 ]
}

@test "_pawl_verify_state_dir: passes when the state dir does not exist yet (mkdir will make real dirs)" {
  run _pawl_verify_state_dir
  [ "$status" -eq 0 ]
}

@test "_pawl_verify_state_dir: REFUSES when .agents/pawl itself is a symlink escaping the repo" {
  mkdir -p "$ROOT/.agents"
  ln -s "$EXT" "$ROOT/.agents/pawl"
  run _pawl_verify_state_dir
  [ "$status" -ne 0 ]
}

@test "_pawl_verify_state_dir: REFUSES when the .agents ANCESTOR is a symlink escaping the repo" {
  ln -s "$EXT" "$ROOT/.agents"
  run _pawl_verify_state_dir
  [ "$status" -ne 0 ]
}

# Round 21: session.json is now a SESSION-scoped shared file (SESSION_JSON, /tmp), NOT under the
# repo state dir. Its leaf-symlink guard is exercised via SESSION_JSON directly.
@test "_write_session_json: a symlinked SESSION_JSON leaf is neutralized, target untouched" {
  SESSION_JSON="$TMP/session.json"
  ln -s "$EXT/session.json" "$SESSION_JSON"           # the session-state leaf is a symlink
  _set_panes_from_enabled cc cod
  _write_session_json
  [ ! -L "$SESSION_JSON" ]                            # the symlink was replaced by a real file
  grep -q '"families":"cc cod"' "$SESSION_JSON"
  grep -q "operator sentinel" "$EXT/session.json"     # external target UNCHANGED
}

@test "cmd_down: does NOT delete through a symlinked SESSION_JSON (leaves target intact)" {
  SESSION_JSON="$TMP/session.json"
  ln -s "$EXT/session.json" "$SESSION_JSON"
  session_exists() { return 0; }
  atm() { return 0; }; tmux() { return 0; }
  run cmd_down --force
  [ -f "$EXT/session.json" ]        # external target SURVIVES (down unlinked the link, not the target)
  grep -q "operator sentinel" "$EXT/session.json"
}

# Round 20: the STATE_DIR chain guard covers ANCESTORS; a LEAF metrics.jsonl committed as a symlink
# must not be followed by the append — unlink the link, leave the target.
@test "route metrics append: a symlinked metrics.jsonl LEAF is neutralized, external file untouched" {
  mkdir -p "$ROOT/.agents/pawl"
  echo "external metrics sentinel" > "$EXT/metrics.jsonl"
  ln -s "$EXT/metrics.jsonl" "$ROOT/.agents/pawl/metrics.jsonl"
  # Exercise the exact guarded append the route uses.
  _pawl_verify_state_dir && mkdir -p "$ROOT/$STATE_DIR"
  _pawl_unlink_if_symlink "$ROOT/$STATE_DIR/metrics.jsonl"
  printf '{"route":1}\n' >> "$ROOT/$STATE_DIR/metrics.jsonl"
  [ ! -L "$ROOT/.agents/pawl/metrics.jsonl" ]         # symlink replaced by a real file
  grep -q '"route":1' "$ROOT/.agents/pawl/metrics.jsonl"
  # the external target was NOT appended to
  [ "$(cat "$EXT/metrics.jsonl")" = "external metrics sentinel" ]
}

@test "_pawl_unlink_if_symlink: leaves a REAL (non-symlink) file alone" {
  mkdir -p "$ROOT/.agents/pawl"
  echo "real prior data" > "$ROOT/.agents/pawl/metrics.jsonl"
  _pawl_unlink_if_symlink "$ROOT/.agents/pawl/metrics.jsonl"
  [ -f "$ROOT/.agents/pawl/metrics.jsonl" ]           # a real file is not removed
  grep -q "real prior data" "$ROOT/.agents/pawl/metrics.jsonl"
}

# Round 24: the route writes verdicts into PAWL_VERDICT_DIR (default <repo>/.agents/pawl-verdicts);
# a committed `.agents/pawl-verdicts` (or `.agents`) symlink would be followed by the verdict write.
@test "_pawl_verdict_dir_safe: passes for a real in-repo verdict dir" {
  PAWL_VERDICT_DIR="$ROOT/.agents/pawl-verdicts"; mkdir -p "$PAWL_VERDICT_DIR"
  run _pawl_verdict_dir_safe
  [ "$status" -eq 0 ]
}

@test "_pawl_verdict_dir_safe: REFUSES a symlinked verdict dir" {
  mkdir -p "$ROOT/.agents"
  ln -s "$EXT" "$ROOT/.agents/pawl-verdicts"
  PAWL_VERDICT_DIR="$ROOT/.agents/pawl-verdicts"
  run _pawl_verdict_dir_safe
  [ "$status" -ne 0 ]
}

@test "_pawl_verdict_dir_safe: REFUSES when the .agents parent is a symlink" {
  ln -s "$EXT" "$ROOT/.agents"
  PAWL_VERDICT_DIR="$ROOT/.agents/pawl-verdicts"
  run _pawl_verdict_dir_safe
  [ "$status" -ne 0 ]
}
