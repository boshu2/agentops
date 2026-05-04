#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

if [ -d ".venv" ]; then
  source .venv/bin/activate
else
  echo "ERROR: no .venv found in $WORKDIR" >&2
  echo '{"score": 0, "total": 4, "pass": false}'
  exit 0
fi

score=0
total=4

# Check 1: pytest passes
if python -m pytest tests/ -q >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: GET /items?page=1&size=2 returns max 2 items
result=$(python3 -c "
from fastapi.testclient import TestClient
import tempfile, os, json
from app.db import get_db, init_db, insert_item
from app.main import app, _get_conn

fd, path = tempfile.mkstemp(suffix='.db')
os.close(fd)
conn = get_db(path)
init_db(conn)
conn.close()

def _override():
    c = get_db(path)
    try:
        yield c
    finally:
        c.close()

app.dependency_overrides[_get_conn] = _override
with TestClient(app, raise_server_exceptions=False) as tc:
    for i in range(5):
        tc.post('/items', json={'name': f'Item {i}', 'price': float(i + 1)})
    r = tc.get('/items?page=1&size=2')
    data = r.json()
    items = data.get('items', [])
    print(len(items))
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "2" ]; then
  score=$((score + 1))
fi

# Check 3: response has total/page/size fields
result=$(python3 -c "
from fastapi.testclient import TestClient
import tempfile, os, json
from app.db import get_db, init_db
from app.main import app, _get_conn

fd, path = tempfile.mkstemp(suffix='.db')
os.close(fd)
conn = get_db(path)
init_db(conn)
conn.close()

def _override():
    c = get_db(path)
    try:
        yield c
    finally:
        c.close()

app.dependency_overrides[_get_conn] = _override
with TestClient(app, raise_server_exceptions=False) as tc:
    r = tc.get('/items?page=1&size=2')
    data = r.json()
    has_all = all(k in data for k in ('total', 'page', 'size'))
    print('yes' if has_all else 'no')
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "yes" ]; then
  score=$((score + 1))
fi

# Check 4: pagination test exists in test file
if grep -q "def test_list_items_pagination" tests/test_main.py 2>/dev/null; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
