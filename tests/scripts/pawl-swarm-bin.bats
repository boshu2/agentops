#!/usr/bin/env bats
# pawl-swarm-bin.bats — locks the ntm-first swarm-binary seam (age-hk5zg.3, S3 of the
# pawl-user-front-door packet). The standing pawl-service drives the NTM swarm CLI, whose
# PUBLIC binary name is `ntm`; `atm` is the operator's personal alias into ~/dev/ntm/dist.
# Hardcoding `atm` at the call sites meant the shipped scripts only worked on the
# operator's machine. These tests lock:
#   1. ONE resolver seam (_pawl_swarm_bin): ntm first, atm fallback, PAWL_SWARM_BIN override.
#   2. No bare hardcoded `atm <verb>` invocation remains anywhere in scripts/pawl.sh.
#   3. Every swarm invocation goes through the resolved "$SWARM" binary.
# All pure (no tmux/ntm/atm actually invoked) — the resolver is command -v + env only.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  # shellcheck disable=SC1090
  source "$SCRIPT"
}

teardown() { rm -rf "$TMP"; }

_mk_stub() { # $1 = dir, $2 = name
  mkdir -p "$1"
  printf '#!/usr/bin/env bash\necho "%s"\n' "$2" > "$1/$2"
  chmod +x "$1/$2"
}

# ── 1. the resolver seam ──────────────────────────────────────────────────────

@test "S3: ntm and atm both on PATH -> resolver picks the public ntm" {
  _mk_stub "$TMP/bin" ntm
  _mk_stub "$TMP/bin" atm
  local saved="$PATH"
  PATH="$TMP/bin"
  run _pawl_swarm_bin
  PATH="$saved"
  [ "$status" -eq 0 ]
  [ "$output" = "ntm" ]
}

@test "S3: only atm on PATH -> resolver falls back to the operator alias" {
  _mk_stub "$TMP/bin" atm
  local saved="$PATH"
  PATH="$TMP/bin"
  run _pawl_swarm_bin
  PATH="$saved"
  [ "$status" -eq 0 ]
  [ "$output" = "atm" ]
}

@test "S3: neither on PATH -> resolver still names ntm (the public tool the error should name)" {
  local saved="$PATH"
  PATH="$TMP/empty"
  run _pawl_swarm_bin
  PATH="$saved"
  [ "$status" -eq 0 ]
  [ "$output" = "ntm" ]
}

@test "S3: PAWL_SWARM_BIN override wins over both" {
  _mk_stub "$TMP/bin" ntm
  local saved="$PATH"
  PATH="$TMP/bin"
  PAWL_SWARM_BIN="/opt/custom/ntm" run _pawl_swarm_bin
  PATH="$saved"
  [ "$status" -eq 0 ]
  [ "$output" = "/opt/custom/ntm" ]
}

# ── 2. no bare hardcoded atm invocation remains ───────────────────────────────

@test "S3: no bare hardcoded 'atm <verb>' invocation outside the resolver seam" {
  # Strip full-line comments, then hunt invocation-shaped atm calls by the verbs the
  # service actually uses. The ONLY allowed atm reference in code is the resolver's
  # fallback probe (command -v atm) and its printf fallback token.
  run bash -c "grep -vE '^[[:space:]]*#' '$SCRIPT' | grep -nE 'atm[[:space:]]+(spawn|kill|send|respawn|config|codex)' | grep -v 'command -v'"
  [ -z "$output" ]
}

@test "S3: exactly one resolver seam definition exists" {
  run grep -c '_pawl_swarm_bin()' "$SCRIPT"
  [ "$output" = "1" ]
}

# ── 3. every swarm invocation goes through \"\$SWARM\" ─────────────────────────

@test "S3: all swarm verbs are invoked via the resolved \$SWARM binary" {
  for verb in spawn kill respawn config "codex preflight" "codex wait-goal-engaged"; do
    run bash -c "grep -cE '\"\\\$SWARM\" $verb' '$SCRIPT'"
    [ "$output" -ge 1 ] || { echo "verb '$verb' not invoked via \"\$SWARM\""; return 1; }
  done
  # send is called from two seams (cod_send + _family_send)
  run bash -c "grep -cE '\"\\\$SWARM\" send' '$SCRIPT'"
  [ "$output" -ge 2 ]
}

@test "S3: embedded pawl.sh copy is byte-identical to the canonical script" {
  cmp "$SCRIPT" "$REPO_ROOT/cli/embedded/pawl/scripts/pawl.sh"
}
