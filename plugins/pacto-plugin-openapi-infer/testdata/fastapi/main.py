from fastapi import FastAPI
from pets import router as pets_router

app = FastAPI(
    title="Pet Store",
    version="1.0.0",
)

app.include_router(pets_router)
