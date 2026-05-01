from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import StreamingResponse

from app.auth.service import current_user

from .queue import job_events
from .repository import add_image, get_image, list_images, list_user_images, toggle_relation
from .schemas import ImageCreate, ImageOut

router = APIRouter(prefix="/api/images", tags=["images"])


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
def gallery(sort: str = "latest", user: dict | None = Depends(optional_user)):
    safe_sort = sort if sort in {"latest", "popular", "favorites"} else "latest"
    return list_images(user["id"] if user else None, safe_sort)


@router.get("/mine", response_model=list[ImageOut])
def my_images(user: dict = Depends(current_user)):
    return list_user_images(user["id"])


@router.post("", response_model=ImageOut)
async def create_image(payload: ImageCreate, request: Request, user: dict = Depends(current_user)):
    prompt = payload.prompt.strip()
    image_id = add_image(user["id"], prompt, "queued", client_ip(request))
    created = get_image(image_id, user["id"])
    if not created:
        raise HTTPException(status_code=500, detail="图片记录创建失败")
    return created


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
