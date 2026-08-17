#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" || -L "$1" ]]; then
  echo "usage: $0 <learning-observations.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import datetime as dt
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 65536:
    raise SystemExit("learn output: artifact exceeds 65536 bytes")
try:
    value = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"learn output: unreadable JSON: {exc}")
if not isinstance(value, dict) or set(value) != {"schema_version", "created_at", "expires_at", "source_digests", "observations"}:
    raise SystemExit("learn output: unexpected or missing fields")
if value["schema_version"] != "learning-observations.v1":
    raise SystemExit("learn output: wrong schema version")
pattern = r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z"
if not isinstance(value["created_at"], str) or not re.fullmatch(pattern, value["created_at"]):
    raise SystemExit("learn output: invalid created_at")
if not isinstance(value["expires_at"], str) or not re.fullmatch(pattern, value["expires_at"]):
    raise SystemExit("learn output: invalid expires_at")
created = dt.datetime.fromisoformat(value["created_at"].replace("Z", "+00:00"))
expires = dt.datetime.fromisoformat(value["expires_at"].replace("Z", "+00:00"))
ttl = expires - created
if ttl < dt.timedelta(days=1) or ttl > dt.timedelta(days=30):
    raise SystemExit("learn output: TTL must be between 1 and 30 days")
digests = value["source_digests"]
if not isinstance(digests, list) or not 1 <= len(digests) <= 100 or len(set(digests)) != len(digests) or not all(isinstance(item, str) and re.fullmatch(r"[a-f0-9]{64}", item) for item in digests):
    raise SystemExit("learn output: invalid source digests")
observations = value["observations"]
if not isinstance(observations, list) or not 1 <= len(observations) <= 100 or not all(isinstance(item, str) and 0 < len(item) <= 4096 for item in observations):
    raise SystemExit("learn output: observations must be bounded")
print(f"valid learning-observations.v1: {path}")
PY
