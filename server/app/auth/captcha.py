import base64
import html
import secrets
import string
import time
from threading import Lock

from fastapi import HTTPException

CAPTCHA_TTL_SECONDS = 300
CAPTCHA_CHARS = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

_captcha_store: dict[str, tuple[str, float]] = {}
_captcha_lock = Lock()


def create_captcha() -> dict:
    cleanup_expired()
    token = secrets.token_urlsafe(24)
    code = "".join(secrets.choice(CAPTCHA_CHARS) for _ in range(5))
    expires_at = time.time() + CAPTCHA_TTL_SECONDS
    with _captcha_lock:
        _captcha_store[token] = (code, expires_at)
    svg = render_svg(code)
    encoded = base64.b64encode(svg.encode("utf-8")).decode("ascii")
    return {"token": token, "image": f"data:image/svg+xml;base64,{encoded}"}


def verify_captcha(token: str, answer: str) -> None:
    normalized = answer.strip().upper()
    with _captcha_lock:
        item = _captcha_store.pop(token, None)
    if not item:
        raise HTTPException(status_code=400, detail="验证码已过期，请刷新")
    code, expires_at = item
    if expires_at < time.time():
        raise HTTPException(status_code=400, detail="验证码已过期，请刷新")
    if normalized != code:
        raise HTTPException(status_code=400, detail="验证码错误")


def cleanup_expired() -> None:
    now = time.time()
    with _captcha_lock:
        expired = [token for token, (_, expires_at) in _captcha_store.items() if expires_at < now]
        for token in expired:
            _captcha_store.pop(token, None)


def render_svg(code: str) -> str:
    escaped = html.escape(code)
    lines = []
    dots = []
    for _ in range(5):
        x1, y1, x2, y2 = (secrets.randbelow(limit) for limit in (160, 56, 160, 56))
        color = secrets.choice(("#fb923c", "#14b8a6", "#64748b", "#f59e0b"))
        lines.append(
            f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{color}" '
            'stroke-width="1.4" stroke-opacity="0.34" />'
        )
    for _ in range(26):
        cx, cy = secrets.randbelow(160), secrets.randbelow(56)
        dots.append(f'<circle cx="{cx}" cy="{cy}" r="1.2" fill="#94a3b8" fill-opacity="0.34" />')
    chars = []
    for index, char in enumerate(escaped):
        x = 24 + index * 24
        y = 35 + secrets.randbelow(7) - 3
        rotate = secrets.randbelow(18) - 9
        chars.append(
            f'<text x="{x}" y="{y}" transform="rotate({rotate} {x} {y})" '
            'font-size="25" font-weight="900" font-family="Arial, sans-serif" '
            'fill="#182136">'
            f"{char}</text>"
        )
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" width="160" height="56" viewBox="0 0 160 56">'
        '<rect width="160" height="56" rx="14" fill="#fff7ed"/>'
        '<rect x="0.5" y="0.5" width="159" height="55" rx="13.5" fill="none" stroke="#fed7aa"/>'
        + "".join(lines)
        + "".join(dots)
        + "".join(chars)
        + "</svg>"
    )
