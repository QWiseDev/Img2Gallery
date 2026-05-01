package admin

import (
	"database/sql"
	"strings"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserOverview(rows rowScanner) (map[string]any, error) {
	var lastIP, lastLogin, createdAt sql.NullString
	var id, isAdmin, total, ready, failed, active int
	var username, displayName, avatarColor string
	err := rows.Scan(&id, &username, &displayName, &avatarColor, &isAdmin, &lastIP, &lastLogin, &createdAt, &total, &ready, &failed, &active)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "username": username, "display_name": displayName, "avatar_color": avatarColor,
		"is_admin": isAdmin == 1, "last_login_ip": nullString(lastIP),
		"last_login_at": nullString(lastLogin), "created_at": nullString(createdAt),
		"total_generations": total, "ready_count": ready, "failed_count": failed, "active_count": active,
	}, nil
}

func scanProvider(rows rowScanner) (map[string]any, error) {
	var id, enabled, isDefault int
	var name, providerType, model, apiBase, apiKey, updatedAt string
	err := rows.Scan(&id, &name, &providerType, &model, &apiBase, &apiKey, &enabled, &isDefault, &updatedAt)
	if err != nil {
		return nil, err
	}
	preview := ""
	if len(apiKey) >= 4 {
		preview = "..." + apiKey[len(apiKey)-4:]
	}
	return map[string]any{
		"id": id, "name": name, "provider_type": providerType, "model": model, "api_base": apiBase,
		"enabled": enabled == 1, "is_default": isDefault == 1, "api_key_set": apiKey != "",
		"api_key_preview": preview, "updated_at": updatedAt,
	}, nil
}

func scanGenerationRecord(rows rowScanner) (map[string]any, error) {
	var id, isHidden int
	var imagePath, sourcePath, errText, requestIP, provider, model, queued, started, completed sql.NullString
	var taskType, prompt, status, createdAt, username, displayName string
	err := rows.Scan(&id, &imagePath, &taskType, &sourcePath, &isHidden, &prompt, &status, &errText, &requestIP, &provider, &model, &queued, &started, &completed, &createdAt, &username, &displayName)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "image_path": nullString(imagePath), "task_type": taskType,
		"source_image_path": nullString(sourcePath), "is_hidden": isHidden == 1,
		"prompt": prompt, "status": status, "error": nullString(errText),
		"request_ip": nullString(requestIP), "provider_name": nullString(provider),
		"model": nullString(model), "queued_at": nullString(queued),
		"started_at": nullString(started), "completed_at": nullString(completed),
		"created_at": createdAt, "username": username, "display_name": displayName,
	}, nil
}

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
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
