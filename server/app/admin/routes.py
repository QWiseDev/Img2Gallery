from fastapi import APIRouter, Depends, Request, Response

from .repository import (
    dashboard,
    delete_generation,
    generation_records,
    get_concurrency,
    list_providers,
    set_setting,
    set_user_admin,
    upsert_provider,
    users_overview,
)
from .schemas import AdminLoginPayload, ConcurrencyPayload, ProviderPayload, UserAdminPayload
from .service import login_admin, logout_admin, require_admin

router = APIRouter(prefix="/api/admin", tags=["admin"])


@router.post("/login")
def login(payload: AdminLoginPayload, response: Response):
    login_admin(payload.password, response)
    return {"ok": True}


@router.post("/logout")
def logout(request: Request, response: Response, admin: dict = Depends(require_admin)):
    logout_admin(request, response)
    return {"ok": True}


@router.get("/me")
def me(admin: dict = Depends(require_admin)):
    return admin


@router.get("/dashboard")
def get_dashboard(admin: dict = Depends(require_admin)):
    return dashboard()


@router.get("/users")
def get_users(admin: dict = Depends(require_admin)):
    return users_overview()


@router.put("/users/{user_id}/admin")
def update_user_admin(user_id: int, payload: UserAdminPayload, admin: dict = Depends(require_admin)):
    updated = set_user_admin(user_id, payload.is_admin)
    if not updated:
        from fastapi import HTTPException

        raise HTTPException(status_code=404, detail="用户不存在")
    return updated


@router.get("/generations")
def get_generations(admin: dict = Depends(require_admin)):
    return generation_records()


@router.delete("/generations/{image_id}")
def remove_generation(image_id: int, admin: dict = Depends(require_admin)):
    deleted = delete_generation(image_id)
    if not deleted:
        from fastapi import HTTPException

        raise HTTPException(status_code=404, detail="作品不存在")
    return deleted


@router.get("/providers")
def providers(admin: dict = Depends(require_admin)):
    return list_providers()


@router.post("/providers")
def save_provider(payload: ProviderPayload, admin: dict = Depends(require_admin)):
    return upsert_provider(payload.model_dump())


@router.put("/providers/{provider_id}")
def update_provider(provider_id: int, payload: ProviderPayload, admin: dict = Depends(require_admin)):
    data = payload.model_dump()
    data["id"] = provider_id
    return upsert_provider(data)


@router.get("/settings")
def settings(admin: dict = Depends(require_admin)):
    return {"concurrency": get_concurrency()}


@router.put("/settings/concurrency")
def update_concurrency(payload: ConcurrencyPayload, admin: dict = Depends(require_admin)):
    set_setting("generation_concurrency", str(payload.concurrency))
    return {"concurrency": payload.concurrency}
