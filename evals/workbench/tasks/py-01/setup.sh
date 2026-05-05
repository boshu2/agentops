#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../python-api" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# --- Introduce bug: GET /items/{item_id} returns 200 with null instead of 404 ---
# Replace the 404 raise with a return of None (which FastAPI serializes as null/200)
cd "$WORKDIR"

python3 -c "
import pathlib
p = pathlib.Path('app/main.py')
src = p.read_text()
old = '''    row = get_item(conn, item_id)
    if row is None:
        raise HTTPException(status_code=404, detail=\"Item not found\")
    return row'''
new = '''    row = get_item(conn, item_id)
    return row'''
assert old in src, f'Could not find expected block in main.py'
p.write_text(src.replace(old, new))
"

# Also remove the HTTPException import usage check won't matter,
# but the test_get_item_not_found test should now fail (expects 404, gets 200/500)
