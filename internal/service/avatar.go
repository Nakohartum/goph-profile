package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"goph-profile/internal/domain"
	"goph-profile/internal/ports"
)

const MaxUploadSize int64 = 10 << 20

var (
	ErrEmptyUserID   = errors.New("X-User-ID is required")
	ErrEmptyFile     = errors.New("file is required")
	ErrFileTooLarge  = errors.New("file too large")
	ErrInvalidFormat = errors.New("invalid file format")
)

type AvatarService struct {
	repo      ports.AvatarRepository
	storage   ports.ObjectStorage
	publisher ports.EventPublisher
}

func NewAvatarService(r ports.AvatarRepository, s ports.ObjectStorage, p ports.EventPublisher) *AvatarService {
	return &AvatarService{repo: r, storage: s, publisher: p}
}

func (s *AvatarService) Upload(ctx context.Context, userID, fileName string, data io.Reader, declaredSize int64) (*domain.Avatar, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrEmptyUserID
	}
	if data == nil {
		return nil, ErrEmptyFile
	}
	if declaredSize > MaxUploadSize {
		return nil, ErrFileTooLarge
	}
	b, err := io.ReadAll(io.LimitReader(data, MaxUploadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(b) == 0 {
		return nil, ErrEmptyFile
	}
	if int64(len(b)) > MaxUploadSize {
		return nil, ErrFileTooLarge
	}
	mimeType := http.DetectContentType(b)
	if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
		return nil, ErrInvalidFormat
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		ext = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mimeType]
	}
	id := uuid.NewString()
	key := fmt.Sprintf("avatars/%s/original%s", id, ext)
	now := time.Now().UTC()
	a := &domain.Avatar{ID: id, UserID: userID, FileName: filepath.Base(fileName), MIMEType: mimeType, SizeBytes: int64(len(b)), S3Key: key, UploadStatus: "uploaded", ProcessingStatus: domain.StatusPending, CreatedAt: now, UpdatedAt: now}
	if err = s.storage.Put(ctx, key, bytes.NewReader(b), int64(len(b)), mimeType); err != nil {
		return nil, fmt.Errorf("store avatar: %w", err)
	}
	if err = s.repo.Create(ctx, a); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, fmt.Errorf("create metadata: %w", err)
	}
	if err = s.publisher.Publish(ctx, domain.ProcessEvent{MessageID: uuid.NewString(), AvatarID: id, UserID: userID, S3Key: key, Action: "process"}); err != nil {
		return nil, fmt.Errorf("publish processing event: %w", err)
	}
	return a, nil
}

func (s *AvatarService) Get(ctx context.Context, id, size string) (*domain.Avatar, io.ReadCloser, string, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, "", err
	}
	key := a.S3Key
	if size != "" && size != "original" {
		v, ok := a.ThumbnailS3Keys[size]
		if !ok {
			return nil, nil, "", domain.ErrNotFound
		}
		key = v
	}
	r, ct, err := s.storage.Get(ctx, key)
	return a, r, ct, err
}
func (s *AvatarService) Metadata(ctx context.Context, id string) (*domain.Avatar, error) {
	return s.repo.Get(ctx, id)
}
func (s *AvatarService) List(ctx context.Context, user string) ([]domain.Avatar, error) {
	return s.repo.ListByUser(ctx, user)
}
func (s *AvatarService) Delete(ctx context.Context, id, user string) error {
	if strings.TrimSpace(user) == "" {
		return ErrEmptyUserID
	}
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.UserID != user {
		return domain.ErrForbidden
	}
	if err = s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	keys := []string{a.S3Key}
	for _, key := range a.ThumbnailS3Keys {
		keys = append(keys, key)
	}
	return s.publisher.Publish(ctx, domain.ProcessEvent{MessageID: uuid.NewString(), AvatarID: id, UserID: user, S3Key: a.S3Key, S3Keys: keys, Action: "delete"})
}
