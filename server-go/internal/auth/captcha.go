package auth

import (
	"encoding/base64"
	"html"
	"strings"
	"sync"
	"time"
)

const (
	captchaTTL   = 5 * time.Minute
	captchaChars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

type CaptchaStore struct {
	mu    sync.Mutex
	items map[string]captchaItem
}

type captchaItem struct {
	code      string
	expiresAt time.Time
}

func NewCaptchaStore() *CaptchaStore {
	return &CaptchaStore{items: map[string]captchaItem{}}
}

func (s *CaptchaStore) Create() map[string]string {
	s.cleanup()
	token := randomToken(24)
	code := randomCode(5)
	s.mu.Lock()
	s.items[token] = captchaItem{code: code, expiresAt: time.Now().Add(captchaTTL)}
	s.mu.Unlock()
	encoded := base64.StdEncoding.EncodeToString([]byte(renderSVG(code)))
	return map[string]string{"token": token, "image": "data:image/svg+xml;base64," + encoded}
}

func (s *CaptchaStore) Verify(token, answer string) string {
	normalized := strings.ToUpper(strings.TrimSpace(answer))
	s.mu.Lock()
	item, ok := s.items[token]
	delete(s.items, token)
	s.mu.Unlock()
	if !ok || item.expiresAt.Before(time.Now()) {
		return "验证码已过期，请刷新"
	}
	if normalized != item.code {
		return "验证码错误"
	}
	return ""
}

func (s *CaptchaStore) cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, item := range s.items {
		if item.expiresAt.Before(now) {
			delete(s.items, token)
		}
	}
}

func randomCode(length int) string {
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteByte(captchaChars[randomInt(len(captchaChars))])
	}
	return b.String()
}

func renderSVG(code string) string {
	escaped := html.EscapeString(code)
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="160" height="56" viewBox="0 0 160 56">`)
	b.WriteString(`<rect width="160" height="56" rx="14" fill="#fff7ed"/>`)
	b.WriteString(`<rect x="0.5" y="0.5" width="159" height="55" rx="13.5" fill="none" stroke="#fed7aa"/>`)
	for i := 0; i < 5; i++ {
		color := []string{"#fb923c", "#14b8a6", "#64748b", "#f59e0b"}[randomInt(4)]
		b.WriteString(`<line x1="` + itoa(randomInt(160)) + `" y1="` + itoa(randomInt(56)) + `" x2="` + itoa(randomInt(160)) + `" y2="` + itoa(randomInt(56)) + `" stroke="` + color + `" stroke-width="1.4" stroke-opacity="0.34" />`)
	}
	for i := 0; i < 26; i++ {
		b.WriteString(`<circle cx="` + itoa(randomInt(160)) + `" cy="` + itoa(randomInt(56)) + `" r="1.2" fill="#94a3b8" fill-opacity="0.34" />`)
	}
	for i, char := range escaped {
		x := 24 + i*24
		y := 35 + randomInt(7) - 3
		rotate := randomInt(18) - 9
		b.WriteString(`<text x="` + itoa(x) + `" y="` + itoa(y) + `" transform="rotate(` + itoa(rotate) + ` ` + itoa(x) + ` ` + itoa(y) + `)" font-size="25" font-weight="900" font-family="Arial, sans-serif" fill="#182136">` + string(char) + `</text>`)
	}
	b.WriteString(`</svg>`)
	return b.String()
}
