#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../python-api" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

cd "$WORKDIR"

# --- Remove Pydantic validators from models.py ---
python3 -c "
import pathlib
p = pathlib.Path('app/models.py')
src = p.read_text()

# Remove the price validator
old_price = '''
    @field_validator(\"price\")
    @classmethod
    def price_must_be_positive(cls, v: float) -> float:
        if v <= 0:
            raise ValueError(\"price must be greater than 0\")
        return v'''
assert old_price in src, 'Could not find price validator in models.py'
src = src.replace(old_price, '')

# Remove the name validator
old_name = '''
    @field_validator(\"name\")
    @classmethod
    def name_must_be_nonempty(cls, v: str) -> str:
        if not v.strip():
            raise ValueError(\"name must not be empty\")
        return v'''
assert old_name in src, 'Could not find name validator in models.py'
src = src.replace(old_name, '')

# Remove unused import of field_validator
src = src.replace('from pydantic import BaseModel, field_validator', 'from pydantic import BaseModel')

p.write_text(src)
"

# --- Remove validation tests from test_main.py ---
python3 -c "
import pathlib
p = pathlib.Path('tests/test_main.py')
src = p.read_text()

old_price_test = '''

def test_create_item_invalid_price(client):
    resp = client.post(\"/items\", json={\"name\": \"Bad\", \"price\": -5.0})
    assert resp.status_code == 422'''
assert old_price_test in src, 'Could not find test_create_item_invalid_price'
src = src.replace(old_price_test, '')

old_name_test = '''

def test_create_item_empty_name(client):
    resp = client.post(\"/items\", json={\"name\": \"  \", \"price\": 1.0})
    assert resp.status_code == 422'''
assert old_name_test in src, 'Could not find test_create_item_empty_name'
src = src.replace(old_name_test, '')

p.write_text(src)
"
