package admin

import (
	"database/sql"
	"strings"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserOverview(rows rowScanner) (UserOverview, error) {
	var lastIP, lastLogin, createdAt sql.NullString
	var id, isAdmin, total, ready, failed, active int
	var username, displayName, avatarColor string
	err := rows.Scan(&id, &username, &displayName, &avatarColor, &isAdmin, &lastIP, &lastLogin, &createdAt, &total, &ready, &failed, &active)
	if err != nil {
		return UserOverview{}, err
	}
	return UserOverview{
		ID:               id,
		Username:         username,
		DisplayName:      displayName,
		AvatarColor:      avatarColor,
		IsAdmin:          isAdmin == 1,
		LastLoginIP:      nullString(lastIP),
		LastLoginAt:      nullString(lastLogin),
		CreatedAt:        nullString(createdAt),
		TotalGenerations: total,
		ReadyCount:       ready,
		FailedCount:      failed,
		ActiveCount:      active,
	}, nil
}

func scanProvider(rows rowScanner) (ProviderResponse, error) {
	var id, enabled, isDefault int
	var name, providerType, model, apiBase, apiKey, updatedAt string
	err := rows.Scan(&id, &name, &providerType, &model, &apiBase, &apiKey, &enabled, &isDefault, &updatedAt)
	if err != nil {
		return ProviderResponse{}, err
	}
	preview := ""
	if len(apiKey) >= 4 {
		preview = "..." + apiKey[len(apiKey)-4:]
	}
	return ProviderResponse{
		ID:            id,
		Name:          name,
		ProviderType:  providerType,
		Model:         model,
		APIBase:       apiBase,
		Enabled:       enabled == 1,
		IsDefault:     isDefault == 1,
		APIKeySet:     apiKey != "",
		APIKeyPreview: preview,
		UpdatedAt:     updatedAt,
	}, nil
}

func scanGenerationRecord(rows rowScanner) (GenerationRecord, error) {
	var id, isHidden int
	var imagePath, sourcePath, errText, requestIP, provider, model, queued, started, completed sql.NullString
	var taskType, prompt, status, createdAt, username, displayName string
	err := rows.Scan(&id, &imagePath, &taskType, &sourcePath, &isHidden, &prompt, &status, &errText, &requestIP, &provider, &model, &queued, &started, &completed, &createdAt, &username, &displayName)
	if err != nil {
		return GenerationRecord{}, err
	}
	return GenerationRecord{
		ID:              id,
		ImagePath:       nullString(imagePath),
		TaskType:        taskType,
		SourceImagePath: nullString(sourcePath),
		IsHidden:        isHidden == 1,
		Prompt:          prompt,
		Status:          status,
		Error:           nullString(errText),
		RequestIP:       nullString(requestIP),
		ProviderName:    nullString(provider),
		Model:           nullString(model),
		QueuedAt:        nullString(queued),
		StartedAt:       nullString(started),
		CompletedAt:     nullString(completed),
		CreatedAt:       createdAt,
		Username:        username,
		DisplayName:     displayName,
	}, nil
}

func nullString(value sql.NullString) *string {
	if value.Valid {
		return &value.String
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func strPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isPathInside(root, target string) bool {
	root = strings.TrimRight(root, "/") + "/"
	return strings.HasPrefix(target, root)
}

const generationRecordsSQL = `
SELECT images.id, images.image_path, images.task_type, images.source_image_path,
	images.is_hidden, images.prompt, images.status, images.error, images.request_ip,
	images.provider_name, images.model, images.queued_at, images.started_at,
	images.completed_at, images.created_at, users.username, users.display_name
FROM images
JOIN users ON users.id = images.user_id
ORDER BY images.id DESC
LIMIT ?
`
