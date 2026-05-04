package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
)

func TestRoutesSmoke(t *testing.T) {
	handler, cleanup := newTestApp(t)
	defer cleanup()

	assertJSONStatus(t, handler, http.MethodGet, "/health", http.StatusOK, `"status":"ok"`)
	assertJSONStatus(t, handler, http.MethodGet, "/api/images?sort=latest&offset=0&limit=24", http.StatusOK, "[]")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("captcha status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var captcha map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &captcha); err != nil {
		t.Fatalf("captcha JSON decode returned error: %v", err)
	}
	if captcha["token"] == "" {
		t.Fatalf("captcha token was empty")
	}
	if !strings.HasPrefix(captcha["image"], "data:image/svg+xml;base64,") {
		t.Fatalf("captcha image did not use svg data URL: %q", captcha["image"])
	}
}

func TestSPAFallbackReportsMissingBuild(t *testing.T) {
	handler, cleanup := newTestApp(t)
	defer cleanup()

	assertJSONStatus(t, handler, http.MethodGet, "/admin", http.StatusNotFound, "Frontend has not been built")
}

func TestCORSAllowsConfiguredAndSelfOrigins(t *testing.T) {
	cfg := config.Config{
		Addr:         "0.0.0.0:8000",
		ClientOrigin: "http://localhost:5173",
	}
	handler := withCORS(okHandler(), cfg)

	assertCORSOrigin(t, handler, "http://localhost:5173", "http://localhost:5173")
	assertCORSOrigin(t, handler, "http://127.0.0.1:5173", "http://127.0.0.1:5173")
	assertCORSOrigin(t, handler, "http://localhost:8000", "http://localhost:8000")
}

func TestCORSRejectsLookalikeSelfOrigins(t *testing.T) {
	cfg := config.Config{
		Addr:         "0.0.0.0:8000",
		ClientOrigin: "http://localhost:5173",
	}
	handler := withCORS(okHandler(), cfg)

	assertCORSOrigin(t, handler, "http://localhost.attacker.test:8000", "")
	assertCORSOrigin(t, handler, "http://evil-localhost.test:8000", "")

	cfg.Addr = "127.0.0.1:8000"
	handler = withCORS(okHandler(), cfg)
	assertCORSOrigin(t, handler, "http://127.0.0.1.attacker.test:8000", "")
}

func assertJSONStatus(t *testing.T, handler http.Handler, method, path string, status int, bodyPart string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != status {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, rec.Code, status, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), bodyPart) {
		t.Fatalf("%s %s body = %s, want to contain %q", method, path, rec.Body.String(), bodyPart)
	}
}

func assertCORSOrigin(t *testing.T, handler http.Handler, origin, want string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
		t.Fatalf("Access-Control-Allow-Origin for %q = %q, want %q", origin, got, want)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func newTestApp(t *testing.T) (http.Handler, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Config{
		Addr:            "127.0.0.1:0",
		AppSecret:       "test-secret",
		AdminPassword:   "admin-password",
		DatabasePath:    filepath.Join(root, "app.db"),
		ImageStorageDir: filepath.Join(root, "images"),
		ClientOrigin:    "http://127.0.0.1:5173",
		AppTimezone:     "Asia/Shanghai",
		FrontendDist:    filepath.Join(root, "client", "dist"),
	}
	if err := EnsureStorage(cfg); err != nil {
		t.Fatalf("EnsureStorage returned error: %v", err)
	}
	handler, database, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return handler, func() { _ = database.Close() }
}
