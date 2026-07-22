#!/usr/bin/env bash
set -euo pipefail
PACK_DIR="${GC_PACK_DIR:?Gas City pack directory required}"
exec python3 "$PACK_DIR/assets/scripts/program_start.py" "$@"
