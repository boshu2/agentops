#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/nightly-evolution.sh"

    TMP_DIR="$(mktemp -d)"
    MOCK_BIN="$TMP_DIR/bin"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$MOCK_BIN" "$FAKE_REPO/scripts" "$FAKE_REPO/.agents"

    cp "$SCRIPT" "$FAKE_REPO/scripts/nightly-evolution.sh"
    chmod +x "$FAKE_REPO/scripts/nightly-evolution.sh"

    cat >"$FAKE_REPO/scripts/nightly-rpi-brief.sh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "$out"
printf '{"ok":true}\n' > "$out/summary.json"
STUB
    chmod +x "$FAKE_REPO/scripts/nightly-rpi-brief.sh"

    cat >"$FAKE_REPO/scripts/ao-rpi-autonomous-cycle.sh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${AO_RPI_LOG:?}"
STUB
    chmod +x "$FAKE_REPO/scripts/ao-rpi-autonomous-cycle.sh"

    git -C "$FAKE_REPO" init -q
    git -C "$FAKE_REPO" config user.email test@example.com
    git -C "$FAKE_REPO" config user.name Test
    touch "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m init

    make_common_stubs
}

teardown() {
    rm -rf "$TMP_DIR"
}

make_common_stubs() {
    cat >"$MOCK_BIN/ao" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${AO_LOG:?}"
if [[ "$1 $2" == "overnight setup" ]]; then
  printf '{"status":"dry-run","dream":{"runners":["claude","codex"],"scheduler_mode":"systemd"}}\n'
  exit 0
fi
if [[ "$1 $2 $3" == "daemon jobs submit" ]]; then
  if [[ "${AO_DAEMON_SUBMIT_FAIL:-}" == "1" ]]; then
    printf 'daemon submit failed\n' >&2
    exit 55
  fi
  printf '{"job_id":"job-dream","status":"queued"}\n'
  exit 0
fi
if [[ "$1 $2" == "overnight start" ]]; then
  printf '{"status":"ok"}\n'
  exit 0
fi
printf '{}\n'
STUB
    chmod +x "$MOCK_BIN/ao"

    cat >"$MOCK_BIN/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf '[]\n'
STUB
    chmod +x "$MOCK_BIN/gh"

    cat >"$MOCK_BIN/bd" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
    chmod +x "$MOCK_BIN/bd"

    cat >"$MOCK_BIN/bushido-box" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf '{"command":"ai-sane","status":"ok","result":{"summary":{"ok":5,"warn":0,"fail":0}}}\n'
STUB
    chmod +x "$MOCK_BIN/bushido-box"

    cat >"$MOCK_BIN/claude" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
    chmod +x "$MOCK_BIN/claude"

    cat >"$MOCK_BIN/codex" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
    chmod +x "$MOCK_BIN/codex"
}

stub_bushido_status() {
    local status="$1"
    cat >"$MOCK_BIN/bushido-box" <<STUB
#!/usr/bin/env bash
set -euo pipefail
printf '{"command":"ai-sane","status":"${status}","result":{"summary":{"ok":0,"warn":1,"fail":0}}}\n'
STUB
    chmod +x "$MOCK_BIN/bushido-box"
}

stub_nightly_brief_failure() {
    cat >"$FAKE_REPO/scripts/nightly-rpi-brief.sh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
exit 42
STUB
    chmod +x "$FAKE_REPO/scripts/nightly-rpi-brief.sh"
}

add_remote_nightly_branches() {
    git init --bare -q "$TMP_DIR/origin.git"
    git -C "$FAKE_REPO" remote add origin "$TMP_DIR/origin.git"
    git -C "$FAKE_REPO" push -q origin HEAD:refs/heads/nightly/2026-05-01
    git -C "$FAKE_REPO" push -q origin HEAD:refs/heads/nightly/2026-05-01-v2
}

@test "public validation scenario fixtures are schema-shaped" {
    for scenario in "$REPO_ROOT"/tests/scenarios/nightly-evolution/*.json; do
        jq -e '
          .version == 1 and
          (.id | test("^auto-.+")) and
          (.date | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}$")) and
          .goal and .narrative and .expected_outcome and
          (.satisfaction_threshold >= 0 and .satisfaction_threshold <= 1) and
          (.source == "agent") and
          (.status == "active") and
          (.acceptance_vectors | type == "array" and length > 0) and
          (.scope.files | index("scripts/nightly-evolution.sh"))
        ' "$scenario"
    done
}

@test "dry-run writes digest without running Dream or evolve" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief

    [ "$status" -eq 0 ]
    [ -f "$TMP_DIR/out/digest.json" ]
    [ -f "$TMP_DIR/out/digest.md" ]
    jq -e '.mode == "dry-run" and .phases.dream == "not-requested" and .phases.evolve == "not-requested"' "$TMP_DIR/out/digest.json"
    ! grep -q 'overnight start' "$AO_LOG"
    [ ! -s "$AO_RPI_LOG" ]
}

@test "dry-run with requested phases records planned without executing" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --run-dream \
        --run-evolve

    [ "$status" -eq 0 ]
    jq -e '.mode == "dry-run" and .phases.dream == "planned" and .phases.evolve == "planned"' "$TMP_DIR/out/digest.json"
    ! grep -q 'overnight start' "$AO_LOG"
    [ ! -s "$AO_RPI_LOG" ]
}

@test "emit-systemd writes service and timer templates" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --emit-systemd \
        --schedule "Mon..Fri 12:15:00 UTC"

    [ "$status" -eq 0 ]
    [ -f "$TMP_DIR/out/systemd/agentops-nightly-evolution.service" ]
    [ -f "$TMP_DIR/out/systemd/agentops-nightly-evolution.timer" ]
    grep -q 'ExecStart=.*nightly-evolution.sh --execute --run-dream --run-evolve' "$TMP_DIR/out/systemd/agentops-nightly-evolution.service"
    grep -q 'OnCalendar=Mon..Fri 12:15:00 UTC' "$TMP_DIR/out/systemd/agentops-nightly-evolution.timer"
    jq -e '.artifacts.systemd_dir != null' "$TMP_DIR/out/digest.json"
}

@test "execute blocks when ai-sane is not ok" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    stub_bushido_status "warn"

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-dream

    [ "$status" -ne 0 ]
    [[ "$output" == *"bushido-box ai-sane is required for execute mode, got: warn"* ]]
    [ ! -f "$AO_LOG" ] || ! grep -q 'overnight start' "$AO_LOG"
    [ ! -s "$AO_RPI_LOG" ]
}

@test "execute can override ai-sane gate explicitly" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    stub_bushido_status "warn"

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-dream \
        --no-require-ai-sane

    [ "$status" -eq 0 ]
    grep -q 'daemon jobs submit' "$AO_LOG"
    ! grep -q 'overnight start' "$AO_LOG"
    jq -e '.readiness.ai_sane_status == "warn" and .phases.dream == "submitted"' "$TMP_DIR/out/digest.json"
}

@test "execute run-dream submits daemon dream job payload" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-dream

    [ "$status" -eq 0 ]
    grep -q 'daemon jobs submit' "$AO_LOG"
    ! grep -q 'overnight start' "$AO_LOG"
    jq -e '.mode == "execute" and .phases.dream == "submitted"' "$TMP_DIR/out/digest.json"
    payload="$(jq -r '.artifacts.dream_payload' "$TMP_DIR/out/digest.json")"
    jq -e '.job_type == "dream.run" and .mode == "daemon" and .max_iterations == 1 and (.output_dir | endswith("/dream"))' "$payload"
}

@test "execute run-dream falls back to legacy subprocess when daemon submit fails" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG AO_DAEMON_SUBMIT_FAIL=1

    run env PATH="$MOCK_BIN:$PATH" AO_DAEMON_SUBMIT_FAIL="$AO_DAEMON_SUBMIT_FAIL" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-dream

    [ "$status" -eq 0 ]
    grep -q 'daemon jobs submit' "$AO_LOG"
    grep -q 'overnight start' "$AO_LOG"
    grep -q -- '--runner claude' "$AO_LOG"
    grep -q -- '--runner codex' "$AO_LOG"
    jq -e '.mode == "execute" and .phases.dream == "ok"' "$TMP_DIR/out/digest.json"
}

@test "execute run-dream can skip legacy subprocess fallback" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG AO_DAEMON_SUBMIT_FAIL=1

    run env PATH="$MOCK_BIN:$PATH" AO_DAEMON_SUBMIT_FAIL="$AO_DAEMON_SUBMIT_FAIL" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-dream \
        --skip-dream-subprocess

    [ "$status" -ne 0 ]
    [[ "$output" == *"Dream daemon submit failed"* ]]
    grep -q 'daemon jobs submit' "$AO_LOG"
    ! grep -q 'overnight start' "$AO_LOG"
}

@test "kill switch prevents nightly run" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    mkdir -p "$FAKE_REPO/.agents/evolve"
    touch "$FAKE_REPO/.agents/evolve/STOP"

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief

    [ "$status" -ne 0 ]
    [[ "$output" == *"nightly kill switch is present"* ]]
    [ ! -f "$AO_LOG" ]
    [ ! -s "$AO_RPI_LOG" ]
}

@test "active lock blocks overlapping runs" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    mkdir -p "$FAKE_REPO/.agents/nightly/nightly-evolution.lock"

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief

    [ "$status" -ne 0 ]
    [[ "$output" == *"another nightly evolution run appears active"* ]]
    [ ! -f "$AO_LOG" ]
    [ ! -s "$AO_RPI_LOG" ]
}

@test "nightly brief failure is advisory and recorded" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    stub_nightly_brief_failure

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01

    [ "$status" -eq 0 ]
    jq -e '.phases.nightly_brief == "failed" and .phases.dream == "not-requested"' "$TMP_DIR/out/digest.json"
}

@test "branch suffix skips existing remote nightly heads" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG
    add_remote_nightly_branches

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief

    [ "$status" -eq 0 ]
    jq -e '.planned_branch == "nightly/2026-05-01-v3"' "$TMP_DIR/out/digest.json"
}

@test "execute run-evolve passes runtime env and branch to supervisor wrapper" {
    AO_LOG="$TMP_DIR/ao.log"
    AO_RPI_LOG="$TMP_DIR/rpi.log"
    export AO_LOG AO_RPI_LOG

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/nightly-evolution.sh" \
        --repo-root "$FAKE_REPO" \
        --output-dir "$TMP_DIR/out" \
        --date 2026-05-01 \
        --skip-brief \
        --execute \
        --run-evolve \
        --runtime-cmd codex \
        --runtime-mode direct \
        --landing-policy off

    [ "$status" -eq 0 ]
    grep -q -- '--landing-branch nightly/2026-05-01' "$AO_RPI_LOG"
    jq -e '.runtime.command == "codex" and .runtime.mode == "direct" and .phases.evolve == "ok"' "$TMP_DIR/out/digest.json"
}
