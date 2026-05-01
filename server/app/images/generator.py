import base64
import mimetypes
import secrets
from pathlib import Path

import httpx

from app.shared.config import get_settings

REQUEST_TIMEOUT_SECONDS = 600


class ImageGenerationError(Exception):
    pass


async def generate_and_store(prompt: str, provider: dict) -> str:
    settings = get_settings()
    settings.storage_dir.mkdir(parents=True, exist_ok=True)
    payload = await request_image(provider, prompt)
    image_bytes, suffix = await extract_image(settings, payload)
    return store_image_bytes(settings, image_bytes, suffix)


async def edit_and_store(prompt: str, source_image_path: str | None, provider: dict) -> str:
    settings = get_settings()
    settings.storage_dir.mkdir(parents=True, exist_ok=True)
    payload = await request_image_edit(provider, prompt, source_image_path)
    image_bytes, suffix = await extract_image(settings, payload)
    return store_image_bytes(settings, image_bytes, suffix)


def store_image_bytes(settings, image_bytes: bytes, suffix: str) -> str:
    filename = f"{secrets.token_hex(12)}{suffix}"
    path = settings.storage_dir / filename
    path.write_bytes(image_bytes)
    return filename


async def request_image(provider: dict, prompt: str) -> dict:
    validate_provider(provider)
    url = f"{provider['api_base'].rstrip('/')}/v1/images/generations"
    body = {
        "model": provider["model"],
        "prompt": prompt,
        "n": 1,
        "response_format": "b64_json",
    }
    headers = {"Authorization": f"Bearer {provider['api_key']}"}
    try:
        async with httpx.AsyncClient(timeout=REQUEST_TIMEOUT_SECONDS) as client:
            response = await client.post(url, json=body, headers=headers)
            response.raise_for_status()
            return response.json()
    except httpx.HTTPStatusError as exc:
        detail = exc.response.text.strip().replace("\n", " ")[:240]
        message = f"生图接口返回 {exc.response.status_code}"
        if detail:
            message = f"{message}：{detail}"
        raise ImageGenerationError(message) from exc
    except httpx.TimeoutException as exc:
        raise ImageGenerationError(f"生图接口请求超时（超过 {REQUEST_TIMEOUT_SECONDS} 秒）") from exc
    except httpx.HTTPError as exc:
        detail = str(exc).strip() or exc.__class__.__name__
        raise ImageGenerationError(f"生图接口请求失败：{detail}") from exc


async def request_image_edit(provider: dict, prompt: str, source_image_path: str | None) -> dict:
    validate_provider(provider)
    source_path = resolve_source_image(source_image_path)
    url = f"{provider['api_base'].rstrip('/')}/v1/images/edits"
    headers = {"Authorization": f"Bearer {provider['api_key']}"}
    content_type = mimetypes.guess_type(source_path.name)[0] or "application/octet-stream"
    data = {
        "model": provider["model"],
        "prompt": prompt,
        "n": "1",
    }
    try:
        async with httpx.AsyncClient(timeout=REQUEST_TIMEOUT_SECONDS) as client:
            with source_path.open("rb") as image_file:
                files = {"image": (source_path.name, image_file, content_type)}
                response = await client.post(url, data=data, files=files, headers=headers)
            response.raise_for_status()
            return response.json()
    except httpx.HTTPStatusError as exc:
        detail = exc.response.text.strip().replace("\n", " ")[:240]
        message = f"图片编辑接口返回 {exc.response.status_code}"
        if detail:
            message = f"{message}：{detail}"
        raise ImageGenerationError(message) from exc
    except httpx.TimeoutException as exc:
        raise ImageGenerationError(f"图片编辑接口请求超时（超过 {REQUEST_TIMEOUT_SECONDS} 秒）") from exc
    except httpx.HTTPError as exc:
        detail = str(exc).strip() or exc.__class__.__name__
        raise ImageGenerationError(f"图片编辑接口请求失败：{detail}") from exc


def validate_provider(provider: dict) -> None:
    if not provider:
        raise ImageGenerationError("未配置模型提供商")
    if provider.get("provider_type") != "openai_compatible":
        raise ImageGenerationError("当前提供商类型暂未支持")
    if not provider.get("api_base"):
        raise ImageGenerationError("未配置当前模型提供商 API 地址")
    if not provider.get("api_key"):
        raise ImageGenerationError("未配置当前模型提供商 API Key")


def resolve_source_image(source_image_path: str | None) -> Path:
    if not source_image_path:
        raise ImageGenerationError("图片编辑任务缺少原图")
    settings = get_settings()
    root = settings.storage_dir.resolve()
    path = (settings.storage_dir / source_image_path).resolve()
    if root != path and root not in path.parents:
        raise ImageGenerationError("图片编辑任务原图路径无效")
    if not path.exists() or not path.is_file():
        raise ImageGenerationError("图片编辑任务原图不存在")
    return path


async def extract_image(settings, payload: dict) -> tuple[bytes, str]:
    data = payload.get("data") or []
    if not data:
        raise ImageGenerationError("生图接口未返回图片数据")
    first = data[0]
    if first.get("b64_json"):
        return base64.b64decode(first["b64_json"]), ".png"
    if first.get("url"):
        return await download_image(first["url"])
    raise ImageGenerationError("不支持的生图接口返回格式")


async def download_image(url: str) -> tuple[bytes, str]:
    async with httpx.AsyncClient(timeout=60) as client:
        response = await client.get(url)
        response.raise_for_status()
    content_type = response.headers.get("content-type", "").split(";")[0]
    suffix = mimetypes.guess_extension(content_type) or Path(url).suffix or ".png"
    return response.content, suffix
