#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
install-nightly-scheduler.sh

Generate and install the systemd user timer for the nightly evolution chain.

Installs a .service + .timer pair under ~/.config/systemd/user/ that calls
scripts/nightly-evolution.sh on a daily schedule. Safe by default: the service
runs in dry-run mode unless you edit the override or pass --execute-mode.

Options:
  --repo-root <path>       Repository root (default: git top-level or cwd)
  --schedule <calendar>    OnCalendar value (default: *-*-* 12:15:00 UTC)
  --execute-mode           Service runs with --execute --run-dream --run-evolve
  --dry-run-mode           Service runs in dry-run (default, preview only)
  --runners <csv>          Dream runners (default: claude,codex)
  --runtime-cmd <cmd>      Evolve runtime command (default: claude)
  --max-cycles <n>         Max evolve cycles (default: 1)
  --enable                 Enable and start the timer after install
  --uninstall              Stop, disable, and remove the timer+service
  --status                 Show timer status and exit
  --dry-run                Preview generated files without installing
  -h, --help               Show this help

Examples:
  scripts/install-nightly-scheduler.sh --dry-run
  scripts/install-nightly-scheduler.sh --enable
  scripts/install-nightly-scheduler.sh --execute-mode --enable
  scripts/install-nightly-scheduler.sh --uninstall
  scripts/install-nightly-scheduler.sh --status
EOF
}

die() { echo "install-nightly-scheduler: $*" >&2; exit 1; }

REPO_ROOT=""
SCHEDULE="*-*-* 12:15:00 UTC"
EXECUTE_MODE=false
RUNNERS="claude,codex"
RUNTIME_CMD="claude"
MAX_CYCLES="1"
ENABLE=false
UNINSTALL=false
STATUS=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT="${2:-}"; shift 2 ;;
    --schedule) SCHEDULE="${2:-}"; shift 2 ;;
    --execute-mode) EXECUTE_MODE=true; shift ;;
    --dry-run-mode) EXECUTE_MODE=false; shift ;;
    --runners) RUNNERS="${2:-}"; shift 2 ;;
    --runtime-cmd) RUNTIME_CMD="${2:-}"; shift 2 ;;
    --max-cycles) MAX_CYCLES="${2:-}"; shift 2 ;;
    --enable) ENABLE=true; shift ;;
    --uninstall) UNINSTALL=true; shift ;;
    --status) STATUS=true; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

if [[ -z "$REPO_ROOT" ]]; then
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi
REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"

UNIT_NAME="agentops-nightly-evolution"
SYSTEMD_DIR="$HOME/.config/systemd/user"
SERVICE_PATH="$SYSTEMD_DIR/${UNIT_NAME}.service"
TIMER_PATH="$SYSTEMD_DIR/${UNIT_NAME}.timer"

if [[ "$STATUS" == true ]]; then
  systemctl --user status "${UNIT_NAME}.timer" 2>&1 || true
  echo ""
  systemctl --user list-timers "${UNIT_NAME}.timer" 2>&1 || true
  exit 0
fi

if [[ "$UNINSTALL" == true ]]; then
  echo "Stopping and disabling ${UNIT_NAME}..."
  systemctl --user stop "${UNIT_NAME}.timer" 2>/dev/null || true
  systemctl --user disable "${UNIT_NAME}.timer" 2>/dev/null || true
  rm -f "$SERVICE_PATH" "$TIMER_PATH"
  systemctl --user daemon-reload
  echo "Removed: $SERVICE_PATH"
  echo "Removed: $TIMER_PATH"
  exit 0
fi

EXEC_START="$REPO_ROOT/scripts/nightly-evolution.sh"
if [[ "$EXECUTE_MODE" == true ]]; then
  EXEC_START="$EXEC_START --execute --run-dream --run-evolve --runners $RUNNERS --runtime-cmd $RUNTIME_CMD --max-cycles $MAX_CYCLES"
fi

KILL_SWITCH_CHECK="test ! -f $REPO_ROOT/.agents/evolve/STOP && test ! -f $REPO_ROOT/.agents/rpi/KILL && test ! -f \${HOME}/.config/evolve/KILL"

SERVICE_CONTENT="[Unit]
Description=AgentOps private local nightly evolution
Documentation=file://${REPO_ROOT}/docs/runbooks/nightly-evolution.md
ConditionPathExists=!${REPO_ROOT}/.agents/evolve/STOP
ConditionPathExists=!${REPO_ROOT}/.agents/rpi/KILL

[Service]
Type=oneshot
WorkingDirectory=${REPO_ROOT}
ExecStartPre=/bin/bash -c '${KILL_SWITCH_CHECK}'
ExecStart=${EXEC_START}
StandardOutput=journal
StandardError=journal
TimeoutStartSec=3600
Environment=PATH=/home/boful/.local/bin:/home/boful/bin:/usr/local/bin:/usr/bin:/bin
Environment=HOME=/home/boful
"

TIMER_CONTENT="[Unit]
Description=Schedule AgentOps private local nightly evolution

[Timer]
OnCalendar=${SCHEDULE}
Persistent=true
RandomizedDelaySec=10m

[Install]
WantedBy=timers.target
"

if [[ "$DRY_RUN" == true ]]; then
  echo "=== ${SERVICE_PATH} ==="
  echo "$SERVICE_CONTENT"
  echo ""
  echo "=== ${TIMER_PATH} ==="
  echo "$TIMER_CONTENT"
  exit 0
fi

mkdir -p "$SYSTEMD_DIR"
printf '%s' "$SERVICE_CONTENT" > "$SERVICE_PATH"
printf '%s' "$TIMER_CONTENT" > "$TIMER_PATH"
systemctl --user daemon-reload

echo "Installed: $SERVICE_PATH"
echo "Installed: $TIMER_PATH"

if [[ "$ENABLE" == true ]]; then
  systemctl --user enable --now "${UNIT_NAME}.timer"
  echo "Enabled and started: ${UNIT_NAME}.timer"
  systemctl --user list-timers "${UNIT_NAME}.timer"
fi
