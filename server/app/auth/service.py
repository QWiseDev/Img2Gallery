import hashlib
import hmac
import secrets
from datetime import datetime, timedelta, timezone

from fastapi import HTTPException, Request, Response, status

from app.shared.database import get_db
from app.shared.time import local_timestamp

SESSION_COOKIE = "gallery_session"
SESSION_DAYS = 14
AVATAR_COLORS = ("#f97316", "#14b8a6", "#6366f1", "#e11d48", "#0f766e")


def hash_password(password: str, salt: str | None = None) -> str:
    password_salt = salt or secrets.token_hex(16)
    digest = hashlib.pbkdf2_hmac(
        "sha256", password.encode("utf-8"), password_salt.encode("utf-8"), 120_000
    )
    return f"{password_salt}${digest.hex()}"


def verify_password(password: str, stored_hash: str) -> bool:
    salt, _ = stored_hash.split("$", 1)
    return hmac.compare_digest(hash_password(password, salt), stored_hash)


def row_to_user(row) -> dict:
    return {
        "id": row["id"],
        "username": row["username"],
        "display_name": row["display_name"],
        "avatar_color": row["avatar_color"],
        "is_admin": bool(row["is_admin"]),
    }


def create_user(username: str, password: str, display_name: str | None) -> dict:
    normalized = username.strip().lower()
    shown_name = (display_name or username).strip() or username
    color = AVATAR_COLORS[sum(ord(ch) for ch in normalized) % len(AVATAR_COLORS)]
    try:
        with get_db() as db:
            cursor = db.execute(
                """
                INSERT INTO users (username, display_name, password_hash, avatar_color, created_at)
                VALUES (?, ?, ?, ?, ?)
                """,
                (normalized, shown_name, hash_password(password), color, local_timestamp()),
            )
            row = db.execute("SELECT * FROM users WHERE id = ?", (cursor.lastrowid,)).fetchone()
    except Exception as exc:
        if "UNIQUE constraint failed" in str(exc):
            raise HTTPException(status_code=409, detail="用户名已存在") from exc
        raise
    return row_to_user(row)


def authenticate(username: str, password: str) -> dict:
    with get_db() as db:
        row = db.execute(
            "SELECT * FROM users WHERE username = ?", (username.strip().lower(),)
        ).fetchone()
    if not row or not verify_password(password, row["password_hash"]):
        raise HTTPException(status_code=401, detail="用户名或密码错误")
    return row_to_user(row)


def record_login(user_id: int, ip_address: str | None) -> None:
    with get_db() as db:
        db.execute(
            """
            UPDATE users
            SET last_login_ip = ?, last_login_at = ?
            WHERE id = ?
            """,
            (ip_address, local_timestamp(), user_id),
        )


def issue_session(response: Response, user_id: int) -> None:
    token = secrets.token_urlsafe(32)
    expires = datetime.now(timezone.utc) + timedelta(days=SESSION_DAYS)
    with get_db() as db:
        db.execute(
            "INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
            (token, user_id, expires.isoformat()),
        )
    response.set_cookie(
        SESSION_COOKIE,
        token,
        max_age=SESSION_DAYS * 24 * 3600,
        httponly=True,
        samesite="lax",
    )


def clear_session(request: Request, response: Response) -> None:
    token = request.cookies.get(SESSION_COOKIE)
    if token:
        with get_db() as db:
            db.execute("DELETE FROM sessions WHERE token = ?", (token,))
    response.delete_cookie(SESSION_COOKIE)


def current_user(request: Request) -> dict:
    token = request.cookies.get(SESSION_COOKIE)
    if not token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="请先登录")
    with get_db() as db:
        row = db.execute(
            """
            SELECT users.*
            FROM sessions
            JOIN users ON users.id = sessions.user_id
            WHERE sessions.token = ? AND sessions.expires_at > ?
            """,
            (token, datetime.now(timezone.utc).isoformat()),
        ).fetchone()
    if not row:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="登录已过期")
    return row_to_user(row)
