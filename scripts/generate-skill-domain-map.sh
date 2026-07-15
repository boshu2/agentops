#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ "${1:-}" == "--check" ]]; then
  exec python3 "$root/scripts/generate-skill-mesh.py" --check
fi
exec python3 "$root/scripts/generate-skill-mesh.py"
