package auth

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/db"
)

func TestHashPasswordUsesSaltDigestFormat(t *testing.T) {
	hash, err := HashPassword("secret", "abcd")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 2 {
		t.Fatalf("expected salt$digest format, got %q", hash)
	}
	if parts[0] != "abcd" {
		t.Fatalf("salt = %q, want abcd", parts[0])
	}
	if len(parts[1]) != 64 {
		t.Fatalf("digest length = %d, want 64", len(parts[1]))
	}
}

func TestVerifyPasswordAcceptsOnlyMatchingPassword(t *testing.T) {
	hash, err := HashPassword("secret", "abcd")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !VerifyPassword("secret", hash) {
		t.Fatalf("VerifyPassword rejected matching password")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatalf("VerifyPassword accepted wrong password")
	}
}

func TestCreateUserNormalizesUsername(t *testing.T) {
	service := newTestService(t)

	user, err := service.CreateUser("  Alice  ", "secret", "")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if user.Username != "alice" {
		t.Fatalf("username = %q, want alice", user.Username)
	}
	if user.DisplayName != "Alice" {
		t.Fatalf("display name = %q, want Alice", user.DisplayName)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	cfg := config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "app.db"),
		AppTimezone:  "Asia/Shanghai",
	}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Init(database, cfg); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	return NewService(database, cfg)
}
