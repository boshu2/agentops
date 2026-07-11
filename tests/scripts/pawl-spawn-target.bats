#!/usr/bin/env bats
# age-l3xj (cross-family refuter round 13): `atm spawn <project>` roots panes at
# projects_base/<project> (no cwd flag). Deriving PROJECT=basename(ROOT) targets the WRONG or a
# MISSING directory whenever the repo is not a direct child of projects_base (e.g. a nested
# worktree at ~/dev/agentops-wt/age-l3xj resolves to ~/dev/age-l3xj). `up` must VERIFY the spawn
# target resolves back to THIS repo before spawning, and fail closed with an actionable message
# otherwise — never spawn into the wrong repo.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  # atm mock: `atm config get projects_base` echoes our temp projects_base; everything else no-ops.
  cat > "$TMP/bin/atm" <<EOF
#!/usr/bin/env bash
if [ "\$1 \$2 \$3" = "config get projects_base" ]; then printf '%s' "$TMP/pb"; exit 0; fi
exit 0
EOF
  chmod +x "$TMP/bin/atm"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/tmux"; chmod +x "$TMP/bin/tmux"
  export PATH="$TMP/bin:$PATH"
  export NTM_PROJECTS_BASE="$TMP/pb"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  log() { :; }
  mkdir -p "$TMP/pb"
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; unset NTM_PROJECTS_BASE PAWL_PROJECT; }

@test "_pawl_projects_base prefers the live atm config" {
  [ "$(_pawl_projects_base)" = "$TMP/pb" ]
}

@test "verify PASSES when the repo is a direct child of projects_base" {
  mkdir -p "$TMP/pb/myrepo"
  ROOT="$TMP/pb/myrepo"; PROJECT="myrepo"
  run _pawl_verify_spawn_target
  [ "$status" -eq 0 ]
}

@test "verify FAILS for a NESTED worktree (basename resolves to a different dir)" {
  mkdir -p "$TMP/pb/agentops-wt/age-l3xj"      # the real repo is nested…
  # …but projects_base/age-l3xj does NOT exist (or is a different dir).
  ROOT="$TMP/pb/agentops-wt/age-l3xj"; PROJECT="age-l3xj"
  run _pawl_verify_spawn_target
  [ "$status" -ne 0 ]
}

@test "verify FAILS when projects_base/<name> is a DIFFERENT directory than ROOT" {
  mkdir -p "$TMP/pb/age-l3xj"                   # a decoy dir with the same basename
  mkdir -p "$TMP/elsewhere/age-l3xj"
  ROOT="$TMP/elsewhere/age-l3xj"; PROJECT="age-l3xj"
  run _pawl_verify_spawn_target
  [ "$status" -ne 0 ]                            # must not target the decoy under projects_base
}

@test "an explicit PAWL_PROJECT is trusted (operator named the project deliberately)" {
  export PAWL_PROJECT=whatever
  ROOT="$TMP/anywhere"; PROJECT="whatever"
  run _pawl_verify_spawn_target
  [ "$status" -eq 0 ]
}

@test "cmd_up fails closed BEFORE spawning when the target does not resolve to this repo" {
  mkdir -p "$TMP/pb/agentops-wt/age-l3xj"
  ROOT="$TMP/pb/agentops-wt/age-l3xj"; PROJECT="age-l3xj"; SESSION="age-l3xj--pawl-service"
  # No session exists; probe returns at least one family so cmd_up reaches the spawn guard.
  session_exists() { return 1; }
  probe_families() { echo cc; }
  resolve_default_families() { echo cc; }
  _set_panes_from_enabled() { :; }
  run cmd_up
  [ "$status" -ne 0 ]
  [[ "$output" == *"not a direct child of the NTM projects_base"* ]]
}
