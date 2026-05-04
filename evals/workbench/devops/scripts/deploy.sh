#!/usr/bin/env bash
set -euo pipefail

# Simulated deployment script
# Reads config from deploy.yaml (no yq dependency), validates, and runs deploy steps.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="${DEPLOY_CONFIG:-${SCRIPT_DIR}/../config/deploy.yaml}"

# --- Config parser (grep/awk, no yq) ---

yaml_get() {
    local key="$1" file="$2"
    { grep "^${key}:" "$file" 2>/dev/null || true; } | awk -F': ' '{print $2}' | sed 's/^"//;s/"$//' | tr -d '\r'
}

if [[ ! -f "$CONFIG" ]]; then
    echo "ERROR: config file not found: $CONFIG" >&2
    exit 1
fi

APP_NAME="$(yaml_get app_name "$CONFIG")"
VERSION="$(yaml_get version "$CONFIG")"
TARGET="$(yaml_get target "$CONFIG")"
HEALTH_URL="$(yaml_get health_url "$CONFIG")"
LOG_DIR="$(yaml_get log_dir "$CONFIG")"

# --- Validate required fields ---

missing=()
[[ -z "${APP_NAME:-}" ]] && missing+=(app_name)
[[ -z "${VERSION:-}" ]] && missing+=(version)
[[ -z "${TARGET:-}" ]] && missing+=(target)

if [[ ${#missing[@]} -gt 0 ]]; then
    echo "ERROR: missing required config fields: ${missing[*]}" >&2
    exit 1
fi

LOG_DIR="${LOG_DIR:-/tmp/wb-deploy-logs}"
mkdir -p "$LOG_DIR"
LOGFILE="${LOG_DIR}/deploy-$(date +%Y%m%d-%H%M%S).log"

log() { echo "[$(date +%H:%M:%S)] $*" | tee -a "$LOGFILE"; }

# --- Deploy pipeline ---

log "=== Deploying ${APP_NAME} v${VERSION} to ${TARGET} ==="

log "Step 1/4: Validate configuration"
if [[ "$TARGET" != "staging" && "$TARGET" != "production" && "$TARGET" != "development" ]]; then
    log "ERROR: invalid target '${TARGET}' (expected staging|production|development)"
    exit 1
fi
log "  Config valid."

log "Step 2/4: Build"
sleep 0.1  # simulate build time
log "  Build complete."

log "Step 3/4: Deploy to ${TARGET}"
if [[ "${SIMULATE_DEPLOY_FAILURE:-}" == "1" ]]; then
    log "ERROR: deployment to ${TARGET} failed (simulated)"
    exit 1
fi
sleep 0.1  # simulate deploy time
log "  Deploy complete."

log "Step 4/4: Verify"
if [[ -n "${HEALTH_URL:-}" ]] && command -v curl &>/dev/null; then
    log "  Health endpoint: ${HEALTH_URL} (skipped in simulation)"
fi
log "  Verification complete."

log "=== Deployment successful ==="
echo "$LOGFILE"
