#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
case "${1:-}" in
  --check) exec python3 "$root/scripts/generate-skill-mesh.py" --check ;;
  --stdout) exec python3 "$root/scripts/generate-skill-mesh.py" --print catalog ;;
  --out) exec python3 "$root/scripts/generate-skill-mesh.py" --print catalog --out "${2:?missing output path}" ;;
  -h|--help) echo 'Usage: generate-skill-catalog.sh [--check|--stdout|--out PATH]'; exit 0 ;;
  '') exec python3 "$root/scripts/generate-skill-mesh.py" ;;
  *) echo "unknown argument: $1" >&2; exit 2 ;;
esac
