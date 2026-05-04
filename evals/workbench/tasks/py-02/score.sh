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

# Check 2: POST with price=-1 returns 422
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
    r = tc.post('/items', json={'name': 'Bad', 'price': -1.0})
    print(r.status_code)
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "422" ]; then
  score=$((score + 1))
fi

# Check 3: POST with empty name returns 422
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
    r = tc.post('/items', json={'name': '  ', 'price': 1.0})
    print(r.status_code)
app.dependency_overrides.clear()
os.unlink(path)
" 2>/dev/null || echo "ERROR")
if [ "$result" = "422" ]; then
  score=$((score + 1))
fi

# Check 4: test file has validation test functions
if grep -q "def test_create_item_invalid_price" tests/test_main.py 2>/dev/null &&
   grep -q "def test_create_item_empty_name" tests/test_main.py 2>/dev/null; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
