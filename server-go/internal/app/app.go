package app

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/admin"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/auth"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/db"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
)

func New(cfg config.Config) (http.Handler, *sql.DB, error) {
	database, err := db.Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Init(database, cfg); err != nil {
		_ = database.Close()
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	authService := auth.NewService(database, cfg)
	authHandlers := auth.NewHandlers(authService, auth.NewCaptchaStore())
	adminService := admin.NewService(database, cfg, authService)
	adminHandlers := admin.NewHandlers(adminService, admin.NewRepository(database, cfg))
	authHandlers.Register(mux)
	adminHandlers.Register(mux)
	return withCORS(mux, cfg), database, nil
}

func withCORS(next http.Handler, cfg config.Config) http.Handler {
	allowed := map[string]bool{cfg.ClientOrigin: true, "http://127.0.0.1:5173": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] || originAllowedForSelf(origin, cfg) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowedForSelf(origin string, cfg config.Config) bool {
	return origin != "" && strings.Contains(origin, hostFromAddr(cfg.Addr))
}

func hostFromAddr(addr string) string {
	addr = strings.Replace(addr, "0.0.0.0", "localhost", 1)
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return strings.Split(addr, ":")[0]
}

func EnsureStorage(cfg config.Config) error {
	return os.MkdirAll(cfg.ImageStorageDir, 0o755)
}
