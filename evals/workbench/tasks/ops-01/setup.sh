#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../devops" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Remove strict error handling from deploy.sh
sed -i 's/^set -euo pipefail$//' "$WORKDIR/scripts/deploy.sh"

# Remove the entire validation section that checks required fields.
# This removes lines from "# --- Validate required fields ---" through the closing "fi"
# of the missing-fields check, so the script silently continues with empty vars.
sed -i '/^# --- Validate required fields ---$/,/^fi$/d' "$WORKDIR/scripts/deploy.sh"
