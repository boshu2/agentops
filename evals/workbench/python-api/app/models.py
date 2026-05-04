from pydantic import BaseModel, field_validator


class ItemCreate(BaseModel):
    name: str
    description: str = ""
    price: float

    @field_validator("price")
    @classmethod
    def price_must_be_positive(cls, v: float) -> float:
        if v <= 0:
            raise ValueError("price must be greater than 0")
        return v

    @field_validator("name")
    @classmethod
    def name_must_be_nonempty(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("name must not be empty")
        return v


class ItemResponse(BaseModel):
    id: int
    name: str
    description: str
    price: float
    created_at: str


class ItemList(BaseModel):
    items: list[ItemResponse]
    total: int
    page: int
    size: int
