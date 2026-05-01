package images

import (
	"database/sql"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/timeutil"
)

type Repository struct {
	db  *sql.DB
	cfg config.Config
}

type Provider struct {
	Name         string
	ProviderType string
	Model        string
	APIBase      string
	APIKey       string
}

func NewRepository(database *sql.DB, cfg config.Config) *Repository {
	return &Repository{db: database, cfg: cfg}
}

func (r *Repository) AddImage(userID int, prompt, status, requestIP, taskType, sourcePath string) (int64, error) {
	now := timeutil.LocalTimestamp(r.cfg.AppTimezone)
	res, err := r.db.Exec(`
		INSERT INTO images (user_id, prompt, task_type, source_image_path, status, request_ip, queued_at, created_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`, userID, prompt, taskType, sourcePath, status, requestIP, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) MarkRunning(imageID int, provider Provider) error {
	_, err := r.db.Exec(`
		UPDATE images SET status = 'running', provider_name = ?, model = ?, started_at = ?
		WHERE id = ? AND status = 'queued'
	`, provider.Name, provider.Model, timeutil.LocalTimestamp(r.cfg.AppTimezone), imageID)
	return err
}

func (r *Repository) MarkReady(imageID int, imagePath string) error {
	_, err := r.db.Exec("UPDATE images SET status = 'ready', image_path = ?, error = NULL, completed_at = ? WHERE id = ?", imagePath, timeutil.LocalTimestamp(r.cfg.AppTimezone), imageID)
	return err
}

func (r *Repository) MarkFailed(imageID int, detail string) error {
	_, err := r.db.Exec("UPDATE images SET status = 'failed', error = ?, completed_at = ? WHERE id = ?", detail, timeutil.LocalTimestamp(r.cfg.AppTimezone), imageID)
	return err
}

func (r *Repository) ResetRunningJobs() error {
	_, err := r.db.Exec("UPDATE images SET status = 'queued', started_at = NULL WHERE status = 'running'")
	return err
}

func (r *Repository) NextQueuedJobs(limit int) ([]int, error) {
	rows, err := r.db.Query("SELECT id FROM images WHERE status = 'queued' ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) QueueCounts() map[string]int {
	var queued, running int
	_ = r.db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0)
		FROM images
	`).Scan(&queued, &running)
	return map[string]int{"queued": queued, "running": running}
}

func (r *Repository) QueuePosition(imageID int) any {
	var status string
	if err := r.db.QueryRow("SELECT status FROM images WHERE id = ?", imageID).Scan(&status); err != nil {
		return nil
	}
	if status != "queued" {
		return 0
	}
	var position int
	_ = r.db.QueryRow("SELECT COUNT(*) FROM images WHERE status = 'queued' AND id <= ?", imageID).Scan(&position)
	return position
}

func (r *Repository) ListImages(viewerID int, sort string, limit, offset int) ([]map[string]any, error) {
	order := "images.created_at DESC"
	if sort == "popular" {
		order = "likes DESC, images.created_at DESC"
	}
	where := "WHERE images.is_hidden = 0"
	args := []any{viewerID, viewerID}
	if sort == "favorites" && viewerID > 0 {
		where += " AND EXISTS (SELECT 1 FROM image_favorites f WHERE f.image_id = images.id AND f.user_id = ?)"
		args = append(args, viewerID)
	}
	args = append(args, limit, offset)
	return r.queryImages(baseImageSQL(where, order), args...)
}

func (r *Repository) ListUserImages(userID, limit, offset int) ([]map[string]any, error) {
	sqlText := baseImageSQL("WHERE images.user_id = ?", "images.id DESC")
	return r.queryImages(sqlText, userID, userID, userID, limit, offset)
}

func (r *Repository) GetImage(imageID, viewerID int) (map[string]any, bool, error) {
	rows, err := r.queryImages(baseImageSQL("WHERE images.id = ?", "images.id DESC"), viewerID, viewerID, imageID, 1, 0)
	if err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	return rows[0], true, nil
}

func (r *Repository) ToggleRelation(table string, imageID, userID int) (bool, error) {
	var existing int
	err := r.db.QueryRow("SELECT 1 FROM "+table+" WHERE image_id = ? AND user_id = ?", imageID, userID).Scan(&existing)
	if err == nil {
		_, err = r.db.Exec("DELETE FROM "+table+" WHERE image_id = ? AND user_id = ?", imageID, userID)
		return false, err
	}
	_, err = r.db.Exec("INSERT INTO "+table+" (image_id, user_id) VALUES (?, ?)", imageID, userID)
	return true, err
}

func (r *Repository) ActiveProvider() (Provider, bool, error) {
	row := r.db.QueryRow(`
		SELECT name, provider_type, model, api_base, api_key
		FROM model_providers WHERE enabled = 1
		ORDER BY is_default DESC, id ASC LIMIT 1
	`)
	var provider Provider
	err := row.Scan(&provider.Name, &provider.ProviderType, &provider.Model, &provider.APIBase, &provider.APIKey)
	if err == sql.ErrNoRows {
		return Provider{}, false, nil
	}
	return provider, err == nil, err
}

func (r *Repository) GetConcurrency() int {
	var value string
	err := r.db.QueryRow("SELECT value FROM app_settings WHERE key = 'generation_concurrency'").Scan(&value)
	if err != nil || value == "" {
		return 1
	}
	if value < "1" || value > "8" {
		return 1
	}
	return int(value[0] - '0')
}

func (r *Repository) queryImages(query string, args ...any) ([]map[string]any, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []map[string]any
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}
