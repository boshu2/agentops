#!/usr/bin/env bats
# Acceptance for scripts/check-no-daemonized-wake.sh (ag-cjruu).
# The guard must FAIL when the wake bridge is wired into any always-on launcher
# (systemd/launchd/cron/loop) and PASS for clean / on-demand / self references.

setup() {
  GUARD="${BATS_TEST_DIRNAME}/../../scripts/check-no-daemonized-wake.sh"
  ROOT="$(mktemp -d)"          # non-git fixture dir -> guard uses grep fallback
}
teardown() { rm -rf "$ROOT"; }

@test "PASS: no reference to the bridge" {
  echo "nothing here" > "$ROOT/readme.md"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 0 ]
}

@test "PASS: on-demand mention (doc / plain call, no daemon)" {
  printf '%s\n' "Run scripts/ntm-attention-tend.sh mysession once when you need it." > "$ROOT/docs.md"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 0 ]
}

@test "FAIL: systemd .service references the bridge" {
  printf '%s\n' "[Service]" "ExecStart=/repo/scripts/ntm-attention-tend.sh sess" > "$ROOT/wake.service"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"VIOLATION"* ]]
}

@test "FAIL: systemd .timer references the bridge" {
  printf '%s\n' "[Timer]" "OnCalendar=*:0/5" "Unit=wake.service ntm-attention-tend" > "$ROOT/wake.timer"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: launchd .plist references the bridge" {
  printf '%s\n' "<string>/repo/scripts/ntm-attention-tend.sh</string>" > "$ROOT/com.wake.plist"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: cron schedule invokes the bridge" {
  printf '%s\n' "*/5 * * * * /repo/scripts/ntm-attention-tend.sh sess" > "$ROOT/crontab"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: shell loop wraps the bridge" {
  printf '%s\n' "#!/usr/bin/env bash" "while true; do" "  ntm-attention-tend.sh sess" "  sleep 30" "done" > "$ROOT/loop.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: cron @reboot macro invokes the bridge" {
  printf '%s\n' "@reboot /repo/scripts/ntm-attention-tend.sh sess" > "$ROOT/crontab"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: for ((;;)) infinite loop wraps the bridge" {
  printf '%s\n' "#!/usr/bin/env bash" "for ((;;)); do ntm-attention-tend.sh sess; sleep 30; done" > "$ROOT/loop2.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: watch -n drives the bridge" {
  printf '%s\n' "#!/usr/bin/env bash" "watch -n 30 ntm-attention-tend.sh sess" > "$ROOT/w.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: nohup backgrounds the bridge" {
  printf '%s\n' "#!/usr/bin/env bash" "nohup ntm-attention-tend.sh sess &" > "$ROOT/n.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: pm2 process manager runs the bridge" {
  printf '%s\n' "module.exports = { apps: [{ script: 'scripts/ntm-attention-tend.sh' }] }" "// pm2 ecosystem" > "$ROOT/ecosystem.config.js"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: supervisor program runs the bridge" {
  printf '%s\n' "[program:wake]" "command=scripts/ntm-attention-tend.sh sess" "; supervisord" > "$ROOT/supervisor.conf"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: generated unit (.service.in) references the bridge" {
  printf '%s\n' "[Service]" "ExecStart=@PREFIX@/scripts/ntm-attention-tend.sh sess" > "$ROOT/wake.service.in"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "FAIL: systemd drop-in (.service.d/override.conf) references the bridge" {
  mkdir -p "$ROOT/wake.service.d"
  printf '%s\n' "[Service]" "ExecStart=scripts/ntm-attention-tend.sh sess" > "$ROOT/wake.service.d/override.conf"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 1 ]
}

@test "PASS: finite for-loop over sessions is on-demand, not a daemon" {
  printf '%s\n' "#!/usr/bin/env bash" "for s in a b c; do ntm-attention-tend.sh \"\$s\"; done" > "$ROOT/finite.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 0 ]
}

# This is the WIRING: CI runs this bats file (validate.yml), so this case runs the
# guard against the REAL repo root every build. If anyone later daemonizes the
# bridge in-repo, THIS test fails in CI — the guard has teeth, not just fixtures.
@test "guard runs against the repo root and passes (CI enforcement)" {
  repo_root="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
  run bash "$GUARD" "$repo_root"
  [ "$status" -eq 0 ]
}

@test "PASS: the bridge file itself is not flagged" {
  mkdir -p "$ROOT/scripts"
  printf '%s\n' "#!/usr/bin/env bash" "# ntm-attention-tend.sh self" "while kill -0 x; do :; done" > "$ROOT/scripts/ntm-attention-tend.sh"
  run bash "$GUARD" "$ROOT"
  [ "$status" -eq 0 ]
}
