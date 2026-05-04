package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr            string
	AppSecret       string
	AdminPassword   string
	DatabasePath    string
	ImageStorageDir string
	ClientOrigin    string
	AppTimezone     string
	FrontendDist    string
}

func Load() Config {
	root := projectRoot()
	cfg := Config{
		Addr:            env("ADDR", "0.0.0.0:8000"),
		AppSecret:       env("APP_SECRET", "dev-secret-change-me"),
		AdminPassword:   env("ADMIN_PASSWORD", "admin123456"),
		DatabasePath:    env("DATABASE_PATH", "server/app.db"),
		ImageStorageDir: env("IMAGE_STORAGE_DIR", "server/storage/images"),
		ClientOrigin:    env("CLIENT_ORIGIN", "http://localhost:5173"),
		AppTimezone:     env("APP_TIMEZONE", "Asia/Shanghai"),
	}
	cfg.DatabasePath = resolvePath(root, cfg.DatabasePath)
	cfg.ImageStorageDir = resolvePath(root, cfg.ImageStorageDir)
	cfg.FrontendDist = filepath.Join(root, "client", "dist")
	return cfg
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func projectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if fileExists(filepath.Join(wd, "client", "package.json")) ||
			fileExists(filepath.Join(wd, "client", "dist", "index.html")) {
			return wd
		}
		next := filepath.Dir(wd)
		if next == wd {
			return wd
		}
		wd = next
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
