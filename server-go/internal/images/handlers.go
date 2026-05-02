package images

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/auth"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/httpx"
)

const (
	defaultPageSize     = 24
	maxPageSize         = 48
	maxSourceImageBytes = 10 * 1024 * 1024
)

type Handlers struct {
	repo *Repository
	auth *auth.Handlers
	cfg  config.Config
	q    *Queue
}

type createPayload struct {
	Prompt string           `json:"prompt"`
	Params GenerationParams `json:"params"`
}

func NewHandlers(repo *Repository, authHandlers *auth.Handlers, cfg config.Config, q *Queue) *Handlers {
	return &Handlers{repo: repo, auth: authHandlers, cfg: cfg, q: q}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/images", h.Gallery)
	mux.HandleFunc("GET /api/images/mine", h.Mine)
	mux.HandleFunc("POST /api/images", h.Create)
	mux.HandleFunc("POST /api/images/edit", h.Edit)
	mux.HandleFunc("GET /api/images/{id}/events", h.Events)
	mux.HandleFunc("POST /api/images/{id}/like", h.Like)
	mux.HandleFunc("POST /api/images/{id}/favorite", h.Favorite)
}

func (h *Handlers) Gallery(w http.ResponseWriter, r *http.Request) {
	viewerID := h.optionalUserID(r)
	sort := r.URL.Query().Get("sort")
	if sort != "popular" && sort != "favorites" {
		sort = "latest"
	}
	images, err := h.repo.ListImages(viewerID, sort, clampLimit(queryInt(r, "limit", defaultPageSize)), max(0, queryInt(r, "offset", 0)))
	writeImages(w, images, err)
}

func (h *Handlers) Mine(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	images, err := h.repo.ListUserImages(user.ID, clampLimit(queryInt(r, "limit", defaultPageSize)), max(0, queryInt(r, "offset", 0)))
	writeImages(w, images, err)
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	var payload createPayload
	if !httpx.DecodeJSON(r, &payload) {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(strings.TrimSpace(payload.Prompt)) < 2 {
		httpx.Error(w, http.StatusBadRequest, "提示词至少需要 2 个字符")
		return
	}
	id, err := h.repo.AddImage(user.ID, strings.TrimSpace(payload.Prompt), "queued", httpx.ClientIP(r), "generate", "", payload.Params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "图片记录创建失败")
		return
	}
	h.writeCreated(w, int(id), user.ID)
}

func (h *Handlers) Edit(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	prompt, sourcePath, params, err := h.parseEditRequest(r)
	if err != nil {
		httpx.Error(w, statusForEditError(err), err.Error())
		return
	}
	id, err := h.repo.AddImage(user.ID, prompt, "queued", httpx.ClientIP(r), "edit", sourcePath, params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "图片编辑记录创建失败")
		return
	}
	h.writeCreated(w, int(id), user.ID)
}

func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	imageID := pathID(r)
	image, found, _ := h.repo.GetImage(imageID, user.ID)
	if !found {
		httpx.Error(w, http.StatusNotFound, "图片不存在")
		return
	}
	author := image["author"].(map[string]any)
	if author["id"] != user.ID {
		httpx.Error(w, http.StatusForbidden, "无权查看该任务")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	for payload := range h.q.Events(r.Context(), imageID, user.ID) {
		_, _ = w.Write([]byte(payload))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func (h *Handlers) Like(w http.ResponseWriter, r *http.Request) {
	h.toggleRelation(w, r, "image_likes")
}

func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	h.toggleRelation(w, r, "image_favorites")
}

func (h *Handlers) toggleRelation(w http.ResponseWriter, r *http.Request, table string) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	imageID := pathID(r)
	if _, found, _ := h.repo.GetImage(imageID, user.ID); !found {
		httpx.Error(w, http.StatusNotFound, "图片不存在")
		return
	}
	_, err := h.repo.ToggleRelation(table, imageID, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	image, _, _ := h.repo.GetImage(imageID, user.ID)
	httpx.JSON(w, http.StatusOK, image)
}

func (h *Handlers) parseEditRequest(r *http.Request) (string, string, GenerationParams, error) {
	if err := r.ParseMultipartForm(maxSourceImageBytes + 1024); err != nil {
		return "", "", GenerationParams{}, editErr("请求格式错误")
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if len(prompt) < 2 {
		return "", "", GenerationParams{}, editErr("提示词至少需要 2 个字符")
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		return "", "", GenerationParams{}, editErr("上传图片不能为空")
	}
	defer file.Close()
	source, err := h.saveSourceImage(file, header.Filename, header.Header.Get("Content-Type"))
	return prompt, source, paramsFromForm(r), err
}

func paramsFromForm(r *http.Request) GenerationParams {
	var params GenerationParams
	params.Size = r.FormValue("size")
	params.Quality = r.FormValue("quality")
	params.OutputFormat = r.FormValue("output_format")
	params.Moderation = r.FormValue("moderation")
	if value := strings.TrimSpace(r.FormValue("output_compression")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			params.OutputCompression = &parsed
		}
	}
	return params
}

func (h *Handlers) saveSourceImage(file io.Reader, name, contentType string) (string, error) {
	suffix := sourceSuffix(contentType, name)
	if suffix == "" {
		return "", editErr("仅支持 PNG、JPG、WEBP 图片")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxSourceImageBytes+1))
	if err != nil || len(content) == 0 {
		return "", editErr("上传图片不能为空")
	}
	if len(content) > maxSourceImageBytes {
		return "", editTooLarge("上传图片不能超过 10MB")
	}
	dir := filepath.Join(h.cfg.ImageStorageDir, "sources")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := randomHex(12) + suffix
	return "sources/" + filename, os.WriteFile(filepath.Join(dir, filename), content, 0o644)
}

func (h *Handlers) writeCreated(w http.ResponseWriter, imageID, userID int) {
	image, found, _ := h.repo.GetImage(imageID, userID)
	if !found {
		httpx.Error(w, http.StatusInternalServerError, "图片记录创建失败")
		return
	}
	httpx.JSON(w, http.StatusOK, image)
}

func (h *Handlers) requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, err := h.auth.CurrentUser(r)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "请先登录")
		return auth.User{}, false
	}
	return user, true
}

func (h *Handlers) optionalUserID(r *http.Request) int {
	user, err := h.auth.CurrentUser(r)
	if err != nil {
		return 0
	}
	return user.ID
}

func writeImages(w http.ResponseWriter, images []map[string]any, err error) {
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	if images == nil {
		images = []map[string]any{}
	}
	httpx.JSON(w, http.StatusOK, images)
}

func sourceSuffix(contentType, name string) string {
	switch strings.ToLower(strings.Split(contentType, ";")[0]) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return suffixFromContentTypeByName(name)
	}
}

func suffixFromContentTypeByName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		if filepath.Ext(name) == ".jpeg" {
			return ".jpg"
		}
		return strings.ToLower(filepath.Ext(name))
	default:
		return ""
	}
}

func clampLimit(limit int) int {
	if limit < 1 {
		return 1
	}
	if limit > maxPageSize {
		return maxPageSize
	}
	return limit
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func pathID(r *http.Request) int {
	value, _ := strconv.Atoi(r.PathValue("id"))
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type editErr string
type editTooLarge string

func (e editErr) Error() string      { return string(e) }
func (e editTooLarge) Error() string { return string(e) }

func statusForEditError(err error) int {
	if _, ok := err.(editTooLarge); ok {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
