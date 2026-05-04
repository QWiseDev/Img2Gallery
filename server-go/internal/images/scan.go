package images

import "database/sql"

type scanner interface {
	Scan(dest ...any) error
}

func scanImage(rows scanner) (Image, error) {
	var id, userID, isHidden, likes, favorites, liked, favorited int
	var prompt, taskType, status, createdAt, username, displayName, avatarColor string
	var size, quality, outputFormat, moderation string
	var imagePath, sourcePath, errText, requestIP, provider, model, queued, started, completed sql.NullString
	var outputCompression sql.NullInt64
	err := rows.Scan(&id, &userID, &prompt, &imagePath, &taskType, &sourcePath, &isHidden, &status, &errText, &requestIP, &provider, &model, &queued, &started, &completed, &createdAt, &username, &displayName, &avatarColor, &size, &quality, &outputFormat, &outputCompression, &moderation, &likes, &favorites, &liked, &favorited)
	if err != nil {
		return Image{}, err
	}
	return Image{
		ID:              id,
		Prompt:          prompt,
		ImageURL:        mediaURL(imagePath),
		TaskType:        taskType,
		SourceImagePath: nullString(sourcePath),
		SourceImageURL:  mediaURL(sourcePath),
		IsHidden:        isHidden == 1,
		Status:          status,
		Error:           nullString(errText),
		RequestIP:       nullString(requestIP),
		ProviderName:    nullString(provider),
		Model:           nullString(model),
		QueuedAt:        nullString(queued),
		StartedAt:       nullString(started),
		CompletedAt:     nullString(completed),
		CreatedAt:       createdAt,
		Params: GenerationParams{
			Size:              size,
			Quality:           quality,
			OutputFormat:      outputFormat,
			OutputCompression: nullInt(outputCompression),
			Moderation:        moderation,
		},
		Author: ImageAuthor{
			ID:          userID,
			Username:    username,
			DisplayName: displayName,
			AvatarColor: avatarColor,
		},
		Likes:         likes,
		Favorites:     favorites,
		LikedByMe:     liked == 1,
		FavoritedByMe: favorited == 1,
	}, nil
}

func mediaURL(value sql.NullString) *string {
	if value.Valid && value.String != "" {
		path := "/media/" + value.String
		return &path
	}
	return nil
}

func nullString(value sql.NullString) *string {
	if value.Valid {
		return &value.String
	}
	return nil
}

func nullInt(value sql.NullInt64) *int {
	if value.Valid {
		intValue := int(value.Int64)
		return &intValue
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
JOIN image_params ON image_params.image_id = images.id
LEFT JOIN image_likes ON image_likes.image_id = images.id
LEFT JOIN image_favorites ON image_favorites.image_id = images.id
` + where + `
GROUP BY images.id
ORDER BY ` + order + `
LIMIT ? OFFSET ?
`
}
