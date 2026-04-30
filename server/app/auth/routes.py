from fastapi import APIRouter, Depends, Request, Response

from .schemas import AuthPayload, LoginPayload, UserOut
from .service import (
    authenticate,
    clear_session,
    create_user,
    current_user,
    issue_session,
    record_login,
)

router = APIRouter(prefix="/api/auth", tags=["auth"])


def client_ip(request: Request) -> str | None:
    forwarded = request.headers.get("x-forwarded-for")
    if forwarded:
        return forwarded.split(",", 1)[0].strip()
    return request.client.host if request.client else None


@router.post("/register", response_model=UserOut)
def register(payload: AuthPayload, request: Request, response: Response):
    user = create_user(payload.username, payload.password, payload.display_name)
    record_login(user["id"], client_ip(request))
    issue_session(response, user["id"])
    return user


@router.post("/login", response_model=UserOut)
def login(payload: LoginPayload, request: Request, response: Response):
    user = authenticate(payload.username, payload.password)
    record_login(user["id"], client_ip(request))
    issue_session(response, user["id"])
    return user


@router.post("/logout")
def logout(request: Request, response: Response):
    clear_session(request, response)
    return {"ok": True}


@router.get("/me", response_model=UserOut)
def me(user: dict = Depends(current_user)):
    return user
