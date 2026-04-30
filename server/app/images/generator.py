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
    filename = f"{secrets.token_hex(12)}{suffix}"
    path = settings.storage_dir / filename
    path.write_bytes(image_bytes)
    return filename


async def request_image(provider: dict, prompt: str) -> dict:
    if not provider:
        raise ImageGenerationError("未配置模型提供商")
    if provider.get("provider_type") != "openai_compatible":
        raise ImageGenerationError("当前提供商类型暂未支持")
    if not provider.get("api_key"):
        raise ImageGenerationError("未配置当前模型提供商 API Key")
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
