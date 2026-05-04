import sqlite3
from datetime import datetime, timezone


def get_db(path: str = "items.db") -> sqlite3.Connection:
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    return conn


def init_db(conn: sqlite3.Connection) -> None:
    conn.execute("""
        CREATE TABLE IF NOT EXISTS items (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            description TEXT,
            price REAL NOT NULL,
            created_at TEXT NOT NULL
        )
        """)
    conn.commit()


def insert_item(conn: sqlite3.Connection, item: dict) -> int:
    now = datetime.now(timezone.utc).isoformat()
    cursor = conn.execute(
        "INSERT INTO items (name, description, price, created_at) VALUES (?, ?, ?, ?)",
        (item["name"], item["description"], item["price"], now),
    )
    conn.commit()
    return cursor.lastrowid  # type: ignore[return-value]


def get_item(conn: sqlite3.Connection, item_id: int) -> dict | None:
    row = conn.execute("SELECT * FROM items WHERE id = ?", (item_id,)).fetchone()
    if row is None:
        return None
    return dict(row)


def list_items(conn: sqlite3.Connection, page: int = 1, size: int = 20) -> tuple[list[dict], int]:
    total = conn.execute("SELECT COUNT(*) FROM items").fetchone()[0]
    offset = (page - 1) * size
    rows = conn.execute(
        "SELECT * FROM items ORDER BY id LIMIT ? OFFSET ?", (size, offset)
    ).fetchall()
    return [dict(r) for r in rows], total


def delete_item(conn: sqlite3.Connection, item_id: int) -> bool:
    cursor = conn.execute("DELETE FROM items WHERE id = ?", (item_id,))
    conn.commit()
    return cursor.rowcount > 0
