package api

import (
	"context"
	"io"

	"goph-profile/internal/domain"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=service.go -destination=mock_service_test.go -package=api

type AvatarService interface {
	Upload(context.Context, string, string, io.Reader, int64) (*domain.Avatar, error)
	Get(context.Context, string, string) (*domain.Avatar, io.ReadCloser, string, error)
	Metadata(context.Context, string) (*domain.Avatar, error)
	List(context.Context, string) ([]domain.Avatar, error)
	Delete(context.Context, string, string) error
}
