package storage

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"goph-profile/internal/domain"
)

type MinIO struct {
	client *minio.Client
	bucket string
}

func (m *MinIO) span(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	return otel.Tracer("goph-profile/s3").Start(ctx, "s3."+operation, trace.WithAttributes(attribute.String("rpc.system", "aws-api"), attribute.String("rpc.service", "S3"), attribute.String("s3.bucket", m.bucket), attribute.String("s3.key", key)))
}

func (m *MinIO) Ping(ctx context.Context) error {
	_, err := m.client.BucketExists(ctx, m.bucket)
	return err
}

func NewMinIO(ctx context.Context, endpoint, access, secret, bucket string, ssl bool) (*MinIO, error) {
	c, e := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: ssl})
	if e != nil {
		return nil, e
	}
	exists, e := c.BucketExists(ctx, bucket)
	if e != nil {
		return nil, e
	}
	if !exists {
		if e = c.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); e != nil {
			return nil, e
		}
	}
	return &MinIO{c, bucket}, nil
}
func (m *MinIO) Put(ctx context.Context, key string, r io.Reader, n int64, ct string) error {
	ctx, span := m.span(ctx, "put_object", key)
	defer span.End()
	span.SetAttributes(attribute.Int64("object.size", n))
	_, e := m.client.PutObject(ctx, m.bucket, key, r, n, minio.PutObjectOptions{ContentType: ct})
	return e
}
func (m *MinIO) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	ctx, span := m.span(ctx, "get_object", key)
	defer span.End()
	o, e := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if e != nil {
		return nil, "", e
	}
	st, e := o.Stat()
	if e != nil {
		_ = o.Close()
		var resp minio.ErrorResponse
		if errors.As(e, &resp) && resp.StatusCode == 404 {
			return nil, "", domain.ErrNotFound
		}
		return nil, "", e
	}
	return o, st.ContentType, nil
}
func (m *MinIO) Delete(ctx context.Context, key string) error {
	ctx, span := m.span(ctx, "delete_object", key)
	defer span.End()
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}
