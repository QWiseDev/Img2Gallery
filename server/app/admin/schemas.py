from pydantic import BaseModel, Field


class AdminLoginPayload(BaseModel):
    password: str = Field(min_length=1, max_length=128)


class ConcurrencyPayload(BaseModel):
    concurrency: int = Field(ge=1, le=8)


class ProviderPayload(BaseModel):
    name: str = Field(min_length=2, max_length=60)
    provider_type: str = Field(default="openai_compatible", max_length=40)
    model: str = Field(min_length=2, max_length=80)
    api_base: str = Field(min_length=8, max_length=300)
    api_key: str | None = Field(default=None, max_length=500)
    enabled: bool = True
    is_default: bool = True


class UserAdminPayload(BaseModel):
    is_admin: bool
