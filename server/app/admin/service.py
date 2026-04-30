import hmac
import secrets
from datetime import datetime, timedelta, timezone

from fastapi import HTTPException, Request, Response, status

from app.auth.service import SESSION_COOKIE
from app.shared.config import get_settings
from app.shared.database import get_db

ADMIN_COOKIE = "gallery_admin_session"
ADMIN_SESSION_HOURS = 12


def require_admin(request: Request) -> dict:
    user_admin = admin_from_user_session(request)
    if user_admin:
        return user_admin
    token = request.cookies.get(ADMIN_COOKIE)
    if not token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="请先登录管理员")
    with get_db() as db:
        row = db.execute(
            """
            SELECT token, expires_at
            FROM admin_sessions
            WHERE token = ? AND expires_at > ?
            """,
            (token, datetime.now(timezone.utc).isoformat()),
        ).fetchone()
    if not row:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="管理员登录已过期")
    return {"role": "admin", "source": "password"}


def admin_from_user_session(request: Request) -> dict | None:
    token = request.cookies.get(SESSION_COOKIE)
    if not token:
        return None
    with get_db() as db:
        row = db.execute(
            """
            SELECT users.id, users.username
            FROM sessions
            JOIN users ON users.id = sessions.user_id
            WHERE sessions.token = ?
              AND sessions.expires_at > ?
              AND users.is_admin = 1
            """,
            (token, datetime.now(timezone.utc).isoformat()),
        ).fetchone()
    if not row:
        return None
    return {"role": "admin", "source": "user", "user_id": row["id"], "username": row["username"]}


def login_admin(password: str, response: Response) -> None:
    settings = get_settings()
    if not hmac.compare_digest(password, settings.admin_password):
        raise HTTPException(status_code=401, detail="管理员密码错误")
    token = secrets.token_urlsafe(32)
    expires = datetime.now(timezone.utc) + timedelta(hours=ADMIN_SESSION_HOURS)
    with get_db() as db:
        db.execute(
            "INSERT INTO admin_sessions (token, expires_at) VALUES (?, ?)",
            (token, expires.isoformat()),
        )
    response.set_cookie(
        ADMIN_COOKIE,
        token,
        max_age=ADMIN_SESSION_HOURS * 3600,
        httponly=True,
        samesite="lax",
    )


def logout_admin(request: Request, response: Response) -> None:
    token = request.cookies.get(ADMIN_COOKIE)
    if token:
        with get_db() as db:
            db.execute("DELETE FROM admin_sessions WHERE token = ?", (token,))
    response.delete_cookie(ADMIN_COOKIE)
