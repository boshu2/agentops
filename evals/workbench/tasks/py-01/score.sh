#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

# Activate venv
if [ -d ".venv" ]; then
  source .venv/bin/activate
else
  echo "ERROR: no .venv found in $WORKDIR" >&2
  echo '{"score": 0, "total": 3, "pass": false}'
  exit 0
fi

score=0
total=3

# Check 1: pytest passes (all tests including test_get_item_not_found)
if python -m pytest tests/ -q >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: GET /items/99999 returns 404
# Start a test server via TestClient
result=$(python3 -c "
from fastapi.testclient import TestClient
import tempfile, os, sys
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
    r = tc.get('/items/99999')
    print(r.status_code)
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "404" ]; then
  score=$((score + 1))
fi

# Check 3: GET /items/{valid_id} returns 200
result=$(python3 -c "
from fastapi.testclient import TestClient
import tempfile, os
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
    create = tc.post('/items', json={'name': 'Test', 'price': 5.0})
    item_id = create.json()['id']
    r = tc.get(f'/items/{item_id}')
    print(r.status_code)
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "200" ]; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
