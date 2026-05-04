package images

type Image struct {
	ID              int              `json:"id"`
	Prompt          string           `json:"prompt"`
	ImageURL        *string          `json:"image_url"`
	TaskType        string           `json:"task_type"`
	SourceImagePath *string          `json:"source_image_path"`
	SourceImageURL  *string          `json:"source_image_url"`
	IsHidden        bool             `json:"is_hidden"`
	Status          string           `json:"status"`
	Error           *string          `json:"error"`
	RequestIP       *string          `json:"request_ip"`
	ProviderName    *string          `json:"provider_name"`
	Model           *string          `json:"model"`
	QueuedAt        *string          `json:"queued_at"`
	StartedAt       *string          `json:"started_at"`
	CompletedAt     *string          `json:"completed_at"`
	CreatedAt       string           `json:"created_at"`
	Params          GenerationParams `json:"params"`
	Author          ImageAuthor      `json:"author"`
	Likes           int              `json:"likes"`
	Favorites       int              `json:"favorites"`
	LikedByMe       bool             `json:"liked_by_me"`
	FavoritedByMe   bool             `json:"favorited_by_me"`
}

type ImageAuthor struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarColor string `json:"avatar_color"`
}

type QueueCounts struct {
	Queued  int `json:"queued"`
	Running int `json:"running"`
}

type QueueEvent struct {
	Status   string      `json:"status"`
	Position *int        `json:"position"`
	Queue    QueueCounts `json:"queue"`
	Image    *Image      `json:"image,omitempty"`
}
