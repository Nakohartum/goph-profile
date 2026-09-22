package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"goph-profile/internal/domain"
)

type rowStub struct {
	values []any
	err    error
}

func (r rowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *int64:
			*d = v.(int64)
		case *[]byte:
			*d = v.([]byte)
		case *int:
			*d = v.(int)
		case *time.Time:
			*d = v.(time.Time)
		}
	}
	return nil
}
func TestScanAvatar(t *testing.T) {
	now := time.Now()
	row := rowStub{values: []any{"id", "user", "a.png", "image/png", int64(10), "key", []byte(`{"100x100":"thumb"}`), "uploaded", domain.StatusCompleted, 10, 20, now, now}}
	a, err := scanAvatar(row)
	if err != nil || a.ID != "id" || a.ThumbnailS3Keys["100x100"] != "thumb" {
		t.Fatalf("avatar=%#v err=%v", a, err)
	}
	if _, err = scanAvatar(rowStub{err: pgx.ErrNoRows}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	row.values[6] = []byte("bad")
	if _, err = scanAvatar(row); err == nil {
		t.Fatal("expected JSON error")
	}
}
