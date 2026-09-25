package api

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"goph-profile/internal/domain"
)

func testHandler(t *testing.T, state string) (http.Handler, *MockAvatarService, *bytes.Buffer) {
	t.Helper()
	service := NewMockAvatarService(gomock.NewController(t))
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := NewHandler(service, logger, func() map[string]string { return map[string]string{"database": state} })
	return handler, service, &logs
}

func TestHealthAndUploadPage(t *testing.T) {
	h, _, _ := testHandler(t, "up")
	for _, path := range []string{"/health", "/web/upload"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	h, _, _ = testHandler(t, "down")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("health: %d", w.Code)
	}
}

func TestUploadGetMetadataListAndDelete(t *testing.T) {
	h, service, _ := testHandler(t, "up")
	avatar := &domain.Avatar{ID: "id", UserID: "user", FileName: "avatar.png", MIMEType: "image/png", ProcessingStatus: domain.StatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	service.EXPECT().Upload(gomock.Any(), "user", "avatar.png", gomock.Any(), int64(3)).Return(avatar, nil)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "avatar.png")
	_, _ = part.Write([]byte("png"))
	_ = form.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-User-ID", "user")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	service.EXPECT().Get(gomock.Any(), "id", "").Return(avatar, io.NopCloser(strings.NewReader("image")), "image/png", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/avatars/id", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d", w.Code)
	}
	service.EXPECT().Metadata(gomock.Any(), "id").Return(avatar, nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/avatars/id/metadata", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("metadata: %d", w.Code)
	}
	service.EXPECT().List(gomock.Any(), "user").Return([]domain.Avatar{*avatar}, nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users/user/avatars", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	service.EXPECT().Delete(gomock.Any(), "id", "user").Return(nil)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/id", nil)
	req.Header.Set("X-User-ID", "user")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestInternalErrorIsLoggedAndHidden(t *testing.T) {
	h, service, logs := testHandler(t, "up")
	service.EXPECT().Metadata(gomock.Any(), "id").Return(nil, errors.New("database password leaked"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/avatars/id/metadata", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "password") || !strings.Contains(w.Body.String(), http.StatusText(http.StatusInternalServerError)) {
		t.Fatalf("unsafe response: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "database password leaked") {
		t.Fatalf("error was not logged: %s", logs.String())
	}
}

func TestGalleryEscapesFileName(t *testing.T) {
	h, service, _ := testHandler(t, "up")
	service.EXPECT().List(gomock.Any(), "user").Return([]domain.Avatar{{ID: "id", FileName: `<script>alert(1)</script>`}}, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/web/gallery/user", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `<script>alert(1)</script>`) {
		t.Fatalf("filename was not escaped: %s", w.Body.String())
	}
}

func TestValidationAndDeleteLatest(t *testing.T) {
	h, service, _ := testHandler(t, "up")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/avatars/id?size=bad", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("size: %d", w.Code)
	}
	service.EXPECT().List(gomock.Any(), "user").Return([]domain.Avatar{{ID: "id", UserID: "user"}}, nil)
	service.EXPECT().Delete(gomock.Any(), "id", "user").Return(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user/avatar", nil)
	req.Header.Set("X-User-ID", "user")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete latest: %d", w.Code)
	}
}
