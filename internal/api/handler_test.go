package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"goph-profile/internal/domain"
	"goph-profile/internal/mocks"
	"goph-profile/internal/service"
)

func testHandler(t *testing.T, state string) (http.Handler, map[string]*domain.Avatar) {
	t.Helper()
	c := gomock.NewController(t)
	r := mocks.NewMockAvatarRepository(c)
	s := mocks.NewMockObjectStorage(c)
	p := mocks.NewMockEventPublisher(c)
	items := map[string]*domain.Avatar{}
	objects := map[string][]byte{}
	r.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, a *domain.Avatar) error { items[a.ID] = a; return nil }).AnyTimes()
	r.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id string) (*domain.Avatar, error) {
		a, ok := items[id]
		if !ok {
			return nil, domain.ErrNotFound
		}
		return a, nil
	}).AnyTimes()
	r.EXPECT().ListByUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u string) ([]domain.Avatar, error) {
		out := []domain.Avatar{}
		for _, a := range items {
			if a.UserID == u {
				out = append(out, *a)
			}
		}
		return out, nil
	}).AnyTimes()
	r.EXPECT().SoftDelete(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id string) error {
		if _, ok := items[id]; !ok {
			return domain.ErrNotFound
		}
		delete(items, id)
		return nil
	}).AnyTimes()
	s.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, k string, rd io.Reader, _ int64, _ string) error {
		objects[k], _ = io.ReadAll(rd)
		return nil
	}).AnyTimes()
	s.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, k string) (io.ReadCloser, string, error) {
		b, ok := objects[k]
		if !ok {
			return nil, "", domain.ErrNotFound
		}
		return io.NopCloser(bytes.NewReader(b)), "image/png", nil
	}).AnyTimes()
	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return NewHandler(service.NewAvatarService(r, s, p), func() map[string]string { return map[string]string{"database": state} }), items
}
func TestHealthAndPages(t *testing.T) {
	h, _ := testHandler(t, "up")
	for _, path := range []string{"/health", "/web/upload", "/web/gallery/user"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	h, _ = testHandler(t, "down")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 503 {
		t.Fatalf("health: %d", w.Code)
	}
}
func TestUploadListGetDelete(t *testing.T) {
	h, items := testHandler(t, "up")
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "avatar.png")
	_, _ = part.Write(imageData.Bytes())
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/avatars", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-User-ID", "user")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var id string
	for key := range items {
		id = key
	}
	for _, path := range []string{"/api/v1/avatars/" + id, "/api/v1/avatars/" + id + "/metadata", "/api/v1/users/user/avatars", "/api/v1/users/user/avatar"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	req = httptest.NewRequest("DELETE", "/api/v1/avatars/"+id, nil)
	req.Header.Set("X-User-ID", "wrong")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("forbidden: %d", w.Code)
	}
	req = httptest.NewRequest("DELETE", "/api/v1/avatars/"+id, nil)
	req.Header.Set("X-User-ID", "user")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
}
func TestErrors(t *testing.T) {
	h, _ := testHandler(t, "up")
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"GET", "/api/v1/avatars/missing", 404}, {"GET", "/api/v1/avatars/x?size=bad", 400}, {"POST", "/api/v1/avatars", 413}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("bad"))
		if tc.method == "POST" {
			req.Header.Set("Content-Type", "multipart/form-data")
		}
		h.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%s: got %d want %d", tc.path, w.Code, tc.code)
		}
	}
}
func TestDeleteLatest(t *testing.T) {
	h, items := testHandler(t, "up")
	items["id"] = &domain.Avatar{ID: "id", UserID: "user"}
	req := httptest.NewRequest("DELETE", "/api/v1/users/user/avatar", nil)
	req.Header.Set("X-User-ID", "other")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
	req = httptest.NewRequest("DELETE", "/api/v1/users/user/avatar", nil)
	req.Header.Set("X-User-ID", "user")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("got %d", w.Code)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/api/v1/users/user/avatar", nil)
	req.Header.Set("X-User-ID", "user")
	h.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("got %d", w.Code)
	}
}
