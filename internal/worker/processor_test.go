package worker

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

func pngData() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 6)))
	return b.Bytes()
}
func TestProcessAndIdempotency(t *testing.T) {
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	p := NewProcessor(r, s)
	e := domain.ProcessEvent{MessageID: "message", AvatarID: "avatar", S3Key: "original", Action: "process"}
	r.EXPECT().BeginProcessing(gomock.Any(), "avatar", "message").Return(true, nil)
	s.EXPECT().Get(gomock.Any(), "original").Return(io.NopCloser(bytes.NewReader(pngData())), "image/png", nil)
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "image/jpeg").Return(nil).Times(2)
	r.EXPECT().CompleteProcessing(gomock.Any(), "avatar", 8, 6, gomock.Len(2)).Return(nil)
	if err := p.Handle(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	r.EXPECT().BeginProcessing(gomock.Any(), "avatar", "message").Return(false, nil)
	if err := p.Handle(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}
func TestDelete(t *testing.T) {
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	s.EXPECT().Delete(gomock.Any(), "a").Return(nil)
	s.EXPECT().Delete(gomock.Any(), "b").Return(nil)
	if err := NewProcessor(r, s).Handle(context.Background(), domain.ProcessEvent{Action: "delete", S3Keys: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
}
func TestDecodeFailureMarksFailed(t *testing.T) {
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	r.EXPECT().BeginProcessing(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	s.EXPECT().Get(gomock.Any(), "bad").Return(io.NopCloser(bytes.NewReader([]byte("bad"))), "", nil)
	r.EXPECT().FailProcessing(gomock.Any(), gomock.Any()).Return(nil)
	err := NewProcessor(r, s).Handle(context.Background(), domain.ProcessEvent{Action: "process", S3Key: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestDeleteFailure(t *testing.T) {
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	s.EXPECT().Delete(gomock.Any(), "a").Return(errors.New("failed"))
	if err := NewProcessor(r, s).Handle(context.Background(), domain.ProcessEvent{Action: "delete", S3Key: "a"}); err == nil {
		t.Fatal("expected error")
	}
}
