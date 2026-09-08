package client

type UserAccount struct {
	ID          int64                  `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	Status      string                 `json:"status"`
	Plan        string                 `json:"plan"`
	CreatedAt   string                 `json:"created_at"`
	Stats       UserAccountStats       `json:"stats"`
	Limits      UserAccountLimits      `json:"limits"`
	Preferences UserAccountPreferences `json:"preferences"`
}

type UserAccountStats struct {
	Images struct {
		Count            int64 `json:"count"`
		StorageUsedBytes int64 `json:"storage_used_bytes"`
	} `json:"images"`
	Albums struct {
		Count int64 `json:"count"`
	} `json:"albums"`
}

type UserAccountLimits struct {
	MaxUploadBytes          int64    `json:"max_upload_bytes"`
	StorageQuotaBytes       int64    `json:"storage_quota_bytes"`
	CanMultiUpload          bool     `json:"can_multi_upload"`
	ImageUploadEnabled      bool     `json:"image_upload_enabled"`
	DirectUploadEnabled     bool     `json:"direct_upload_enabled"`
	AllowedThumbnailFormats []string `json:"allowed_thumbnail_formats"`
}

type UserAccountPreferences struct {
	UploadNSFWByDefault bool `json:"upload_nsfw_by_default"`
	ThumbnailOriginal   bool `json:"thumbnail_original"`
	ThumbnailWebP       bool `json:"thumbnail_webp"`
	ThumbnailAVIF       bool `json:"thumbnail_avif"`
}
