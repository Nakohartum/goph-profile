package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"goph-profile/internal/domain"
	"goph-profile/internal/observability"
)

type Postgres struct{ pool *pgxpool.Pool }

func dbSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer("goph-profile/postgres").Start(ctx, "postgres."+operation, trace.WithAttributes(attribute.String("db.system", "postgresql"), attribute.String("db.operation.name", operation)))
}
func (p *Postgres) updatePoolMetrics() {
	s := p.pool.Stat()
	observability.DBConnections.WithLabelValues("acquired").Set(float64(s.AcquiredConns()))
	observability.DBConnections.WithLabelValues("idle").Set(float64(s.IdleConns()))
	observability.DBConnections.WithLabelValues("total").Set(float64(s.TotalConns()))
}

func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return &Postgres{pool: p}, nil
}
func (p *Postgres) Close()                         { p.pool.Close() }
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Postgres) Create(ctx context.Context, a *domain.Avatar) error {
	ctx, span := dbSpan(ctx, "insert")
	defer span.End()
	defer p.updatePoolMetrics()
	_, e := p.pool.Exec(ctx, `INSERT INTO avatars(id,user_id,file_name,mime_type,size_bytes,s3_key,upload_status,processing_status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, a.ID, a.UserID, a.FileName, a.MIMEType, a.SizeBytes, a.S3Key, a.UploadStatus, a.ProcessingStatus, a.CreatedAt, a.UpdatedAt)
	if e != nil {
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
	}
	return e
}

const selectAvatar = `SELECT id,user_id,file_name,mime_type,size_bytes,s3_key,thumbnail_s3_keys,upload_status,processing_status,width,height,created_at,updated_at FROM avatars WHERE id=$1 AND deleted_at IS NULL`

func scanAvatar(row pgx.Row) (*domain.Avatar, error) {
	var a domain.Avatar
	var raw []byte
	e := row.Scan(&a.ID, &a.UserID, &a.FileName, &a.MIMEType, &a.SizeBytes, &a.S3Key, &raw, &a.UploadStatus, &a.ProcessingStatus, &a.Width, &a.Height, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	if len(raw) > 0 {
		if e = json.Unmarshal(raw, &a.ThumbnailS3Keys); e != nil {
			return nil, fmt.Errorf("decode thumbnails: %w", e)
		}
	}
	return &a, nil
}
func (p *Postgres) Get(ctx context.Context, id string) (*domain.Avatar, error) {
	ctx, span := dbSpan(ctx, "select")
	defer span.End()
	defer p.updatePoolMetrics()
	return scanAvatar(p.pool.QueryRow(ctx, selectAvatar, id))
}
func (p *Postgres) ListByUser(ctx context.Context, user string) ([]domain.Avatar, error) {
	ctx, span := dbSpan(ctx, "select_list")
	defer span.End()
	defer p.updatePoolMetrics()
	rows, e := p.pool.Query(ctx, `SELECT id,user_id,file_name,mime_type,size_bytes,s3_key,thumbnail_s3_keys,upload_status,processing_status,width,height,created_at,updated_at FROM avatars WHERE user_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Avatar{}
	for rows.Next() {
		a, e := scanAvatar(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}
func (p *Postgres) SoftDelete(ctx context.Context, id string) error {
	ctx, span := dbSpan(ctx, "soft_delete")
	defer span.End()
	defer p.updatePoolMetrics()
	tag, e := p.pool.Exec(ctx, `UPDATE avatars SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if e == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return e
}
func (p *Postgres) BeginProcessing(ctx context.Context, id, msg string) (bool, error) {
	ctx, span := dbSpan(ctx, "begin_processing")
	defer span.End()
	defer p.updatePoolMetrics()
	tag, e := p.pool.Exec(ctx, `UPDATE avatars SET processing_status='processing',processing_message_id=$2,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL AND processing_status IN ('pending','failed')`, id, msg)
	return e == nil && tag.RowsAffected() == 1, e
}
func (p *Postgres) CompleteProcessing(ctx context.Context, id string, w, h int, thumbs map[string]string) error {
	ctx, span := dbSpan(ctx, "complete_processing")
	defer span.End()
	defer p.updatePoolMetrics()
	b, e := json.Marshal(thumbs)
	if e != nil {
		return e
	}
	_, e = p.pool.Exec(ctx, `UPDATE avatars SET processing_status='completed',thumbnail_s3_keys=$2,width=$3,height=$4,updated_at=NOW() WHERE id=$1`, id, b, w, h)
	return e
}
func (p *Postgres) FailProcessing(ctx context.Context, id string) error {
	ctx, span := dbSpan(ctx, "fail_processing")
	defer span.End()
	defer p.updatePoolMetrics()
	_, e := p.pool.Exec(ctx, `UPDATE avatars SET processing_status='failed',updated_at=NOW() WHERE id=$1`, id)
	return e
}
