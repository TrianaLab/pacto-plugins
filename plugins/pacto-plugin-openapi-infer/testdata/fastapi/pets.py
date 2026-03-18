from fastapi import APIRouter
from models import Pet, PetCreate

router = APIRouter(prefix="/pets")


@router.get("/", summary="List all pets")
def list_pets(limit: int = 20) -> list[Pet]:
    return []


@router.get("/{petId}", summary="Get a pet")
def get_pet(petId: int) -> Pet:
    pass


@router.post("/", summary="Create a pet")
def create_pet(pet: PetCreate) -> Pet:
    pass


@router.delete("/{petId}", summary="Delete a pet")
def delete_pet(petId: int) -> None:
    pass
