package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/auth"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
)

const (
	CookieName        = "gallery_admin_session"
	adminSessionHours = 12
)

type Service struct {
	db   *sql.DB
	cfg  config.Config
	auth *auth.Service
}

func NewService(database *sql.DB, cfg config.Config, authService *auth.Service) *Service {
	return &Service{db: database, cfg: cfg, auth: authService}
}

func (s *Service) Login(password string) (string, int, error) {
	if !hmac.Equal([]byte(password), []byte(s.cfg.AdminPassword)) {
		return "", 0, ErrBadAdminPassword
	}
	token := token(32)
	expires := time.Now().UTC().Add(adminSessionHours * time.Hour)
	_, err := s.db.Exec("INSERT INTO admin_sessions (token, expires_at) VALUES (?, ?)", token, utcISO(expires))
	return token, adminSessionHours * 3600, err
}

func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM admin_sessions WHERE token = ?", token)
	return err
}

func (s *Service) Require(r *http.Request) (map[string]any, error) {
	if adminUser, ok := s.adminFromUser(r); ok {
		return adminUser, nil
	}
	cookie, _ := r.Cookie(CookieName)
	if cookie == nil {
		return nil, ErrAdminLoginRequired
	}
	var stored string
	err := s.db.QueryRow("SELECT token FROM admin_sessions WHERE token = ? AND expires_at > ?", cookie.Value, utcISO(time.Now().UTC())).Scan(&stored)
	if err != nil {
		return nil, ErrAdminSessionExpired
	}
	return map[string]any{"role": "admin", "source": "password"}, nil
}

func (s *Service) adminFromUser(r *http.Request) (map[string]any, bool) {
	cookie, _ := r.Cookie(auth.SessionCookie)
	if cookie == nil {
		return nil, false
	}
	user, err := s.auth.CurrentUser(cookie.Value)
	if err != nil || !user.IsAdmin {
		return nil, false
	}
	return map[string]any{
		"role": "admin", "source": "user", "user_id": user.ID, "username": user.Username,
	}, true
}

func token(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func utcISO(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000+00:00")
}

var (
	ErrBadAdminPassword    = errors.New("bad admin password")
	ErrAdminLoginRequired  = errors.New("admin login required")
	ErrAdminSessionExpired = errors.New("admin session expired")
)
