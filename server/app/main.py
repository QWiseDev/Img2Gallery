from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles

from app.admin.routes import router as admin_router
from app.auth.routes import router as auth_router
from app.images.queue import start_queue_manager, stop_queue_manager
from app.images.routes import router as images_router
from app.shared.config import get_settings
from app.shared.database import init_db

settings = get_settings()

app = FastAPI(title="AI Prompt Gallery")

app.add_middleware(
    CORSMiddleware,
    allow_origins=[settings.client_origin, "http://127.0.0.1:5173"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.on_event("startup")
async def startup() -> None:
    settings.storage_dir.mkdir(parents=True, exist_ok=True)
    init_db()
    await start_queue_manager()


@app.on_event("shutdown")
async def shutdown() -> None:
    await stop_queue_manager()


@app.get("/health")
def health():
    return {"status": "ok"}


app.include_router(auth_router)
app.include_router(images_router)
app.include_router(admin_router)
app.mount("/media", StaticFiles(directory=settings.storage_dir), name="media")

frontend_dist = settings.project_root / "client" / "dist"
frontend_assets = frontend_dist / "assets"

if frontend_assets.exists():
    app.mount("/assets", StaticFiles(directory=frontend_assets), name="assets")


@app.get("/{full_path:path}", include_in_schema=False)
def serve_frontend(full_path: str):
    index_file = frontend_dist / "index.html"
    requested_file = frontend_dist / full_path
    if requested_file.is_file():
        return FileResponse(requested_file)
    if index_file.exists():
        return FileResponse(index_file)
    return {"detail": "Frontend has not been built"}
