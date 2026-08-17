#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 --root <.agents/scratch/learn> --authorization-id <id> [--now YYYY-MM-DDTHH:MM:SSZ] [--apply]" >&2
  exit 2
}

root=""
authorization_id=""
now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
apply=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --root) [[ $# -ge 2 ]] || usage; root="$2"; shift 2 ;;
    --authorization-id) [[ $# -ge 2 ]] || usage; authorization_id="$2"; shift 2 ;;
    --now) [[ $# -ge 2 ]] || usage; now="$2"; shift 2 ;;
    --apply) apply=true; shift ;;
    *) usage ;;
  esac
done

[[ -n "$authorization_id" && ${#authorization_id} -le 256 ]] || { echo "learn prune: authorization ID is required" >&2; exit 2; }
[[ "$now" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || { echo "learn prune: --now must be canonical UTC" >&2; exit 2; }
[[ -n "$root" && -d "$root" && ! -L "$root" ]] || { echo "learn prune: root must be a real directory" >&2; exit 2; }
physical_root="$(cd "$root" && pwd -P)"
case "$physical_root" in
  */.agents/scratch/learn) ;;
  *) echo "learn prune: root must end in .agents/scratch/learn" >&2; exit 2 ;;
esac

python3 - "$physical_root" "$authorization_id" "$now" "$apply" <<'PY'
import datetime as dt
import json
import os
from pathlib import Path
import re
import stat
import sys
import time

root = Path(sys.argv[1])
authorization_id = sys.argv[2]
now_text = sys.argv[3]
apply = sys.argv[4] == "true"
now = dt.datetime.fromisoformat(now_text.replace("Z", "+00:00"))
deadline = time.monotonic() + 10
seen = expired = live = unknown = 0
timestamp = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z")

with os.scandir(root) as entries:
    for entry in entries:
        seen += 1
        if seen > 1000:
            raise SystemExit("learn prune: 1000-entry ceiling exceeded")
        if time.monotonic() > deadline:
            raise SystemExit("learn prune: 10-second deadline exceeded")
        if not entry.name.endswith(".json") or entry.is_symlink() or not entry.is_file(follow_symlinks=False):
            continue
        candidate = Path(entry.path)
        before = entry.stat(follow_symlinks=False)
        if before.st_size > 65536:
            unknown += 1
            print(f"retain unknown: {candidate}")
            continue
        try:
            value = json.loads(candidate.read_bytes())
        except (OSError, json.JSONDecodeError):
            unknown += 1
            print(f"retain unknown: {candidate}")
            continue
        expires_at = value.get("expires_at") if isinstance(value, dict) and value.get("schema_version") == "learning-observations.v1" else None
        if not isinstance(expires_at, str):
            unknown += 1
            print(f"retain unknown: {candidate}")
            continue
        if not timestamp.fullmatch(expires_at):
            raise SystemExit(f"learn prune: malformed expiry in {candidate}")
        expires = dt.datetime.fromisoformat(expires_at.replace("Z", "+00:00"))
        if expires <= now:
            if apply:
                current = candidate.lstat()
                same_entry = (
                    stat.S_ISREG(current.st_mode)
                    and (current.st_dev, current.st_ino, current.st_mode, current.st_size)
                    == (before.st_dev, before.st_ino, before.st_mode, before.st_size)
                )
                if not same_entry:
                    raise SystemExit(f"learn prune: candidate changed before delete: {candidate}")
                candidate.unlink()
                if os.path.lexists(candidate):
                    raise SystemExit(f"learn prune: deletion verification failed: {candidate}")
                print(f"deleted expired: {candidate}")
            else:
                print(f"would delete expired: {candidate}")
            expired += 1
        else:
            live += 1
            print(f"retain live: {candidate}")

print(
    f"learn prune: authorization={authorization_id} apply={str(apply).lower()} "
    f"seen={seen} expired={expired} live={live} unknown={unknown}"
)
PY
