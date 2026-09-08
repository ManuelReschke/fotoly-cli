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

type ImageListQuery struct {
	Limit    int
	Cursor   string
	AlbumID  int64
	IsPublic *bool
	IsNSFW   *bool
	Tag      string
}

type ImageCollection struct {
	Items      []ImageSummary `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor *string        `json:"next_cursor"`
}

type ImageSummary struct {
	ImageUUID     string `json:"image_uuid"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	IsPublic      bool   `json:"is_public"`
	IsNSFW        bool   `json:"is_nsfw"`
	ShareLink     string `json:"share_link"`
	ViewURL       string `json:"view_url"`
	StableURL     string `json:"stable_url"`
	ViewCount     int    `json:"view_count"`
	DownloadCount int    `json:"download_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type ImageResource struct {
	ImageUUID         string   `json:"image_uuid"`
	ViewURL           string   `json:"view_url,omitempty"`
	IsNSFW            bool     `json:"is_nsfw"`
	Tags              []string `json:"tags,omitempty"`
	URL               string   `json:"url,omitempty"`
	AvailableVariants []string `json:"available_variants,omitempty"`
}

type ImageStatus struct {
	Complete bool    `json:"complete"`
	Failed   bool    `json:"failed"`
	ViewURL  *string `json:"view_url"`
}

type ImageDeletionAccepted struct {
	ImageUUID string `json:"image_uuid"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

type AlbumCollection struct {
	Albums []AlbumSummary `json:"albums"`
}

type AlbumSummary struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	IsNSFW      bool   `json:"is_nsfw"`
	ShareLink   string `json:"share_link"`
	ViewURL     string `json:"view_url"`
	ImageCount  int64  `json:"image_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type UploadSessionRequest struct {
	FileSize   int64  `json:"file_size"`
	AlbumID    *int64 `json:"album_id,omitempty"`
	IsNSFW     *bool  `json:"is_nsfw,omitempty"`
	Processing *struct {
		Profile string `json:"profile"`
	} `json:"processing,omitempty"`
}

type UploadSessionResponse struct {
	UploadURL string `json:"upload_url"`
	Token     string `json:"token"`
	PoolID    int64  `json:"pool_id"`
	ExpiresAt int64  `json:"expires_at"`
	MaxBytes  int64  `json:"max_bytes"`
	AlbumID   *int64 `json:"album_id,omitempty"`
}

type StorageUploadResponse struct {
	ImageUUID *string `json:"image_uuid"`
	ViewURL   *string `json:"view_url"`
	URL       *string `json:"url"`
	Duplicate *bool   `json:"duplicate"`
}
