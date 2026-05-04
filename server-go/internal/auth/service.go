package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/timeutil"
	"golang.org/x/crypto/pbkdf2"
)

const (
	SessionCookie = "gallery_session"
	sessionDays   = 14
)

var avatarColors = []string{"#f97316", "#14b8a6", "#6366f1", "#e11d48", "#0f766e"}

type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarColor string `json:"avatar_color"`
	IsAdmin     bool   `json:"is_admin"`
}

type Service struct {
	db  *sql.DB
	cfg config.Config
}

func NewService(database *sql.DB, cfg config.Config) *Service {
	return &Service{db: database, cfg: cfg}
}

func (s *Service) CreateUser(username, password, displayName string) (User, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	shown := strings.TrimSpace(displayName)
	if shown == "" {
		shown = strings.TrimSpace(username)
	}
	hash, err := HashPassword(password, "")
	if err != nil {
		return User{}, err
	}
	color := avatarColors[avatarIndex(normalized)%len(avatarColors)]
	res, err := s.db.Exec(`
		INSERT INTO users (username, display_name, password_hash, avatar_color, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, normalized, shown, hash, color, timeutil.LocalTimestamp(s.cfg.AppTimezone))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return s.UserByID(int(id))
}

func (s *Service) Authenticate(username, password string) (User, error) {
	row := s.db.QueryRow("SELECT id, username, display_name, password_hash, avatar_color, is_admin FROM users WHERE username = ?", strings.ToLower(strings.TrimSpace(username)))
	var user User
	var hash string
	var isAdmin int
	err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &hash, &user.AvatarColor, &isAdmin)
	if err != nil || !VerifyPassword(password, hash) {
		return User{}, ErrBadCredentials
	}
	user.IsAdmin = isAdmin == 1
	return user, nil
}

func (s *Service) UserByID(id int) (User, error) {
	row := s.db.QueryRow("SELECT id, username, display_name, avatar_color, is_admin FROM users WHERE id = ?", id)
	var user User
	var isAdmin int
	if err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.AvatarColor, &isAdmin); err != nil {
		return User{}, err
	}
	user.IsAdmin = isAdmin == 1
	return user, nil
}

func (s *Service) RecordLogin(userID int, ip string) error {
	_, err := s.db.Exec("UPDATE users SET last_login_ip = ?, last_login_at = ? WHERE id = ?", ip, timeutil.LocalTimestamp(s.cfg.AppTimezone), userID)
	return err
}

func (s *Service) IssueSession(userID int) (string, int, error) {
	token := randomToken(32)
	expires := time.Now().UTC().Add(sessionDays * 24 * time.Hour)
	_, err := s.db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, utcISO(expires))
	return token, sessionDays * 24 * 3600, err
}

func (s *Service) ClearSession(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Service) CurrentUser(token string) (User, error) {
	if token == "" {
		return User{}, ErrLoginRequired
	}
	row := s.db.QueryRow(`
		SELECT users.id, users.username, users.display_name, users.avatar_color, users.is_admin
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token = ? AND sessions.expires_at > ?
	`, token, utcISO(time.Now().UTC()))
	var user User
	var isAdmin int
	err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.AvatarColor, &isAdmin)
	if err != nil {
		return User{}, ErrSessionExpired
	}
	user.IsAdmin = isAdmin == 1
	return user, nil
}

func HashPassword(password, salt string) (string, error) {
	if salt == "" {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		salt = hex.EncodeToString(bytes)
	}
	digest := pbkdf2.Key([]byte(password), []byte(salt), 120000, 32, sha256.New)
	return salt + "$" + hex.EncodeToString(digest), nil
}

func VerifyPassword(password, storedHash string) bool {
	parts := strings.SplitN(storedHash, "$", 2)
	if len(parts) != 2 {
		return false
	}
	hash, err := HashPassword(password, parts[0])
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(hash), []byte(storedHash))
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func randomInt(max int) int {
	n, err := rand.Int(rand.Reader, bigInt(max))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

func bigInt(v int) *big.Int {
	return big.NewInt(int64(v))
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func avatarIndex(username string) int {
	sum := 0
	for _, ch := range username {
		sum += int(ch)
	}
	return sum
}

func utcISO(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000+00:00")
}

var (
	ErrUserExists     = errors.New("user exists")
	ErrBadCredentials = errors.New("bad credentials")
	ErrLoginRequired  = errors.New("login required")
	ErrSessionExpired = errors.New("session expired")
)
