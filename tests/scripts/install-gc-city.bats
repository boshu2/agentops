#!/usr/bin/env bats
# install-gc-city.bats — hermetic tests for scripts/install-gc-city.sh
# (age-gc-adoption-u0he.1). No real gc/bd/dolt/tmux: everything is stubbed on
# PATH in a temp dir; the membrane pack is a minimal fixture. The gc stub keeps
# session state in $STATE/sessions so start/new/list round-trip.

setup() {
  source "$(git rev-parse --show-toplevel)/lib/bats-common.bash"
  REPO_ROOT="$(bats_repo_root)"
  INSTALLER="$REPO_ROOT/scripts/install-gc-city.sh"

  # Physical path: the installer canonicalizes its city arg with `pwd -P`
  # (round-8 wrong-object class), so the test's expectations must live on the
  # same resolution — macOS mktemp returns /var/folders symlink-crossed paths.
  TMP="$(cd "$(mktemp -d)" && pwd -P)"
  STATE="$TMP/state"
  BIN="$TMP/bin"
  CITY="$TMP/city"
  mkdir -p "$STATE" "$BIN"
  : > "$STATE/sessions"

  # --- fake gc source tree: go.mod (beads pin) + the gc stub at bin/gc --------
  GC_SRC="$TMP/gcsrc"
  mkdir -p "$GC_SRC/bin"
  cat > "$GC_SRC/go.mod" <<'EOF'
module gcstub

require github.com/steveyegge/beads v1.1.0
EOF
  cat > "$GC_SRC/bin/gc" <<EOF
#!/usr/bin/env bash
STATE="$STATE"
cmd="\${1:-}"; shift || true
case "\$cmd" in
  version)
    echo '{"commit":"stubsha","version":"1.1.1","ok":true}' ;;
  init)
    city=""
    for a in "\$@"; do city="\$a"; done
    mkdir -p "\$city"
    printf '[pack]\nname = "stub-city"\n\n[imports.core]\nsource = "stub"\n' > "\$city/pack.toml"
    printf '# stub city.toml from gc init\n' > "\$city/city.toml" ;;
  import)
    exit 0 ;;
  start)
    if [ -z "\${STUB_NO_LANES:-}" ]; then
      echo "agentops-membrane.verifier" >> "\$STATE/sessions"
      echo "agentops-membrane.agy-verifier" >> "\$STATE/sessions"
    fi ;;
  status)
    printf '{"beads":{"beads_store":"%s","native_store_eligible":%s}}\n' \
      "\${STUB_STORE:-NativeDoltStore}" "\${STUB_ELIGIBLE:-true}" ;;
  session)
    sub="\${1:-}"; shift || true
    case "\$sub" in
      list)
        jq -Rn '{sessions: [inputs | select(length > 0) | {template: .}]}' \
          < "\$STATE/sessions" ;;
      new)
        [ -n "\${STUB_SESSION_NEW_FAIL:-}" ] && exit 1
        echo "\$1" >> "\$STATE/sessions" ;;
    esac ;;
  doctor)
    printf '{"results":[{"name":"agentops-membrane:law0-print-args","status":"%s"},{"name":"agentops-membrane:membrane-health","status":"%s"}]}\n' \
      "\${STUB_DOCTOR_LAW0:-ok}" "\${STUB_DOCTOR_MEMBRANE:-ok}" ;;
esac
exit 0
EOF
  chmod +x "$GC_SRC/bin/gc"

  # --- tool stubs on PATH ------------------------------------------------------
  bats_stub_bin "$BIN" bd '
case "${1:-}" in
  version) echo "bd version ${STUB_BD_VERSION:-1.1.0} (stub)" ;;
  context) printf "{\"dolt_mode\":\"%s\"}\n" "${STUB_DOLT_MODE:-server}" ;;
esac
exit 0'
  bats_stub_bin "$BIN" dolt 'echo "dolt version ${STUB_DOLT_VERSION:-2.1.10}"'
  bats_stub_bin "$BIN" tmux 'exit 0'
  export PATH="$BIN:$PATH"

  # --- minimal membrane pack fixture --------------------------------------------
  PACK="$TMP/pack"
  mkdir -p "$PACK/membrane" "$PACK/formulas" "$PACK/scripts" \
    "$PACK/doctor/law0-print-args" "$PACK/template-fragments" \
    "$PACK/agents/planner" "$PACK/agents/builder" \
    "$PACK/agents/verifier" "$PACK/agents/agy-verifier" \
    "$PACK/agents/breaker-helper"
  printf '[pack]\nname = "agentops-membrane"\nschema = 2\n' > "$PACK/pack.toml"
  printf '#!/usr/bin/env bash\necho close-gate-fixture\n' > "$PACK/membrane/close-gate.sh"
  printf '#!/usr/bin/env bash\necho finalize-fixture\n' > "$PACK/membrane/finalize.sh"
  printf '. as $x | $x\n' > "$PACK/membrane/finalize.jq"
  # deliberately NOT executable in the pack — the installer must set exec bits
  chmod -x "$PACK/membrane/close-gate.sh" "$PACK/membrane/finalize.sh"
  cat > "$PACK/formulas/membrane-quest.toml" <<'EOF'
formula = "membrane-quest"
[steps.check.check]
mode = "exec"
path = "membrane/close-gate.sh"
EOF
  cat > "$PACK/scripts/pretrust-codex-home.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
mkdir -p "$1/.gc/codex-home"
printf '{}\n' > "$1/.gc/codex-home/hooks.json"
echo "pretrust fixture seeded"
EOF
  chmod +x "$PACK/scripts/pretrust-codex-home.sh"
  cat > "$PACK/doctor/law0-print-args/run.sh" <<'EOF'
#!/usr/bin/env bash
echo "LAW 0 ok (fixture)"
exit "${STUB_LAW0_RC:-0}"
EOF
  chmod +x "$PACK/doctor/law0-print-args/run.sh"
  cat > "$PACK/template-fragments/usage-local.toml" <<'EOF'
# fixture fragment
[usage]
provider = "local"
EOF
  for a in planner builder verifier agy-verifier breaker-helper; do
    printf 'scope = "city"\n' > "$PACK/agents/$a/agent.toml"
  done
}

teardown() { rm -rf "$TMP"; }

install() {
  run bash "$INSTALLER" "$CITY" --name testcity \
    --gc-binary "$GC_SRC/bin/gc" --pack-source "$PACK" "$@"
}

# --- (1) version contract: fail hard, with remedy --------------------------------

@test "bd version mismatch against gc's beads pin fails hard with remedy" {
  export STUB_BD_VERSION=1.0.0
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"version contract violation"* ]]
  [[ "$output" == *"bd 1.0.0"* ]]
  [[ "$output" == *"v1.1.0"* ]]
  [[ "$output" == *"REMEDY:"* ]]
  # fail-hard means nothing was stood up
  [ ! -e "$CITY/city.toml" ]
}

@test "dolt below the floor fails hard with remedy" {
  export STUB_DOLT_VERSION=2.0.9
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"dolt 2.0.9"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

@test "unresolvable beads pin (no go.mod near gc) fails hard with remedy" {
  mkdir -p "$TMP/nogomod"
  cp "$GC_SRC/bin/gc" "$TMP/nogomod/gc"
  HOME="$TMP/fakehome" run bash "$INSTALLER" "$CITY" --name testcity \
    --gc-binary "$TMP/nogomod/gc" --pack-source "$PACK"
  [ "$status" -ne 0 ]
  [[ "$output" == *"cannot resolve the beads version"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

# --- (2) dolt_mode=server enforced (F1) -------------------------------------------

@test "dolt_mode != server after start fails hard with remedy" {
  export STUB_DOLT_MODE=file
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"NOT in server mode"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

@test "non-native beads store in gc status fails hard" {
  export STUB_STORE=BdCliStore
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"beads_store"* ]]
  [[ "$output" == *"NativeDoltStore"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

@test "GC_BEADS=file in the environment is refused" {
  export GC_BEADS=file
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"GC_BEADS=file"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

# --- (3) --dry-run writes nothing ---------------------------------------------------

@test "--dry-run prints the plan and writes nothing" {
  install --dry-run
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY RUN"* ]]
  [[ "$output" == *"supervisor port"* ]]
  [ ! -e "$CITY" ]
  [ ! -s "$STATE/sessions" ]
}

# --- (4) gap A: membrane materialization ---------------------------------------------

@test "happy path materializes membrane/ into the city with exec bits" {
  install
  [ "$status" -eq 0 ]
  [ -f "$CITY/membrane/close-gate.sh" ]
  [ -x "$CITY/membrane/close-gate.sh" ]
  [ -x "$CITY/membrane/finalize.sh" ]
  [ -f "$CITY/membrane/finalize.jq" ]
  [[ "$output" == *"resolves: membrane/close-gate.sh"* ]]
}

@test "happy path writes the full membrane city config" {
  install
  [ "$status" -eq 0 ]
  # LAW-0 print sinks dead in city.toml
  grep -q 'print_args = \[\]' "$CITY/city.toml"
  grep -q '\[providers.antigravity\]' "$CITY/city.toml"
  # usage fragment applied
  grep -q '^\[usage\]' "$CITY/city.toml"
  grep -q '^provider = "local"' "$CITY/city.toml"
  # lanes always-on
  grep -q 'template = "agentops-membrane.verifier"' "$CITY/city.toml"
  grep -q 'template = "agentops-membrane.agy-verifier"' "$CITY/city.toml"
  ! grep -q 'template = "agentops-membrane.breaker-helper"' "$CITY/city.toml"
  # pack imported by local path
  grep -q '\[imports.agentops-membrane\]' "$CITY/pack.toml"
  # isolation: env.sh + dedicated GC_HOME + explicit supervisor port
  grep -q "GC_HOME=\"$CITY/.gc-home\"" "$CITY/env.sh"
  grep -q '^port = ' "$CITY/.gc-home/supervisor.toml"
  # residual gap B: pre-trusted codex home
  [ -s "$CITY/.gc/codex-home/hooks.json" ]
  # sessions registered + verified, doctor gate green
  [[ "$output" == *"all required session templates registered"* ]]
  [[ "$output" == *"Done — membrane city is up"* ]]
}

# --- (5) idempotent re-run ------------------------------------------------------------

@test "re-run is idempotent (no duplicated config, still green)" {
  install
  [ "$status" -eq 0 ]
  install
  [ "$status" -eq 0 ]
  [[ "$output" == *"Done — membrane city is up"* ]]
  [ "$(grep -c '\[providers.claude\]' "$CITY/city.toml")" -eq 1 ]
  [ "$(grep -c '\[imports.agentops-membrane\]' "$CITY/pack.toml")" -eq 1 ]
  [ "$(grep -c '^\[usage\]' "$CITY/city.toml")" -eq 1 ]
}

@test "re-run preserves operator additions below the marker" {
  install
  [ "$status" -eq 0 ]
  echo '[mail]' >> "$CITY/city.toml"
  echo 'retention_ttl = "168h"' >> "$CITY/city.toml"
  install
  [ "$status" -eq 0 ]
  grep -q 'retention_ttl = "168h"' "$CITY/city.toml"
}

# --- (6) sessions: verify-not-just-list (F3) --------------------------------------------

@test "missing lane sessions after start fail hard, naming the missing templates" {
  export STUB_NO_LANES=1
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"sessions NOT registered"* ]]
  [[ "$output" == *"agentops-membrane.verifier"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

@test "--skip-sessions skips session creation and verification" {
  export STUB_NO_LANES=1
  install --skip-sessions
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIPPED (--skip-sessions)"* ]]
  [ ! -s "$STATE/sessions" ]
}

# --- (7) doctor gate -------------------------------------------------------------------

@test "red membrane-health doctor check fails the install" {
  export STUB_DOCTOR_MEMBRANE=error
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"membrane-health"* ]]
  [[ "$output" == *"REMEDY:"* ]]
}

@test "law0 pack check failure blocks before boot" {
  export STUB_LAW0_RC=2
  install
  [ "$status" -ne 0 ]
  [[ "$output" == *"LAW 0 check failed"* ]]
  # blocked BEFORE gc start: no sessions were ever registered
  [ ! -s "$STATE/sessions" ]
}
