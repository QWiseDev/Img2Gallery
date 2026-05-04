package admin

type AdminIdentity struct {
	Role     string `json:"role"`
	Source   string `json:"source"`
	UserID   *int   `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

type Dashboard struct {
	Users       int                `json:"users"`
	Images      DashboardImages    `json:"images"`
	Concurrency int                `json:"concurrency"`
	Providers   []ProviderResponse `json:"providers"`
}

type DashboardImages struct {
	Total   int `json:"total"`
	Queued  int `json:"queued"`
	Running int `json:"running"`
	Ready   int `json:"ready"`
	Failed  int `json:"failed"`
}

type UserOverview struct {
	ID               int     `json:"id"`
	Username         string  `json:"username"`
	DisplayName      string  `json:"display_name"`
	AvatarColor      string  `json:"avatar_color"`
	IsAdmin          bool    `json:"is_admin"`
	LastLoginIP      *string `json:"last_login_ip"`
	LastLoginAt      *string `json:"last_login_at"`
	CreatedAt        *string `json:"created_at"`
	TotalGenerations int     `json:"total_generations"`
	ReadyCount       int     `json:"ready_count"`
	FailedCount      int     `json:"failed_count"`
	ActiveCount      int     `json:"active_count"`
}

type UserAdminResult struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarColor string `json:"avatar_color"`
	IsAdmin     bool   `json:"is_admin"`
}

type GenerationRecord struct {
	ID              int     `json:"id"`
	ImagePath       *string `json:"image_path"`
	TaskType        string  `json:"task_type"`
	SourceImagePath *string `json:"source_image_path"`
	IsHidden        bool    `json:"is_hidden"`
	Prompt          string  `json:"prompt"`
	Status          string  `json:"status"`
	Error           *string `json:"error"`
	RequestIP       *string `json:"request_ip"`
	ProviderName    *string `json:"provider_name"`
	Model           *string `json:"model"`
	QueuedAt        *string `json:"queued_at"`
	StartedAt       *string `json:"started_at"`
	CompletedAt     *string `json:"completed_at"`
	CreatedAt       string  `json:"created_at"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"display_name"`
}

type GenerationHiddenResult struct {
	ID       int  `json:"id"`
	IsHidden bool `json:"is_hidden"`
}

type DeleteGenerationResult struct {
	ID int `json:"id"`
}

type ProviderResponse struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ProviderType  string `json:"provider_type"`
	Model         string `json:"model"`
	APIBase       string `json:"api_base"`
	Enabled       bool   `json:"enabled"`
	IsDefault     bool   `json:"is_default"`
	APIKeySet     bool   `json:"api_key_set"`
	APIKeyPreview string `json:"api_key_preview"`
	UpdatedAt     string `json:"updated_at"`
}
