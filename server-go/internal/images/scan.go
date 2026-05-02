package images

import "database/sql"

type scanner interface {
	Scan(dest ...any) error
}

func scanImage(rows scanner) (map[string]any, error) {
	var id, userID, isHidden, likes, favorites, liked, favorited int
	var prompt, taskType, status, createdAt, username, displayName, avatarColor string
	var imagePath, sourcePath, errText, requestIP, provider, model, queued, started, completed sql.NullString
	var size, quality, outputFormat, moderation sql.NullString
	var outputCompression sql.NullInt64
	err := rows.Scan(&id, &userID, &prompt, &imagePath, &taskType, &sourcePath, &isHidden, &status, &errText, &requestIP, &provider, &model, &queued, &started, &completed, &createdAt, &username, &displayName, &avatarColor, &size, &quality, &outputFormat, &outputCompression, &moderation, &likes, &favorites, &liked, &favorited)
	if err != nil {
		return nil, err
	}
	imageURL := any(nil)
	if imagePath.Valid && imagePath.String != "" {
		imageURL = "/media/" + imagePath.String
	}
	sourceURL := any(nil)
	if sourcePath.Valid && sourcePath.String != "" {
		sourceURL = "/media/" + sourcePath.String
	}
	return map[string]any{
		"id": id, "prompt": prompt, "image_url": imageURL, "task_type": taskType,
		"source_image_path": nullString(sourcePath), "source_image_url": sourceURL,
		"is_hidden": isHidden == 1, "status": status, "error": nullString(errText),
		"request_ip": nullString(requestIP), "provider_name": nullString(provider),
		"model": nullString(model), "queued_at": nullString(queued), "started_at": nullString(started),
		"completed_at": nullString(completed), "created_at": createdAt,
		"params": map[string]any{
			"size":               defaultString(size, "auto"),
			"quality":            defaultString(quality, "auto"),
			"output_format":      defaultString(outputFormat, "png"),
			"output_compression": nullInt(outputCompression),
			"moderation":         defaultString(moderation, "auto"),
		},
		"author": map[string]any{"id": userID, "username": username, "display_name": displayName, "avatar_color": avatarColor},
		"likes":  likes, "favorites": favorites, "liked_by_me": liked == 1, "favorited_by_me": favorited == 1,
	}, nil
}

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func defaultString(value sql.NullString, fallback string) string {
	if value.Valid && value.String != "" {
		return value.String
	}
	return fallback
}

func nullInt(value sql.NullInt64) any {
	if value.Valid {
		return int(value.Int64)
	}
	return nil
}

func baseImageSQL(where, order string) string {
	return `
SELECT images.id, images.user_id, images.prompt, images.image_path, images.task_type,
	images.source_image_path, images.is_hidden, images.status, images.error,
	images.request_ip, images.provider_name, images.model, images.queued_at,
	images.started_at, images.completed_at, images.created_at, users.username,
	users.display_name, users.avatar_color,
	image_params.size, image_params.quality, image_params.output_format,
	image_params.output_compression, image_params.moderation,
	COUNT(DISTINCT image_likes.user_id) AS likes,
	COUNT(DISTINCT image_favorites.user_id) AS favorites,
	EXISTS(SELECT 1 FROM image_likes mine WHERE mine.image_id = images.id AND mine.user_id = ?) AS liked_by_me,
	EXISTS(SELECT 1 FROM image_favorites fav WHERE fav.image_id = images.id AND fav.user_id = ?) AS favorited_by_me
FROM images
JOIN users ON users.id = images.user_id
LEFT JOIN image_params ON image_params.image_id = images.id
LEFT JOIN image_likes ON image_likes.image_id = images.id
LEFT JOIN image_favorites ON image_favorites.image_id = images.id
` + where + `
GROUP BY images.id
ORDER BY ` + order + `
LIMIT ? OFFSET ?
`
}
