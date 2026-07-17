#!/usr/bin/env bash
set -euo pipefail

PACK_DIR="${GC_PACK_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
exec python3 "$PACK_DIR/assets/scripts/packet.py" run "$@"
