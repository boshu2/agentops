#!/usr/bin/env bash
set -euo pipefail
json="${1:?Usage: check-am-reservation-conflicts.sh <reservation-response.json>}"
python3 - "$json" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
conflicts = data.get("conflicts") or []
if conflicts:
    first = conflicts[0]
    path = first.get("path") or first.get("path_pattern") or first.get("pattern") or "<unknown>"
    print(f"reservation conflict: {path}")
    sys.exit(1)
sys.exit(0)
PY
