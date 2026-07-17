#!/usr/bin/env bash
set -euo pipefail
exec python3 "$GC_PACK_DIR/assets/scripts/packet.py" doctor-projection
