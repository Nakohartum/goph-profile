package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"testing"

	"go.uber.org/mock/gomock"

	"goph-profile/internal/domain"
	"goph-profile/internal/mocks"
)

func imageBytes() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return b.Bytes()
}
func testService(t *testing.T) (*AvatarService, *mocks.MockAvatarRepository, *mocks.MockObjectStorage, *mocks.MockEventPublisher) {
	t.Helper()
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	p := mocks.NewMockEventPublisher(c)
	return NewAvatarService(r, s, p), r, s, p
}
func expectUpload(r *mocks.MockAvatarRepository, s *mocks.MockObjectStorage, p *mocks.MockEventPublisher) {
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "image/png").Return(nil)
	r.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
}

func TestUploadAndGet(t *testing.T) {
	svc, r, s, p := testService(t)
	expectUpload(r, s, p)
	a, e := svc.Upload(context.Background(), "user", "avatar.png", bytes.NewReader(imageBytes()), int64(len(imageBytes())))
	if e != nil {
		t.Fatal(e)
	}
	if a.MIMEType != "image/png" {
		t.Fatalf("unexpected result: %#v", a)
	}
	r.EXPECT().Get(gomock.Any(), a.ID).Return(a, nil)
	s.EXPECT().Get(gomock.Any(), a.S3Key).Return(io.NopCloser(bytes.NewReader(imageBytes())), "image/png", nil)
	_, body, ct, e := svc.Get(context.Background(), a.ID, "original")
	if e != nil || ct != "image/png" {
		t.Fatalf("get: %s %v", ct, e)
	}
	_ = body.Close()
	r.EXPECT().Get(gomock.Any(), a.ID).Return(a, nil)
	if _, e = svc.Metadata(context.Background(), a.ID); e != nil {
		t.Fatal(e)
	}
	r.EXPECT().ListByUser(gomock.Any(), "user").Return([]domain.Avatar{*a}, nil)
	if list, e := svc.List(context.Background(), "user"); e != nil || len(list) != 1 {
		t.Fatalf("list: %v", e)
	}
}
func TestUploadValidation(t *testing.T) {
	svc, _, _, _ := testService(t)
	cases := []struct {
		name, user string
		data       io.Reader
		size       int64
		want       error
	}{{"user", "", bytes.NewReader(imageBytes()), 1, ErrEmptyUserID}, {"empty", "u", bytes.NewReader(nil), 0, ErrEmptyFile}, {"large", "u", bytes.NewReader(nil), MaxUploadSize + 1, ErrFileTooLarge}, {"format", "u", bytes.NewBufferString("not image"), 9, ErrInvalidFormat}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := svc.Upload(context.Background(), tc.user, "x", tc.data, tc.size)
			if e != tc.want {
				t.Fatalf("got %v want %v", e, tc.want)
			}
		})
	}
}
func TestDeleteAuthorization(t *testing.T) {
	svc, r, s, p := testService(t)
	expectUpload(r, s, p)
	a, e := svc.Upload(context.Background(), "owner", "a.png", bytes.NewReader(imageBytes()), 0)
	if e != nil {
		t.Fatal(e)
	}
	r.EXPECT().Get(gomock.Any(), a.ID).Return(a, nil)
	if e = svc.Delete(context.Background(), a.ID, "other"); e != domain.ErrForbidden {
		t.Fatalf("got %v", e)
	}
	r.EXPECT().Get(gomock.Any(), a.ID).Return(a, nil)
	r.EXPECT().SoftDelete(gomock.Any(), a.ID).Return(nil)
	p.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e domain.ProcessEvent) error {
		if e.Action != "delete" {
			t.Fatal("wrong action")
		}
		return nil
	})
	if e = svc.Delete(context.Background(), a.ID, "owner"); e != nil {
		t.Fatal(e)
	}
}
func TestMissingThumbnail(t *testing.T) {
	svc, r, _, _ := testService(t)
	r.EXPECT().Get(gomock.Any(), "id").Return(&domain.Avatar{ID: "id"}, nil)
	_, _, _, e := svc.Get(context.Background(), "id", "100x100")
	if e != domain.ErrNotFound {
		t.Fatalf("got %v", e)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func TestUploadDependencyFailures(t *testing.T) {
	svc, r, s, p := testService(t)
	if _, e := svc.Upload(context.Background(), "u", "a.png", brokenReader{}, 0); e == nil {
		t.Fatal("expected read error")
	}
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("storage"))
	if _, e := svc.Upload(context.Background(), "u", "a.png", bytes.NewReader(imageBytes()), 0); e == nil {
		t.Fatal("expected storage error")
	}
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	r.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("database"))
	s.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
	if _, e := svc.Upload(context.Background(), "u", "a.png", bytes.NewReader(imageBytes()), 0); e == nil {
		t.Fatal("expected repo error")
	}
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	r.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(errors.New("broker"))
	if _, e := svc.Upload(context.Background(), "u", "a.png", bytes.NewReader(imageBytes()), 0); e == nil {
		t.Fatal("expected broker error")
	}
	r.EXPECT().Get(gomock.Any(), "missing").Return(nil, domain.ErrNotFound)
	if e := svc.Delete(context.Background(), "missing", "u"); e != domain.ErrNotFound {
		t.Fatalf("got %v", e)
	}
	if e := svc.Delete(context.Background(), "missing", ""); e != ErrEmptyUserID {
		t.Fatalf("got %v", e)
	}
}
