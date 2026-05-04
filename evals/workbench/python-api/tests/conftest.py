import tempfile
import os

import pytest
from fastapi.testclient import TestClient

from app.db import get_db, init_db
from app.main import app, _get_conn

_db_path: str = ""


def _override_get_conn():
    conn = get_db(_db_path)
    try:
        yield conn
    finally:
        conn.close()


@pytest.fixture()
def test_db():
    fd, path = tempfile.mkstemp(suffix=".db")
    os.close(fd)
    conn = get_db(path)
    init_db(conn)
    yield conn, path
    conn.close()
    os.unlink(path)


@pytest.fixture()
def client(test_db):
    global _db_path
    conn, path = test_db
    _db_path = path
    app.dependency_overrides[_get_conn] = _override_get_conn
    with TestClient(app) as tc:
        yield tc
    app.dependency_overrides.clear()
