package admin

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/timeutil"
)

type Repository struct {
	db  *sql.DB
	cfg config.Config
}

type ProviderPayload struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	ProviderType string  `json:"provider_type"`
	Model        string  `json:"model"`
	APIBase      string  `json:"api_base"`
	APIKey       *string `json:"api_key"`
	Enabled      bool    `json:"enabled"`
	IsDefault    bool    `json:"is_default"`
}

func NewRepository(database *sql.DB, cfg config.Config) *Repository {
	return &Repository{db: database, cfg: cfg}
}

func (r *Repository) Dashboard() (Dashboard, error) {
	var total, queued, running, ready, failed int
	err := r.db.QueryRow(`
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'ready' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM images
	`).Scan(&total, &queued, &running, &ready, &failed)
	if err != nil {
		return Dashboard{}, err
	}
	users, _ := r.countUsers()
	providers, _ := r.ListProviders()
	return Dashboard{
		Users: users,
		Images: DashboardImages{
			Total:   total,
			Queued:  queued,
			Running: running,
			Ready:   ready,
			Failed:  failed,
		},
		Concurrency: r.GetConcurrency(),
		Providers:   providers,
	}, nil
}

func (r *Repository) UsersOverview() ([]UserOverview, error) {
	rows, err := r.db.Query(`
		SELECT users.id, users.username, users.display_name, users.avatar_color, users.is_admin,
			users.last_login_ip, users.last_login_at, users.created_at, COUNT(images.id),
			COALESCE(SUM(CASE WHEN images.status = 'ready' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN images.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN images.status IN ('queued', 'running') THEN 1 ELSE 0 END), 0)
		FROM users LEFT JOIN images ON images.user_id = users.id
		GROUP BY users.id ORDER BY users.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []UserOverview
	for rows.Next() {
		item, err := scanUserOverview(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, item)
	}
	return users, rows.Err()
}

func (r *Repository) SetUserAdmin(userID int, isAdmin bool) (UserAdminResult, bool, error) {
	_, err := r.db.Exec("UPDATE users SET is_admin = ? WHERE id = ?", boolInt(isAdmin), userID)
	if err != nil {
		return UserAdminResult{}, false, err
	}
	row := r.db.QueryRow("SELECT id, username, display_name, avatar_color, is_admin FROM users WHERE id = ?", userID)
	var id, adminFlag int
	var username, displayName, avatarColor string
	if err := row.Scan(&id, &username, &displayName, &avatarColor, &adminFlag); err != nil {
		return UserAdminResult{}, false, nil
	}
	return UserAdminResult{ID: id, Username: username, DisplayName: displayName, AvatarColor: avatarColor, IsAdmin: adminFlag == 1}, true, nil
}

func (r *Repository) GenerationRecords(limit int) ([]GenerationRecord, error) {
	rows, err := r.db.Query(generationRecordsSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []GenerationRecord
	for rows.Next() {
		item, err := scanGenerationRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, item)
	}
	return records, rows.Err()
}

func (r *Repository) SetGenerationHidden(imageID int, hidden bool) (GenerationHiddenResult, bool, error) {
	_, err := r.db.Exec("UPDATE images SET is_hidden = ? WHERE id = ?", boolInt(hidden), imageID)
	if err != nil {
		return GenerationHiddenResult{}, false, err
	}
	var id, isHidden int
	err = r.db.QueryRow("SELECT id, is_hidden FROM images WHERE id = ?", imageID).Scan(&id, &isHidden)
	if err != nil {
		return GenerationHiddenResult{}, false, nil
	}
	return GenerationHiddenResult{ID: id, IsHidden: isHidden == 1}, true, nil
}

func (r *Repository) DeleteGeneration(imageID int) (DeleteGenerationResult, bool, error) {
	var imagePath, sourceImagePath sql.NullString
	err := r.db.QueryRow("SELECT image_path, source_image_path FROM images WHERE id = ?", imageID).Scan(&imagePath, &sourceImagePath)
	if err != nil {
		return DeleteGenerationResult{}, false, nil
	}
	if _, err := r.db.Exec("DELETE FROM images WHERE id = ?", imageID); err != nil {
		return DeleteGenerationResult{}, false, err
	}
	r.deleteStorageFile(imagePath)
	r.deleteStorageFile(sourceImagePath)
	return DeleteGenerationResult{ID: imageID}, true, nil
}

func (r *Repository) ListProviders() ([]ProviderResponse, error) {
	rows, err := r.db.Query("SELECT id, name, provider_type, model, api_base, api_key, enabled, is_default, updated_at FROM model_providers ORDER BY is_default DESC, id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []ProviderResponse
	for rows.Next() {
		item, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, item)
	}
	return providers, rows.Err()
}

func (r *Repository) UpsertProvider(payload ProviderPayload) (ProviderResponse, error) {
	if payload.IsDefault {
		_, _ = r.db.Exec("UPDATE model_providers SET is_default = 0")
	}
	providerID, err := r.saveProvider(payload)
	if err != nil {
		return ProviderResponse{}, err
	}
	row := r.db.QueryRow("SELECT id, name, provider_type, model, api_base, api_key, enabled, is_default, updated_at FROM model_providers WHERE id = ?", providerID)
	return scanProvider(row)
}

func (r *Repository) GetConcurrency() int {
	value := r.GetSetting("generation_concurrency", "1")
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 1
	}
	if parsed > 8 {
		return 8
	}
	return parsed
}

func (r *Repository) GetSetting(key, fallback string) string {
	var value string
	err := r.db.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return fallback
	}
	return value
}

func (r *Repository) SetSetting(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, timeutil.LocalTimestamp(r.cfg.AppTimezone))
	return err
}

func (r *Repository) saveProvider(payload ProviderPayload) (int64, error) {
	now := timeutil.LocalTimestamp(r.cfg.AppTimezone)
	if payload.ID > 0 {
		return int64(payload.ID), r.updateProvider(payload, now)
	}
	res, err := r.db.Exec(`
		INSERT INTO model_providers (name, provider_type, model, api_base, api_key, enabled, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, payload.Name, payload.ProviderType, payload.Model, payload.APIBase, strPtr(payload.APIKey), boolInt(payload.Enabled), boolInt(payload.IsDefault), now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) updateProvider(payload ProviderPayload, now string) error {
	key := strPtr(payload.APIKey)
	if payload.APIKey == nil {
		_ = r.db.QueryRow("SELECT api_key FROM model_providers WHERE id = ?", payload.ID).Scan(&key)
	}
	_, err := r.db.Exec(`
		UPDATE model_providers
		SET name = ?, provider_type = ?, model = ?, api_base = ?, api_key = ?, enabled = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`, payload.Name, payload.ProviderType, payload.Model, payload.APIBase, key, boolInt(payload.Enabled), boolInt(payload.IsDefault), now, payload.ID)
	return err
}

func (r *Repository) countUsers() (int, error) {
	var users int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users)
	return users, err
}

func (r *Repository) deleteStorageFile(relative sql.NullString) {
	if !relative.Valid || relative.String == "" {
		return
	}
	root, _ := filepath.Abs(r.cfg.ImageStorageDir)
	target, _ := filepath.Abs(filepath.Join(r.cfg.ImageStorageDir, relative.String))
	if root != target && !isPathInside(root, target) {
		return
	}
	_ = os.Remove(target)
}
