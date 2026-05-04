package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/timeutil"

	_ "modernc.org/sqlite"
)

const CurrentSchemaVersion = 1
const upgradeCommand = "go run ./cmd/db-upgrade"

type sqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func Open(cfg config.Config) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func Init(database *sql.DB, cfg config.Config) error {
	fresh, err := databaseIsEmpty(database)
	if err != nil {
		return err
	}
	if _, err := database.Exec(schemaSQL); err != nil {
		return err
	}
	if err := seedDefaults(database, cfg); err != nil {
		return err
	}
	if fresh {
		return setSchemaVersion(database)
	}
	return requireCurrentSchema(database)
}

func Upgrade(database *sql.DB, cfg config.Config) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schemaSQL); err != nil {
		return err
	}
	if err := backfillImageParams(tx); err != nil {
		return err
	}
	if err := clearLoginSessions(tx); err != nil {
		return err
	}
	if err := seedDefaults(tx, cfg); err != nil {
		return err
	}
	if err := setSchemaVersion(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func backfillImageParams(database sqlExecutor) error {
	_, err := database.Exec(`
		INSERT INTO image_params (image_id, size, quality, output_format, output_compression, moderation)
		SELECT images.id, 'auto', 'auto', 'png', NULL, 'auto'
		FROM images
		LEFT JOIN image_params ON image_params.image_id = images.id
		WHERE image_params.image_id IS NULL
	`)
	return err
}

func clearLoginSessions(database sqlExecutor) error {
	if _, err := database.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	_, err := database.Exec("DELETE FROM admin_sessions")
	return err
}

func seedDefaults(database sqlExecutor, cfg config.Config) error {
	now := timeutil.LocalTimestamp(cfg.AppTimezone)
	_, err := database.Exec(`
		INSERT OR IGNORE INTO app_settings (key, value, updated_at)
		VALUES ('generation_concurrency', '1', ?)
	`, now)
	if err != nil {
		return err
	}
	_, err = database.Exec(`
		INSERT OR IGNORE INTO model_providers (
			name, provider_type, model, api_base, api_key, enabled, is_default, created_at, updated_at
		)
		VALUES ('GPT Image 2', 'openai_compatible', 'gpt-image-2', '', '', 1, 1, ?, ?)
	`, now, now)
	return err
}

func setSchemaVersion(database sqlExecutor) error {
	_, err := database.Exec(fmt.Sprintf("PRAGMA user_version = %d", CurrentSchemaVersion))
	return err
}

func requireCurrentSchema(database *sql.DB) error {
	var version int
	if err := database.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != CurrentSchemaVersion {
		return fmt.Errorf("database schema upgrade required: run %s", upgradeCommand)
	}
	var missingParams int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM images
		LEFT JOIN image_params ON image_params.image_id = images.id
		WHERE image_params.image_id IS NULL
	`).Scan(&missingParams); err != nil {
		return err
	}
	if missingParams > 0 {
		return fmt.Errorf("database has %d images without params: run %s", missingParams, upgradeCommand)
	}
	return nil
}

func databaseIsEmpty(database *sql.DB) (bool, error) {
	var count int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&count)
	return count == 0, err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
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

CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS images (
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

CREATE TABLE IF NOT EXISTS image_params (
	image_id INTEGER PRIMARY KEY REFERENCES images(id) ON DELETE CASCADE,
	size TEXT NOT NULL DEFAULT 'auto',
	quality TEXT NOT NULL DEFAULT 'auto',
	output_format TEXT NOT NULL DEFAULT 'png',
	output_compression INTEGER,
	moderation TEXT NOT NULL DEFAULT 'auto'
);

CREATE TABLE IF NOT EXISTS image_likes (
	image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (image_id, user_id)
);

CREATE TABLE IF NOT EXISTS image_favorites (
	image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (image_id, user_id)
);

CREATE TABLE IF NOT EXISTS admin_sessions (
	token TEXT PRIMARY KEY,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS model_providers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	provider_type TEXT NOT NULL DEFAULT 'openai_compatible',
	model TEXT NOT NULL,
	api_base TEXT NOT NULL,
	api_key TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	is_default INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
