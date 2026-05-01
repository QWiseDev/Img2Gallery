package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
)

type Handlers struct {
	auth    *Service
	captcha *CaptchaStore
}

type authPayload struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	DisplayName  string `json:"display_name"`
	CaptchaToken string `json:"captcha_token"`
	CaptchaCode  string `json:"captcha_code"`
}

type loginPayload struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token"`
	CaptchaCode  string `json:"captcha_code"`
}

func NewHandlers(service *Service, captcha *CaptchaStore) *Handlers {
	return &Handlers{auth: service, captcha: captcha}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/captcha", h.Captcha)
	mux.HandleFunc("POST /api/auth/register", h.RegisterUser)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/me", h.Me)
}

func (h *Handlers) Captcha(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, h.captcha.Create())
}

func (h *Handlers) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var payload authPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateCaptcha(h.captcha, payload.CaptchaToken, payload.CaptchaCode); msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	user, err := h.auth.CreateUser(payload.Username, payload.Password, payload.DisplayName)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	_ = h.auth.RecordLogin(user.ID, httpx.ClientIP(r))
	h.setSession(w, user.ID)
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateCaptcha(h.captcha, payload.CaptchaToken, payload.CaptchaCode); msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	user, err := h.auth.Authenticate(payload.Username, payload.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	_ = h.auth.RecordLogin(user.ID, httpx.ClientIP(r))
	h.setSession(w, user.ID)
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		_ = h.auth.ClearSession(cookie.Value)
	}
	httpx.ClearCookie(w, SessionCookie)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.CurrentUser(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handlers) CurrentUser(r *http.Request) (User, error) {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie == nil {
		return User{}, ErrLoginRequired
	}
	return h.auth.CurrentUser(cookie.Value)
}

func (h *Handlers) setSession(w http.ResponseWriter, userID int) {
	token, maxAge, err := h.auth.IssueSession(userID)
	if err == nil {
		httpx.SetCookie(w, SessionCookie, token, maxAge)
	}
}

func validateCaptcha(store *CaptchaStore, token, code string) string {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(code) == "" {
		return "验证码错误"
	}
	return store.Verify(token, code)
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUserExists):
		httpx.Error(w, http.StatusConflict, "用户名已存在")
	case errors.Is(err, ErrBadCredentials):
		httpx.Error(w, http.StatusUnauthorized, "用户名或密码错误")
	case errors.Is(err, ErrLoginRequired):
		httpx.Error(w, http.StatusUnauthorized, "请先登录")
	case errors.Is(err, ErrSessionExpired):
		httpx.Error(w, http.StatusUnauthorized, "登录已过期")
	default:
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
	}
}
