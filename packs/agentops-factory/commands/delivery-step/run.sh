#!/usr/bin/env bash
set -euo pipefail
PACK_DIR="${GC_PACK_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
exec "$PACK_DIR/assets/scripts/delivery-step.sh"
