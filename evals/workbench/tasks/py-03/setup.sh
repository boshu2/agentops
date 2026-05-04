#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../python-api" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

cd "$WORKDIR"

# --- Remove pagination from main.py: return all items, no page/size params ---
python3 -c "
import pathlib

# Fix main.py: remove page/size params, return flat list
p = pathlib.Path('app/main.py')
src = p.read_text()

old = '''@app.get(\"/items\", response_model=ItemList)
def read_items(page: int = 1, size: int = 20, conn=Depends(_get_conn)):
    items, total = list_items(conn, page=page, size=size)
    return ItemList(items=items, total=total, page=page, size=size)'''

new = '''@app.get(\"/items\")
def read_items(conn=Depends(_get_conn)):
    items = list_items(conn)
    return {\"items\": items}'''

assert old in src, 'Could not find paginated read_items in main.py'
src = src.replace(old, new)

# Remove ItemList from import since we no longer use it
src = src.replace(
    'from app.models import ItemCreate, ItemList, ItemResponse',
    'from app.models import ItemCreate, ItemResponse'
)

p.write_text(src)
"

# --- Remove pagination from db.py: return all items ---
python3 -c "
import pathlib
p = pathlib.Path('app/db.py')
src = p.read_text()

old = '''def list_items(conn: sqlite3.Connection, page: int = 1, size: int = 20) -> tuple[list[dict], int]:
    total = conn.execute(\"SELECT COUNT(*) FROM items\").fetchone()[0]
    offset = (page - 1) * size
    rows = conn.execute(
        \"SELECT * FROM items ORDER BY id LIMIT ? OFFSET ?\", (size, offset)
    ).fetchall()
    return [dict(r) for r in rows], total'''

new = '''def list_items(conn: sqlite3.Connection) -> list[dict]:
    rows = conn.execute(\"SELECT * FROM items ORDER BY id\").fetchall()
    return [dict(r) for r in rows]'''

assert old in src, 'Could not find paginated list_items in db.py'
p.write_text(src.replace(old, new))
"

# --- Remove pagination test and fix list_items_empty test ---
python3 -c "
import pathlib
p = pathlib.Path('tests/test_main.py')
src = p.read_text()

# Remove the pagination test
old_pag = '''

def test_list_items_pagination(client):
    for i in range(5):
        client.post(\"/items\", json={\"name\": f\"Item {i}\", \"price\": float(i + 1)})
    resp = client.get(\"/items?page=1&size=2\")
    assert resp.status_code == 200
    data = resp.json()
    assert len(data[\"items\"]) == 2
    assert data[\"total\"] == 5
    assert data[\"page\"] == 1
    assert data[\"size\"] == 2
    assert data[\"items\"][0][\"name\"] == \"Item 0\"
    assert data[\"items\"][1][\"name\"] == \"Item 1\"'''
assert old_pag in src, 'Could not find test_list_items_pagination'
src = src.replace(old_pag, '')

# Fix the empty list test to match unpaginated response
old_empty = '''def test_list_items_empty(client):
    resp = client.get(\"/items\")
    assert resp.status_code == 200
    data = resp.json()
    assert data[\"items\"] == []
    assert data[\"total\"] == 0
    assert data[\"page\"] == 1
    assert data[\"size\"] == 20'''

new_empty = '''def test_list_items_empty(client):
    resp = client.get(\"/items\")
    assert resp.status_code == 200
    data = resp.json()
    assert data[\"items\"] == []'''

assert old_empty in src, 'Could not find test_list_items_empty'
src = src.replace(old_empty, new_empty)

p.write_text(src)
"
