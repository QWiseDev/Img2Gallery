from pathlib import Path

from app.shared.config import get_settings
from app.shared.database import get_db


def get_setting(key: str, default: str) -> str:
    with get_db() as db:
        row = db.execute("SELECT value FROM app_settings WHERE key = ?", (key,)).fetchone()
    return row["value"] if row else default


def set_setting(key: str, value: str) -> None:
    with get_db() as db:
        db.execute(
            """
            INSERT INTO app_settings (key, value, updated_at)
            VALUES (?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
            """,
            (key, value),
        )


def get_concurrency() -> int:
    try:
        return max(1, min(8, int(get_setting("generation_concurrency", "1"))))
    except ValueError:
        return 1


def provider_out(row) -> dict:
    key = row["api_key"] or ""
    preview = f"...{key[-4:]}" if len(key) >= 4 else ""
    return {
        "id": row["id"],
        "name": row["name"],
        "provider_type": row["provider_type"],
        "model": row["model"],
        "api_base": row["api_base"],
        "enabled": bool(row["enabled"]),
        "is_default": bool(row["is_default"]),
        "api_key_set": bool(key),
        "api_key_preview": preview,
        "updated_at": row["updated_at"],
    }


def active_provider() -> dict | None:
    with get_db() as db:
        row = db.execute(
            """
            SELECT *
            FROM model_providers
            WHERE enabled = 1
            ORDER BY is_default DESC, id ASC
            LIMIT 1
            """
        ).fetchone()
    return dict(row) if row else None


def list_providers() -> list[dict]:
    with get_db() as db:
        rows = db.execute("SELECT * FROM model_providers ORDER BY is_default DESC, id ASC").fetchall()
    return [provider_out(row) for row in rows]


def upsert_provider(payload: dict) -> dict:
    with get_db() as db:
        current = None
        if payload.get("id"):
            current = db.execute("SELECT * FROM model_providers WHERE id = ?", (payload["id"],)).fetchone()
        api_key = payload.get("api_key")
        if api_key is None and current:
            api_key = current["api_key"]
        if payload.get("is_default"):
            db.execute("UPDATE model_providers SET is_default = 0")
        if current:
            db.execute(
                """
                UPDATE model_providers
                SET name = ?, provider_type = ?, model = ?, api_base = ?, api_key = ?,
                    enabled = ?, is_default = ?, updated_at = CURRENT_TIMESTAMP
                WHERE id = ?
                """,
                (
                    payload["name"],
                    payload["provider_type"],
                    payload["model"],
                    payload["api_base"],
                    api_key or "",
                    int(payload["enabled"]),
                    int(payload["is_default"]),
                    payload["id"],
                ),
            )
            provider_id = payload["id"]
        else:
            cursor = db.execute(
                """
                INSERT INTO model_providers (
                    name, provider_type, model, api_base, api_key, enabled, is_default
                )
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    payload["name"],
                    payload["provider_type"],
                    payload["model"],
                    payload["api_base"],
                    api_key or "",
                    int(payload["enabled"]),
                    int(payload["is_default"]),
                ),
            )
            provider_id = cursor.lastrowid
        row = db.execute("SELECT * FROM model_providers WHERE id = ?", (provider_id,)).fetchone()
    return provider_out(row)


def users_overview() -> list[dict]:
    with get_db() as db:
        rows = db.execute(
            """
            SELECT
                users.id,
                users.username,
                users.display_name,
                users.avatar_color,
                users.is_admin,
                users.last_login_ip,
                users.last_login_at,
                users.created_at,
                COUNT(images.id) AS total_generations,
                SUM(CASE WHEN images.status = 'ready' THEN 1 ELSE 0 END) AS ready_count,
                SUM(CASE WHEN images.status = 'failed' THEN 1 ELSE 0 END) AS failed_count,
                SUM(CASE WHEN images.status IN ('queued', 'running') THEN 1 ELSE 0 END) AS active_count
            FROM users
            LEFT JOIN images ON images.user_id = users.id
            GROUP BY users.id
            ORDER BY users.created_at DESC
            """
        ).fetchall()
    return [dict(row) for row in rows]


def set_user_admin(user_id: int, is_admin: bool) -> dict | None:
    with get_db() as db:
        db.execute("UPDATE users SET is_admin = ? WHERE id = ?", (int(is_admin), user_id))
        row = db.execute(
            """
            SELECT id, username, display_name, avatar_color, is_admin
            FROM users
            WHERE id = ?
            """,
            (user_id,),
        ).fetchone()
    return dict(row) if row else None


def generation_records(limit: int = 120) -> list[dict]:
    with get_db() as db:
        rows = db.execute(
            """
            SELECT
                images.id,
                images.image_path,
                images.task_type,
                images.source_image_path,
                images.prompt,
                images.status,
                images.error,
                images.request_ip,
                images.provider_name,
                images.model,
                images.queued_at,
                images.started_at,
                images.completed_at,
                images.created_at,
                users.username,
                users.display_name
            FROM images
            JOIN users ON users.id = images.user_id
            ORDER BY images.id DESC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
    return [dict(row) for row in rows]


def delete_generation(image_id: int) -> dict | None:
    settings = get_settings()
    with get_db() as db:
        row = db.execute(
            "SELECT id, image_path, source_image_path FROM images WHERE id = ?",
            (image_id,),
        ).fetchone()
        if not row:
            return None
        db.execute("DELETE FROM images WHERE id = ?", (image_id,))
    delete_storage_file(settings.storage_dir, row["image_path"])
    delete_storage_file(settings.storage_dir, row["source_image_path"])
    return {"id": image_id}


def delete_storage_file(storage_dir: Path, relative_path: str | None) -> None:
    if not relative_path:
        return
    root = storage_dir.resolve()
    path = (storage_dir / relative_path).resolve()
    if root != path and root not in path.parents:
        return
    if path.exists() and path.is_file():
        path.unlink()


def dashboard() -> dict:
    with get_db() as db:
        totals = db.execute(
            """
            SELECT
                COUNT(*) AS total,
                SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END) AS queued,
                SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running,
                SUM(CASE WHEN status = 'ready' THEN 1 ELSE 0 END) AS ready,
                SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed
            FROM images
            """
        ).fetchone()
        users = db.execute("SELECT COUNT(*) AS total FROM users").fetchone()
    return {
        "users": users["total"],
        "images": {key: totals[key] or 0 for key in totals.keys()},
        "concurrency": get_concurrency(),
        "providers": list_providers(),
    }
