#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../python-api" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

cd "$WORKDIR"

# --- Introduce SQL injection: change get_item to use f-string interpolation ---
python3 -c "
import pathlib
p = pathlib.Path('app/db.py')
src = p.read_text()

old = '''def get_item(conn: sqlite3.Connection, item_id: int) -> dict | None:
    row = conn.execute(\"SELECT * FROM items WHERE id = ?\", (item_id,)).fetchone()
    if row is None:
        return None
    return dict(row)'''

new = '''def get_item(conn: sqlite3.Connection, item_id: int) -> dict | None:
    row = conn.execute(f\"SELECT * FROM items WHERE id = {item_id}\").fetchone()
    if row is None:
        return None
    return dict(row)'''

assert old in src, 'Could not find parameterized get_item in db.py'
p.write_text(src.replace(old, new))
"
