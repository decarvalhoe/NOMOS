from fastapi import FastAPI

from app.routers import policies

app = FastAPI(title="Policy API")
app.include_router(policies.router, prefix="/api/v1")
