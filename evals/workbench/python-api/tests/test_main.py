def test_create_item(client):
    resp = client.post(
        "/items", json={"name": "Widget", "description": "A fine widget", "price": 9.99}
    )
    assert resp.status_code == 201
    data = resp.json()
    assert data["name"] == "Widget"
    assert data["description"] == "A fine widget"
    assert data["price"] == 9.99
    assert "id" in data
    assert "created_at" in data


def test_create_item_invalid_price(client):
    resp = client.post("/items", json={"name": "Bad", "price": -5.0})
    assert resp.status_code == 422


def test_create_item_empty_name(client):
    resp = client.post("/items", json={"name": "  ", "price": 1.0})
    assert resp.status_code == 422


def test_get_item(client):
    create = client.post("/items", json={"name": "Gadget", "price": 12.50})
    item_id = create.json()["id"]
    resp = client.get(f"/items/{item_id}")
    assert resp.status_code == 200
    assert resp.json()["name"] == "Gadget"
    assert resp.json()["price"] == 12.50


def test_get_item_not_found(client):
    resp = client.get("/items/9999")
    assert resp.status_code == 404


def test_list_items_empty(client):
    resp = client.get("/items")
    assert resp.status_code == 200
    data = resp.json()
    assert data["items"] == []
    assert data["total"] == 0
    assert data["page"] == 1
    assert data["size"] == 20


def test_list_items_pagination(client):
    for i in range(5):
        client.post("/items", json={"name": f"Item {i}", "price": float(i + 1)})
    resp = client.get("/items?page=1&size=2")
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["items"]) == 2
    assert data["total"] == 5
    assert data["page"] == 1
    assert data["size"] == 2
    assert data["items"][0]["name"] == "Item 0"
    assert data["items"][1]["name"] == "Item 1"


def test_delete_item(client):
    create = client.post("/items", json={"name": "Doomed", "price": 3.0})
    item_id = create.json()["id"]
    resp = client.delete(f"/items/{item_id}")
    assert resp.status_code == 204
    resp = client.get(f"/items/{item_id}")
    assert resp.status_code == 404


def test_delete_item_not_found(client):
    resp = client.delete("/items/9999")
    assert resp.status_code == 404
