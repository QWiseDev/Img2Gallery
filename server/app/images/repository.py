from app.shared.database import get_db
from app.shared.time import local_timestamp


def serialize_image(row) -> dict:
    image_path = row["image_path"]
    source_image_path = row["source_image_path"]
    return {
        "id": row["id"],
        "prompt": row["prompt"],
        "image_url": f"/media/{image_path}" if image_path else None,
        "task_type": row["task_type"],
        "source_image_path": source_image_path,
        "source_image_url": f"/media/{source_image_path}" if source_image_path else None,
        "is_hidden": bool(row["is_hidden"]),
        "status": row["status"],
        "error": row["error"],
        "request_ip": row["request_ip"],
        "provider_name": row["provider_name"],
        "model": row["model"],
        "queued_at": row["queued_at"],
        "started_at": row["started_at"],
        "completed_at": row["completed_at"],
        "created_at": row["created_at"],
        "author": {
            "id": row["user_id"],
            "username": row["username"],
            "display_name": row["display_name"],
            "avatar_color": row["avatar_color"],
        },
        "likes": row["likes"],
        "favorites": row["favorites"],
        "liked_by_me": bool(row["liked_by_me"]),
        "favorited_by_me": bool(row["favorited_by_me"]),
    }


def add_image(
    user_id: int,
    prompt: str,
    status: str,
    request_ip: str | None,
    image_path: str | None = None,
    error: str | None = None,
    provider_name: str | None = None,
    model: str | None = None,
    task_type: str = "generate",
    source_image_path: str | None = None,
):
    with get_db() as db:
        cursor = db.execute(
            """
            INSERT INTO images (
                user_id, prompt, image_path, task_type, source_image_path, status, error,
                request_ip, provider_name, model, queued_at, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                user_id,
                prompt,
                image_path,
                task_type,
                source_image_path,
                status,
                error,
                request_ip,
                provider_name,
                model,
                local_timestamp(),
                local_timestamp(),
            ),
        )
        return cursor.lastrowid


def mark_running(image_id: int, provider_name: str, model: str) -> None:
    with get_db() as db:
        db.execute(
            """
            UPDATE images
            SET status = 'running', provider_name = ?, model = ?, started_at = ?
            WHERE id = ? AND status = 'queued'
            """,
            (provider_name, model, local_timestamp(), image_id),
        )


def mark_ready(image_id: int, image_path: str) -> None:
    with get_db() as db:
        db.execute(
            """
            UPDATE images
            SET status = 'ready', image_path = ?, error = NULL, completed_at = ?
            WHERE id = ?
            """,
            (image_path, local_timestamp(), image_id),
        )


def mark_failed(image_id: int, error: str) -> None:
    with get_db() as db:
        db.execute(
            """
            UPDATE images
            SET status = 'failed', error = ?, completed_at = ?
            WHERE id = ?
            """,
            (error, local_timestamp(), image_id),
        )


def reset_running_jobs() -> None:
    with get_db() as db:
        db.execute(
            """
            UPDATE images
            SET status = 'queued', started_at = NULL
            WHERE status = 'running'
            """
        )


def next_queued_jobs(limit: int) -> list[int]:
    with get_db() as db:
        rows = db.execute(
            """
            SELECT id
            FROM images
            WHERE status = 'queued'
            ORDER BY id ASC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
    return [row["id"] for row in rows]


def queue_counts() -> dict:
    with get_db() as db:
        row = db.execute(
            """
            SELECT
                SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END) AS queued,
                SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running
            FROM images
            """
        ).fetchone()
    return {"queued": row["queued"] or 0, "running": row["running"] or 0}


def queue_position(image_id: int) -> int | None:
    with get_db() as db:
        row = db.execute("SELECT status FROM images WHERE id = ?", (image_id,)).fetchone()
        if not row:
            return None
        if row["status"] != "queued":
            return 0
        position = db.execute(
            """
            SELECT COUNT(*) AS position
            FROM images
            WHERE status = 'queued' AND id <= ?
            """,
            (image_id,),
        ).fetchone()
    return position["position"]


def list_images(viewer_id: int | None, sort: str, limit: int, offset: int) -> list[dict]:
    order = "likes DESC, images.created_at DESC" if sort == "popular" else "images.created_at DESC"
    where_clauses = ["images.is_hidden = 0"]
    params: list[int] = [viewer_id or 0, viewer_id or 0]
    if sort == "favorites" and viewer_id:
        where_clauses.append(
            "EXISTS (SELECT 1 FROM image_favorites f WHERE f.image_id = images.id AND f.user_id = ?)"
        )
        params.append(viewer_id)
    where_sql = f"WHERE {' AND '.join(where_clauses)}"
    with get_db() as db:
        rows = db.execute(
            f"""
            SELECT
                images.*,
                users.username,
                users.display_name,
                users.avatar_color,
                COUNT(DISTINCT image_likes.user_id) AS likes,
                COUNT(DISTINCT image_favorites.user_id) AS favorites,
                EXISTS(
                    SELECT 1 FROM image_likes mine
                    WHERE mine.image_id = images.id AND mine.user_id = ?
                ) AS liked_by_me,
                EXISTS(
                    SELECT 1 FROM image_favorites fav
                    WHERE fav.image_id = images.id AND fav.user_id = ?
                ) AS favorited_by_me
            FROM images
            JOIN users ON users.id = images.user_id
            LEFT JOIN image_likes ON image_likes.image_id = images.id
            LEFT JOIN image_favorites ON image_favorites.image_id = images.id
            {where_sql}
            GROUP BY images.id
            ORDER BY {order}
            LIMIT ? OFFSET ?
            """,
            [*params, limit, offset],
        ).fetchall()
    return [serialize_image(row) for row in rows]


def list_user_images(user_id: int, limit: int, offset: int) -> list[dict]:
    with get_db() as db:
        rows = db.execute(
            """
            SELECT
                images.*,
                users.username,
                users.display_name,
                users.avatar_color,
                COUNT(DISTINCT image_likes.user_id) AS likes,
                COUNT(DISTINCT image_favorites.user_id) AS favorites,
                EXISTS(
                    SELECT 1 FROM image_likes mine
                    WHERE mine.image_id = images.id AND mine.user_id = ?
                ) AS liked_by_me,
                EXISTS(
                    SELECT 1 FROM image_favorites fav
                    WHERE fav.image_id = images.id AND fav.user_id = ?
                ) AS favorited_by_me
            FROM images
            JOIN users ON users.id = images.user_id
            LEFT JOIN image_likes ON image_likes.image_id = images.id
            LEFT JOIN image_favorites ON image_favorites.image_id = images.id
            WHERE images.user_id = ?
            GROUP BY images.id
            ORDER BY images.id DESC
            LIMIT ? OFFSET ?
            """,
            (user_id, user_id, user_id, limit, offset),
        ).fetchall()
    return [serialize_image(row) for row in rows]


def get_image(image_id: int, viewer_id: int | None) -> dict | None:
    with get_db() as db:
        row = db.execute(
            """
            SELECT
                images.*,
                users.username,
                users.display_name,
                users.avatar_color,
                COUNT(DISTINCT image_likes.user_id) AS likes,
                COUNT(DISTINCT image_favorites.user_id) AS favorites,
                EXISTS(
                    SELECT 1 FROM image_likes mine
                    WHERE mine.image_id = images.id AND mine.user_id = ?
                ) AS liked_by_me,
                EXISTS(
                    SELECT 1 FROM image_favorites fav
                    WHERE fav.image_id = images.id AND fav.user_id = ?
                ) AS favorited_by_me
            FROM images
            JOIN users ON users.id = images.user_id
            LEFT JOIN image_likes ON image_likes.image_id = images.id
            LEFT JOIN image_favorites ON image_favorites.image_id = images.id
            WHERE images.id = ?
            GROUP BY images.id
            """,
            (viewer_id or 0, viewer_id or 0, image_id),
        ).fetchone()
    return serialize_image(row) if row else None


def toggle_relation(table: str, image_id: int, user_id: int) -> bool:
    if table not in {"image_likes", "image_favorites"}:
        raise ValueError("Invalid relation table")
    with get_db() as db:
        existing = db.execute(
            f"SELECT 1 FROM {table} WHERE image_id = ? AND user_id = ?",
            (image_id, user_id),
        ).fetchone()
        if existing:
            db.execute(f"DELETE FROM {table} WHERE image_id = ? AND user_id = ?", (image_id, user_id))
            return False
        db.execute(f"INSERT INTO {table} (image_id, user_id) VALUES (?, ?)", (image_id, user_id))
        return True
