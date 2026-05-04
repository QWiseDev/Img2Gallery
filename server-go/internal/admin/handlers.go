package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
)

type Handlers struct {
	service *Service
	repo    *Repository
}

type adminLoginPayload struct {
	Password string `json:"password"`
}

type concurrencyPayload struct {
	Concurrency int `json:"concurrency"`
}

type userAdminPayload struct {
	IsAdmin bool `json:"is_admin"`
}

type generationHiddenPayload struct {
	IsHidden bool `json:"is_hidden"`
}

func NewHandlers(service *Service, repo *Repository) *Handlers {
	return &Handlers{service: service, repo: repo}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/login", h.Login)
	mux.HandleFunc("POST /api/admin/logout", h.Logout)
	mux.HandleFunc("GET /api/admin/me", h.Me)
	mux.HandleFunc("GET /api/admin/dashboard", h.Dashboard)
	mux.HandleFunc("GET /api/admin/users", h.Users)
	mux.HandleFunc("PUT /api/admin/users/{id}/admin", h.SetUserAdmin)
	mux.HandleFunc("GET /api/admin/generations", h.Generations)
	mux.HandleFunc("DELETE /api/admin/generations/{id}", h.DeleteGeneration)
	mux.HandleFunc("PUT /api/admin/generations/{id}/hidden", h.SetGenerationHidden)
	mux.HandleFunc("GET /api/admin/providers", h.Providers)
	mux.HandleFunc("POST /api/admin/providers", h.SaveProvider)
	mux.HandleFunc("PUT /api/admin/providers/{id}", h.UpdateProvider)
	mux.HandleFunc("GET /api/admin/settings", h.Settings)
	mux.HandleFunc("PUT /api/admin/settings/concurrency", h.UpdateConcurrency)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var payload adminLoginPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	token, maxAge, err := h.service.Login(payload.Password)
	if err != nil {
		writeAdminError(w, err)
		return
	}
	httpx.SetCookie(w, CookieName, token, maxAge)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	cookie, _ := r.Cookie(CookieName)
	if cookie != nil {
		_ = h.service.Logout(cookie.Value)
	}
	httpx.ClearCookie(w, CookieName)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdmin(w, r)
	if ok {
		httpx.JSON(w, http.StatusOK, admin)
	}
}

func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	payload, err := h.repo.Dashboard()
	writeResult(w, payload, err)
}

func (h *Handlers) Users(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	payload, err := h.repo.UsersOverview()
	writeResult(w, payload, err)
}

func (h *Handlers) SetUserAdmin(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	var payload userAdminPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	updated, found, err := h.repo.SetUserAdmin(pathID(r), payload.IsAdmin)
	writeMaybeFound(w, updated, found, err, "用户不存在")
}

func (h *Handlers) Generations(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	payload, err := h.repo.GenerationRecords(120)
	writeResult(w, payload, err)
}

func (h *Handlers) DeleteGeneration(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	payload, found, err := h.repo.DeleteGeneration(pathID(r))
	writeMaybeFound(w, payload, found, err, "作品不存在")
}

func (h *Handlers) SetGenerationHidden(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	var payload generationHiddenPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	updated, found, err := h.repo.SetGenerationHidden(pathID(r), payload.IsHidden)
	writeMaybeFound(w, updated, found, err, "作品不存在")
}

func (h *Handlers) Providers(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	payload, err := h.repo.ListProviders()
	writeResult(w, payload, err)
}

func (h *Handlers) SaveProvider(w http.ResponseWriter, r *http.Request) {
	h.saveProvider(w, r, 0)
}

func (h *Handlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	h.saveProvider(w, r, pathID(r))
}

func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	if h.require(w, r) {
		httpx.JSON(w, http.StatusOK, map[string]int{"concurrency": h.repo.GetConcurrency()})
	}
}

func (h *Handlers) UpdateConcurrency(w http.ResponseWriter, r *http.Request) {
	if !h.require(w, r) {
		return
	}
	var payload concurrencyPayload
	if !httpx.DecodeJSON(r, &payload) || payload.Concurrency < 1 || payload.Concurrency > 8 {
		httpx.Error(w, http.StatusBadRequest, "并发数范围为 1-8")
		return
	}
	err := h.repo.SetSetting("generation_concurrency", strconv.Itoa(payload.Concurrency))
	writeResult(w, map[string]int{"concurrency": payload.Concurrency}, err)
}

func (h *Handlers) saveProvider(w http.ResponseWriter, r *http.Request, id int) {
	if !h.require(w, r) {
		return
	}
	var payload ProviderPayload
	if !httpx.DecodeJSON(r, &payload) || !validProvider(payload) {
		httpx.Error(w, http.StatusBadRequest, "模型提供商配置不完整")
		return
	}
	payload.ID = id
	result, err := h.repo.UpsertProvider(payload)
	writeResult(w, result, err)
}

func (h *Handlers) require(w http.ResponseWriter, r *http.Request) bool {
	_, ok := h.requireAdmin(w, r)
	return ok
}

func (h *Handlers) requireAdmin(w http.ResponseWriter, r *http.Request) (AdminIdentity, bool) {
	admin, err := h.service.Require(r)
	if err != nil {
		writeAdminError(w, err)
		return AdminIdentity{}, false
	}
	return admin, true
}

func validProvider(payload ProviderPayload) bool {
	return len(strings.TrimSpace(payload.Name)) >= 2 &&
		len(strings.TrimSpace(payload.ProviderType)) > 0 &&
		len(strings.TrimSpace(payload.Model)) >= 2
}

func writeAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrBadAdminPassword):
		httpx.Error(w, http.StatusUnauthorized, "管理员密码错误")
	case errors.Is(err, ErrAdminLoginRequired):
		httpx.Error(w, http.StatusUnauthorized, "请先登录管理员")
	case errors.Is(err, ErrAdminSessionExpired):
		httpx.Error(w, http.StatusUnauthorized, "管理员登录已过期")
	default:
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
	}
}

func pathID(r *http.Request) int {
	id, _ := strconv.Atoi(r.PathValue("id"))
	return id
}

func writeResult(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	httpx.JSON(w, http.StatusOK, payload)
}

func writeMaybeFound(w http.ResponseWriter, payload any, found bool, err error, detail string) {
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	if !found {
		httpx.Error(w, http.StatusNotFound, detail)
		return
	}
	httpx.JSON(w, http.StatusOK, payload)
}
