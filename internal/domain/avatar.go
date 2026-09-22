package domain

import (
	"errors"
	"time"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

var (
	ErrNotFound  = errors.New("avatar not found")
	ErrForbidden = errors.New("forbidden")
)

type Avatar struct {
	ID               string            `json:"id"`
	UserID           string            `json:"user_id"`
	FileName         string            `json:"file_name"`
	MIMEType         string            `json:"mime_type"`
	SizeBytes        int64             `json:"size"`
	S3Key            string            `json:"-"`
	ThumbnailS3Keys  map[string]string `json:"thumbnail_s3_keys,omitempty"`
	UploadStatus     string            `json:"upload_status"`
	ProcessingStatus string            `json:"processing_status"`
	Width            int               `json:"width,omitempty"`
	Height           int               `json:"height,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type ProcessEvent struct {
	MessageID string   `json:"message_id"`
	AvatarID  string   `json:"avatar_id"`
	UserID    string   `json:"user_id"`
	S3Key     string   `json:"s3_key"`
	S3Keys    []string `json:"s3_keys,omitempty"`
	Action    string   `json:"action"`
}
