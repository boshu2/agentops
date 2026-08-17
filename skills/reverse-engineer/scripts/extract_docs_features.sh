#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: extract_docs_features.sh <paths.txt> <docs_features_prefix>" >&2
  exit 2
fi

PATHS_TXT="$1"
PREFIX_RAW="$2"

# Normalize prefix: "docs/features/" -> "/docs/features"
PREFIX="/${PREFIX_RAW#/}"
PREFIX="${PREFIX%/}"

python3 - "$PATHS_TXT" "$PREFIX" <<'PY'
import sys
from pathlib import Path

paths_txt = Path(sys.argv[1])
prefix = sys.argv[2]
if paths_txt.is_symlink() or not paths_txt.is_file() or paths_txt.stat().st_size > 16 * 1024 * 1024:
    raise SystemExit("paths input must be a regular non-symlink file no larger than 16777216 bytes")

out = set()
for line in paths_txt.read_text(encoding="utf-8", errors="replace").splitlines():
    p = line.strip()
    if not p:
        continue
    if not p.startswith("/"):
        p = "/" + p
    if p.startswith(prefix + "/") or p == prefix:
        # Keep the path *under* docs/features as a slug, without leading slash.
        slug = p.lstrip("/")
        out.add(slug)
        if len(out) > 100000:
            raise SystemExit("docs feature count exceeds 100000")

for s in sorted(out):
    print(s)
PY
