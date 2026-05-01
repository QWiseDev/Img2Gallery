import secrets

from fastapi import APIRouter, Depends, File, Form, HTTPException, Request, UploadFile
from fastapi.responses import StreamingResponse

from app.auth.service import current_user
from app.shared.config import get_settings

from .queue import job_events
from .repository import add_image, get_image, list_images, list_user_images, toggle_relation
from .schemas import ImageCreate, ImageOut

router = APIRouter(prefix="/api/images", tags=["images"])
MAX_SOURCE_IMAGE_BYTES = 10 * 1024 * 1024
SOURCE_IMAGE_TYPES = {
    "image/png": ".png",
    "image/jpeg": ".jpg",
    "image/webp": ".webp",
}
DEFAULT_PAGE_SIZE = 24
MAX_PAGE_SIZE = 48


def optional_user(request: Request) -> dict | None:
    try:
        return current_user(request)
    except HTTPException:
        return None


def client_ip(request: Request) -> str | None:
    forwarded = request.headers.get("x-forwarded-for")
    if forwarded:
        return forwarded.split(",", 1)[0].strip()
    return request.client.host if request.client else None


@router.get("", response_model=list[ImageOut])
def gallery(
    sort: str = "latest",
    limit: int = DEFAULT_PAGE_SIZE,
    offset: int = 0,
    user: dict | None = Depends(optional_user),
):
    safe_sort = sort if sort in {"latest", "popular", "favorites"} else "latest"
    return list_images(user["id"] if user else None, safe_sort, clamp_limit(limit), max(0, offset))


@router.get("/mine", response_model=list[ImageOut])
def my_images(limit: int = DEFAULT_PAGE_SIZE, offset: int = 0, user: dict = Depends(current_user)):
    return list_user_images(user["id"], clamp_limit(limit), max(0, offset))


def clamp_limit(limit: int) -> int:
    return max(1, min(MAX_PAGE_SIZE, limit))


@router.post("", response_model=ImageOut)
async def create_image(payload: ImageCreate, request: Request, user: dict = Depends(current_user)):
    prompt = payload.prompt.strip()
    image_id = add_image(user["id"], prompt, "queued", client_ip(request))
    created = get_image(image_id, user["id"])
    if not created:
        raise HTTPException(status_code=500, detail="图片记录创建失败")
    return created


@router.post("/edit", response_model=ImageOut)
async def create_edit_image(
    request: Request,
    prompt: str = Form(..., min_length=2, max_length=4000),
    image: UploadFile = File(...),
    user: dict = Depends(current_user),
):
    clean_prompt = prompt.strip()
    if len(clean_prompt) < 2:
        raise HTTPException(status_code=422, detail="提示词至少需要 2 个字符")
    source_image_path = await save_source_image(image)
    image_id = add_image(
        user["id"],
        clean_prompt,
        "queued",
        client_ip(request),
        task_type="edit",
        source_image_path=source_image_path,
    )
    created = get_image(image_id, user["id"])
    if not created:
        raise HTTPException(status_code=500, detail="图片编辑记录创建失败")
    return created


async def save_source_image(upload: UploadFile) -> str:
    content_type = (upload.content_type or "").split(";", 1)[0].lower()
    suffix = SOURCE_IMAGE_TYPES.get(content_type)
    if not suffix:
        raise HTTPException(status_code=400, detail="仅支持 PNG、JPG、WEBP 图片")
    content = await upload.read(MAX_SOURCE_IMAGE_BYTES + 1)
    if not content:
        raise HTTPException(status_code=400, detail="上传图片不能为空")
    if len(content) > MAX_SOURCE_IMAGE_BYTES:
        raise HTTPException(status_code=413, detail="上传图片不能超过 10MB")
    settings = get_settings()
    source_dir = settings.storage_dir / "sources"
    source_dir.mkdir(parents=True, exist_ok=True)
    filename = f"{secrets.token_hex(12)}{suffix}"
    (source_dir / filename).write_bytes(content)
    return f"sources/{filename}"


@router.get("/{image_id}/events")
def image_events(image_id: int, user: dict = Depends(current_user)):
    image = get_image(image_id, user["id"])
    if not image:
        raise HTTPException(status_code=404, detail="图片不存在")
    if image["author"]["id"] != user["id"]:
        raise HTTPException(status_code=403, detail="无权查看该任务")
    return StreamingResponse(job_events(image_id, user["id"]), media_type="text/event-stream")


@router.post("/{image_id}/like", response_model=ImageOut)
def like_image(image_id: int, user: dict = Depends(current_user)):
    if not get_image(image_id, user["id"]):
        raise HTTPException(status_code=404, detail="图片不存在")
    toggle_relation("image_likes", image_id, user["id"])
    return get_image(image_id, user["id"])


@router.post("/{image_id}/favorite", response_model=ImageOut)
def favorite_image(image_id: int, user: dict = Depends(current_user)):
    if not get_image(image_id, user["id"]):
        raise HTTPException(status_code=404, detail="图片不存在")
    toggle_relation("image_favorites", image_id, user["id"])
    return get_image(image_id, user["id"])
