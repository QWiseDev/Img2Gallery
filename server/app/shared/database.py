import sqlite3
from contextlib import contextmanager
from typing import Iterator

from .config import get_settings


def connect() -> sqlite3.Connection:
    settings = get_settings()
    settings.db_file.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(settings.db_file)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


@contextmanager
def get_db() -> Iterator[sqlite3.Connection]:
    conn = connect()
    try:
        yield conn
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


def init_db() -> None:
    with get_db() as db:
        db.executescript(
            """
            CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                username TEXT NOT NULL UNIQUE,
                display_name TEXT NOT NULL,
                password_hash TEXT NOT NULL,
                avatar_color TEXT NOT NULL,
                is_admin INTEGER NOT NULL DEFAULT 0,
                last_login_ip TEXT,
                last_login_at TEXT,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS sessions (
                token TEXT PRIMARY KEY,
                user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                expires_at TEXT NOT NULL,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS images (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                prompt TEXT NOT NULL,
                image_path TEXT,
                task_type TEXT NOT NULL DEFAULT 'generate',
                source_image_path TEXT,
                status TEXT NOT NULL CHECK(status IN ('queued', 'running', 'ready', 'failed')),
                error TEXT,
                request_ip TEXT,
                provider_name TEXT,
                model TEXT,
                queued_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
                started_at TEXT,
                completed_at TEXT,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS image_likes (
                image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
                user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (image_id, user_id)
            );

            CREATE TABLE IF NOT EXISTS image_favorites (
                image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
                user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (image_id, user_id)
            );

            CREATE TABLE IF NOT EXISTS admin_sessions (
                token TEXT PRIMARY KEY,
                expires_at TEXT NOT NULL,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS app_settings (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL,
                updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS model_providers (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL UNIQUE,
                provider_type TEXT NOT NULL DEFAULT 'openai_compatible',
                model TEXT NOT NULL,
                api_base TEXT NOT NULL,
                api_key TEXT NOT NULL DEFAULT '',
                enabled INTEGER NOT NULL DEFAULT 1,
                is_default INTEGER NOT NULL DEFAULT 0,
                created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """
        )
        migrate_schema(db)
        seed_defaults(db)


def migrate_schema(db: sqlite3.Connection) -> None:
    ensure_column(db, "users", "is_admin", "INTEGER NOT NULL DEFAULT 0")
    ensure_column(db, "users", "last_login_ip", "TEXT")
    ensure_column(db, "users", "last_login_at", "TEXT")
    migrate_images_status(db)
    image_columns = {
        "request_ip": "TEXT",
        "provider_name": "TEXT",
        "model": "TEXT",
        "task_type": "TEXT NOT NULL DEFAULT 'generate'",
        "source_image_path": "TEXT",
        "queued_at": "TEXT DEFAULT CURRENT_TIMESTAMP",
        "started_at": "TEXT",
        "completed_at": "TEXT",
    }
    for column, definition in image_columns.items():
        ensure_column(db, "images", column, definition)


def ensure_column(db: sqlite3.Connection, table: str, column: str, definition: str) -> None:
    columns = [row["name"] for row in db.execute(f"PRAGMA table_info({table})").fetchall()]
    if column not in columns:
        db.execute(f"ALTER TABLE {table} ADD COLUMN {column} {definition}")


def migrate_images_status(db: sqlite3.Connection) -> None:
    row = db.execute(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'images'"
    ).fetchone()
    if not row or "queued" in row["sql"]:
        return
    db.executescript(
        """
        ALTER TABLE images RENAME TO images_old;
        CREATE TABLE images (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            prompt TEXT NOT NULL,
            image_path TEXT,
            task_type TEXT NOT NULL DEFAULT 'generate',
            source_image_path TEXT,
            status TEXT NOT NULL CHECK(status IN ('queued', 'running', 'ready', 'failed')),
            error TEXT,
            request_ip TEXT,
            provider_name TEXT,
            model TEXT,
            queued_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            started_at TEXT,
            completed_at TEXT,
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        INSERT INTO images (
            id, user_id, prompt, image_path, task_type, source_image_path, status, error, created_at, queued_at, completed_at
        )
        SELECT id, user_id, prompt, image_path, 'generate', NULL, status, error, created_at, created_at, created_at
        FROM images_old;
        DROP TABLE images_old;
        """
    )


def seed_defaults(db: sqlite3.Connection) -> None:
    db.execute(
        """
        INSERT OR IGNORE INTO app_settings (key, value)
        VALUES ('generation_concurrency', '1')
        """
    )
    db.execute(
        """
        INSERT OR IGNORE INTO model_providers (
            name, provider_type, model, api_base, api_key, enabled, is_default
        )
        VALUES ('GPT Image 2', 'openai_compatible', 'gpt-image-2', '', '', 1, 1)
        """
    )
