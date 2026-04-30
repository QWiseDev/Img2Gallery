from functools import lru_cache
from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    app_secret: str = "dev-secret-change-me"
    admin_password: str = "admin123456"
    database_path: str = "server/app.db"
    image_storage_dir: str = "server/storage/images"
    client_origin: str = "http://localhost:5173"

    model_config = SettingsConfigDict(
        env_file=(".env", "../.env"),
        env_file_encoding="utf-8",
        extra="ignore",
    )

    @property
    def project_root(self) -> Path:
        return Path(__file__).resolve().parents[3]

    @property
    def db_file(self) -> Path:
        return self._resolve_path(self.database_path)

    @property
    def storage_dir(self) -> Path:
        return self._resolve_path(self.image_storage_dir)

    def _resolve_path(self, value: str) -> Path:
        path = Path(value)
        if path.is_absolute():
            return path
        return self.project_root / path


@lru_cache
def get_settings() -> Settings:
    return Settings()
