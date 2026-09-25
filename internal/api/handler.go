package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"goph-profile/internal/domain"
	"goph-profile/internal/observability"
	"goph-profile/internal/service"
)

type Handler struct {
	service AvatarService
	logger  *slog.Logger
	health  func() map[string]string
}

func NewHandler(s AvatarService, logger *slog.Logger, health func() map[string]string) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{service: s, logger: logger, health: health}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, h.metricsMiddleware, middleware.Compress(5))
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", h.healthcheck)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/avatars", h.upload)
		r.Get("/avatars/{id}", h.get)
		r.Get("/avatars/{id}/metadata", h.metadata)
		r.Delete("/avatars/{id}", h.delete)
		r.Get("/users/{userID}/avatars", h.list)
		r.Get("/users/{userID}/avatar", h.latest)
		r.Delete("/users/{userID}/avatar", h.deleteLatest)
	})
	r.Get("/web/upload", h.uploadPage)
	r.Post("/web/upload", h.upload)
	r.Get("/web/gallery/{userID}", h.galleryPage)
	return otelhttp.NewHandler(r, "http.request")
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (h *Handler) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		observability.ObserveHTTP("goph-profile-server", r.Method, route, status, started)
		observability.Logger(r.Context(), h.logger).Info("http request", "method", r.Method, "route", route, "status", strconv.Itoa(status), "duration_ms", time.Since(started).Milliseconds())
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (h *Handler) writeError(ctx context.Context, w http.ResponseWriter, e error) {
	status := http.StatusInternalServerError
	msg := http.StatusText(http.StatusInternalServerError)
	switch {
	case errors.Is(e, service.ErrEmptyUserID), errors.Is(e, service.ErrEmptyFile), errors.Is(e, service.ErrInvalidFormat):
		status = http.StatusBadRequest
		msg = e.Error()
	case errors.Is(e, service.ErrFileTooLarge):
		status = http.StatusRequestEntityTooLarge
		msg = e.Error()
	case errors.Is(e, domain.ErrNotFound):
		status = http.StatusNotFound
		msg = e.Error()
	case errors.Is(e, domain.ErrForbidden):
		status = http.StatusForbidden
		msg = e.Error()
	}
	if status >= http.StatusInternalServerError {
		// The HTTP middleware adds trace identifiers to the completion log.
		observability.Logger(ctx, h.logger).Error("request failed", "status", status, "error", e)
	}
	writeJSON(w, status, map[string]any{"error": msg})
}
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxUploadSize+(1<<20))
	if e := r.ParseMultipartForm(service.MaxUploadSize); e != nil {
		h.writeError(r.Context(), w, service.ErrFileTooLarge)
		return
	}
	f, head, e := r.FormFile("file")
	if e != nil {
		h.writeError(r.Context(), w, service.ErrEmptyFile)
		return
	}
	defer func() { _ = f.Close() }()
	user := r.Header.Get("X-User-ID")
	if user == "" {
		user = r.FormValue("user_id")
	}
	a, e := h.service.Upload(r.Context(), user, head.Filename, f, head.Size)
	if e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": a.ID, "user_id": a.UserID, "url": "/api/v1/avatars/" + a.ID, "status": a.ProcessingStatus, "created_at": a.CreatedAt})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	size := r.URL.Query().Get("size")
	if size != "" && size != "original" && size != "100x100" && size != "300x300" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid size"})
		return
	}
	a, body, ct, e := h.service.Get(r.Context(), chi.URLParam(r, "id"), size)
	if e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	defer func() { _ = body.Close() }()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("ETag", fmt.Sprintf("\"%s-%s-%d\"", a.ID, size, a.UpdatedAt.Unix()))
	_, _ = io.Copy(w, body)
}
func (h *Handler) metadata(w http.ResponseWriter, r *http.Request) {
	a, e := h.service.Metadata(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, e := h.service.List(r.Context(), chi.URLParam(r, "userID"))
	if e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if e := h.service.Delete(r.Context(), chi.URLParam(r, "id"), r.Header.Get("X-User-ID")); e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	items, e := h.service.List(r.Context(), chi.URLParam(r, "userID"))
	if e != nil || len(items) == 0 {
		if e == nil {
			e = domain.ErrNotFound
		}
		h.writeError(r.Context(), w, e)
		return
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", items[0].ID)
	r = r.WithContext(contextWithRoute(r, rctx))
	h.get(w, r)
}
func contextWithRoute(r *http.Request, rc *chi.Context) context.Context {
	return context.WithValue(r.Context(), chi.RouteCtxKey, rc)
}
func (h *Handler) deleteLatest(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "userID")
	if r.Header.Get("X-User-ID") != user {
		h.writeError(r.Context(), w, domain.ErrForbidden)
		return
	}
	items, e := h.service.List(r.Context(), user)
	if e != nil || len(items) == 0 {
		if e == nil {
			e = domain.ErrNotFound
		}
		h.writeError(r.Context(), w, e)
		return
	}
	if e = h.service.Delete(r.Context(), items[0].ID, user); e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) healthcheck(w http.ResponseWriter, _ *http.Request) {
	components := h.health()
	ok := true
	for _, v := range components {
		if v != "up" {
			ok = false
		}
	}
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"status": map[bool]string{true: "ok", false: "degraded"}[ok], "components": components})
}
func (h *Handler) uploadPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, uploadHTML)
}
func (h *Handler) galleryPage(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "userID")
	items, e := h.service.List(r.Context(), user)
	if e != nil {
		h.writeError(r.Context(), w, e)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if e = galleryTemplate.Execute(w, struct {
		UserID string
		Items  []domain.Avatar
	}{user, items}); e != nil {
		h.logger.Error("render gallery", "error", e)
	}
}

var galleryTemplate = template.Must(template.New("gallery").Parse(`<!doctype html><meta charset="utf-8"><title>GophProfile</title><style>body{font:16px system-ui;max-width:900px;margin:40px auto}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:20px}img{width:100%;aspect-ratio:1;object-fit:cover}</style><h1>Аватарки</h1><a href="/web/upload">Загрузить</a><div id="gallery" class="grid" data-user="{{.UserID}}">{{range .Items}}<article><img src="/api/v1/avatars/{{.ID}}?size=300x300"><p>{{.FileName}}</p><button data-id="{{.ID}}">Удалить</button></article>{{end}}</div><script>document.addEventListener('click',async e=>{if(!e.target.dataset.id)return;const user=document.getElementById('gallery').dataset.user;await fetch('/api/v1/avatars/'+e.target.dataset.id,{method:'DELETE',headers:{'X-User-ID':user}});location.reload()})</script>`))

const uploadHTML = `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>GophProfile</title><style>body{font:16px system-ui;max-width:600px;margin:60px auto;padding:20px}form{display:grid;gap:16px;padding:30px;border:2px dashed #888}input,button{font:inherit;padding:10px}img{max-width:220px;display:none}</style><h1>Загрузка аватарки</h1><form method="post" enctype="multipart/form-data"><label>User ID <input name="user_id" required></label><input id="file" type="file" name="file" accept="image/jpeg,image/png,image/webp" required><img id="preview"><button>Загрузить</button></form><script>file.onchange=()=>{preview.src=URL.createObjectURL(file.files[0]);preview.style.display='block'}</script></html>`
