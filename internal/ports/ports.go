package ports

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=ports.go -destination=../mocks/mock_ports.go -package=mocks

import (
	"context"
	"io"

	"goph-profile/internal/domain"
)

type AvatarRepository interface {
	Create(context.Context, *domain.Avatar) error
	Get(context.Context, string) (*domain.Avatar, error)
	ListByUser(context.Context, string) ([]domain.Avatar, error)
	SoftDelete(context.Context, string) error
	BeginProcessing(context.Context, string, string) (bool, error)
	CompleteProcessing(context.Context, string, int, int, map[string]string) error
	FailProcessing(context.Context, string) error
}

type ObjectStorage interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, string, error)
	Delete(context.Context, string) error
}

type EventPublisher interface {
	Publish(context.Context, domain.ProcessEvent) error
}
