package app

import (
	"database/sql"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/admin"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/auth"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/db"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/images"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/static"
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
	imagesRepo := images.NewRepository(database, cfg)
	imageQueue := images.NewQueue(imagesRepo, images.NewProviderClient(cfg))
	imageQueue.Start()
	adminService := admin.NewService(database, cfg, authService)
	adminHandlers := admin.NewHandlers(adminService, admin.NewRepository(database, cfg))
	authHandlers.Register(mux)
	images.NewHandlers(imagesRepo, authHandlers, cfg, imageQueue).Register(mux)
	adminHandlers.Register(mux)
	static.Register(mux, cfg)
	return withCORS(mux, cfg), database, nil
}

func withCORS(next http.Handler, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if originAllowed(origin, cfg) {
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

func originAllowed(origin string, cfg config.Config) bool {
	if origin == "" {
		return false
	}
	if origin == cfg.ClientOrigin || origin == "http://127.0.0.1:5173" {
		return true
	}
	return originAllowedForSelf(origin, cfg)
}

func originAllowedForSelf(origin string, cfg config.Config) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	listenerHost, listenerPort := listenerHostPort(cfg.Addr)
	if listenerPort != "" && originPort(parsed) != listenerPort {
		return false
	}
	if isWildcardHost(listenerHost) {
		return isLoopbackHost(parsed.Hostname())
	}
	return sameHost(listenerHost, parsed.Hostname())
}

func listenerHostPort(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		return normalizeHost(host), port
	}
	if strings.HasPrefix(addr, ":") {
		return "", strings.TrimPrefix(addr, ":")
	}
	return normalizeHost(addr), ""
}

func normalizeHost(host string) string {
	host = strings.Trim(host, "[]")
	if isWildcardHost(host) {
		return ""
	}
	return host
}

func originPort(parsed *url.URL) string {
	if parsed.Port() != "" {
		return parsed.Port()
	}
	switch parsed.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func isWildcardHost(host string) bool {
	return host == "" || host == "0.0.0.0" || host == "::"
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sameHost(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	leftIP := net.ParseIP(left)
	rightIP := net.ParseIP(right)
	return leftIP != nil && rightIP != nil && leftIP.Equal(rightIP)
}

func EnsureStorage(cfg config.Config) error {
	return os.MkdirAll(cfg.ImageStorageDir, 0o755)
}
