from pydantic import BaseModel, Field


class ImageCreate(BaseModel):
    prompt: str = Field(min_length=2, max_length=4000)


class ImageOut(BaseModel):
    id: int
    prompt: str
    image_url: str | None
    task_type: str = "generate"
    source_image_url: str | None = None
    is_hidden: bool = False
    status: str
    error: str | None
    request_ip: str | None
    provider_name: str | None
    model: str | None
    queued_at: str | None
    started_at: str | None
    completed_at: str | None
    created_at: str
    author: dict
    likes: int
    favorites: int
    liked_by_me: bool
    favorited_by_me: bool
