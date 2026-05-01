package static

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
)

func Register(mux *http.ServeMux, cfg config.Config) {
	media := http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.ImageStorageDir)))
	mux.Handle("GET /media/", media)
	assetsDir := filepath.Join(cfg.FrontendDist, "assets")
	if exists(assetsDir) {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	}
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		serveSPA(w, r, cfg)
	})
}

func serveSPA(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean == "." {
		clean = "index.html"
	}
	requested := filepath.Join(cfg.FrontendDist, clean)
	if isInside(cfg.FrontendDist, requested) && isFile(requested) {
		http.ServeFile(w, r, requested)
		return
	}
	index := filepath.Join(cfg.FrontendDist, "index.html")
	if isFile(index) {
		http.ServeFile(w, r, index)
		return
	}
	httpx.Error(w, http.StatusNotFound, "Frontend has not been built")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isInside(root, target string) bool {
	absRoot, _ := filepath.Abs(root)
	absTarget, _ := filepath.Abs(target)
	if absRoot == absTarget {
		return true
	}
	prefix := strings.TrimRight(absRoot, string(os.PathSeparator)) + string(os.PathSeparator)
	return strings.HasPrefix(absTarget, prefix)
}
