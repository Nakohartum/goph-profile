package storage

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"goph-profile/internal/domain"
)

type MinIO struct {
	client *minio.Client
	bucket string
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
	_, e := m.client.PutObject(ctx, m.bucket, key, r, n, minio.PutObjectOptions{ContentType: ct})
	return e
}
func (m *MinIO) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
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
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}
