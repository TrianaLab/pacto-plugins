from pydantic import BaseModel


class Pet(BaseModel):
    id: int
    name: str
    tag: str


class PetCreate(BaseModel):
    name: str
    tag: str
