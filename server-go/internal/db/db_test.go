package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
)

func TestInitCreatesExpectedTables(t *testing.T) {
	cfg := config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "app.db"),
		AppTimezone:  "Asia/Shanghai",
	}
	database, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer database.Close()

	if err := Init(database, cfg); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	for _, table := range []string{
		"users",
		"sessions",
		"images",
		"image_params",
		"image_likes",
		"image_favorites",
		"admin_sessions",
		"app_settings",
		"model_providers",
	} {
		assertTableExists(t, database, table)
	}
}

func TestUpgradeBackfillsImageParamsAndClearsSessions(t *testing.T) {
	cfg := config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "app.db"),
		AppTimezone:  "Asia/Shanghai",
	}
	database, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			avatar_color TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			last_login_ip TEXT,
			last_login_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			prompt TEXT NOT NULL,
			image_path TEXT,
			task_type TEXT NOT NULL DEFAULT 'generate',
			source_image_path TEXT,
			is_hidden INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL CHECK(status IN ('queued', 'running', 'ready', 'failed')),
			error TEXT,
			request_ip TEXT,
			provider_name TEXT,
			model TEXT,
			queued_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE admin_sessions (
			token TEXT PRIMARY KEY,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO users (username, display_name, password_hash, avatar_color)
		VALUES ('alice', 'Alice', 'salt$hash', '#14b8a6');
		INSERT INTO images (user_id, prompt, status) VALUES (1, 'old image', 'ready');
		INSERT INTO sessions (token, user_id, expires_at) VALUES ('old-user-session', 1, '2999-01-01T00:00:00.000000+00:00');
		INSERT INTO admin_sessions (token, expires_at) VALUES ('old-admin-session', '2999-01-01T00:00:00.000000+00:00');
	`); err != nil {
		t.Fatalf("seed legacy schema returned error: %v", err)
	}

	if err := Upgrade(database, cfg); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}

	var paramRows int
	if err := database.QueryRow("SELECT COUNT(*) FROM image_params WHERE image_id = 1").Scan(&paramRows); err != nil {
		t.Fatalf("count image_params returned error: %v", err)
	}
	if paramRows != 1 {
		t.Fatalf("image_params rows = %d, want 1", paramRows)
	}
	assertTableCount(t, database, "sessions", 0)
	assertTableCount(t, database, "admin_sessions", 0)

	var version int
	if err := database.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("PRAGMA user_version returned error: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, CurrentSchemaVersion)
	}
}

func TestInitRequiresUpgradeForExistingUnversionedDatabase(t *testing.T) {
	cfg := config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "app.db"),
		AppTimezone:  "Asia/Shanghai",
	}
	database, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec("CREATE TABLE legacy_marker (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("seed existing database returned error: %v", err)
	}

	err = Init(database, cfg)
	if err == nil {
		t.Fatalf("Init returned nil error, want upgrade-required error")
	}
	if !strings.Contains(err.Error(), "db-upgrade") {
		t.Fatalf("Init error = %q, want db-upgrade guidance", err.Error())
	}
}

func assertTableExists(t *testing.T, database *sql.DB, table string) {
	t.Helper()
	var name string
	err := database.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&name)
	if err != nil {
		t.Fatalf("expected table %q to exist: %v", table, err)
	}
}

func assertTableCount(t *testing.T, database *sql.DB, table string, want int) {
	t.Helper()
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	if err != nil {
		t.Fatalf("count %s returned error: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
