from contextlib import asynccontextmanager
from typing import AsyncIterator

from fastapi import Depends, FastAPI, HTTPException, Response

from app.db import delete_item, get_db, get_item, init_db, insert_item, list_items
from app.models import ItemCreate, ItemList, ItemResponse

_db_path: str = "items.db"


def _get_conn():
    conn = get_db(_db_path)
    try:
        yield conn
    finally:
        conn.close()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    conn = get_db(_db_path)
    init_db(conn)
    conn.close()
    yield


app = FastAPI(lifespan=lifespan)


@app.post("/items", status_code=201, response_model=ItemResponse)
def create_item(item: ItemCreate, conn=Depends(_get_conn)):
    item_id = insert_item(conn, item.model_dump())
    row = get_item(conn, item_id)
    return row


@app.get("/items", response_model=ItemList)
def read_items(page: int = 1, size: int = 20, conn=Depends(_get_conn)):
    items, total = list_items(conn, page=page, size=size)
    return ItemList(items=items, total=total, page=page, size=size)


@app.get("/items/{item_id}", response_model=ItemResponse)
def read_item(item_id: int, conn=Depends(_get_conn)):
    row = get_item(conn, item_id)
    if row is None:
        raise HTTPException(status_code=404, detail="Item not found")
    return row


@app.delete("/items/{item_id}", status_code=204)
def remove_item(item_id: int, conn=Depends(_get_conn)):
    if not delete_item(conn, item_id):
        raise HTTPException(status_code=404, detail="Item not found")
    return Response(status_code=204)
