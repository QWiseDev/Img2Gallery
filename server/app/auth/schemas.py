from pydantic import BaseModel, Field


class AuthPayload(BaseModel):
    username: str = Field(min_length=3, max_length=32)
    password: str = Field(min_length=6, max_length=128)
    display_name: str | None = Field(default=None, max_length=40)
    captcha_token: str = Field(min_length=16)
    captcha_code: str = Field(min_length=4, max_length=8)


class LoginPayload(BaseModel):
    username: str = Field(min_length=3, max_length=32)
    password: str = Field(min_length=6, max_length=128)
    captcha_token: str = Field(min_length=16)
    captcha_code: str = Field(min_length=4, max_length=8)


class CaptchaOut(BaseModel):
    token: str
    image: str


class UserOut(BaseModel):
    id: int
    username: str
    display_name: str
    avatar_color: str
    is_admin: bool = False
