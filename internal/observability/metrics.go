package observability

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequests       = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_http_requests_total", Help: "HTTP requests."}, []string{"service", "method", "route", "status"})
	HTTPDuration       = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"service", "method", "route", "status"})
	Uploads            = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_avatar_uploads_total", Help: "Avatar uploads."}, []string{"status"})
	UploadDuration     = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_avatar_upload_duration_seconds", Help: "Avatar upload duration."}, []string{"status"})
	StorageBytes       = prometheus.NewGauge(prometheus.GaugeOpts{Name: "gophprofile_avatar_storage_bytes", Help: "Bytes successfully uploaded to object storage."})
	Processing         = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_avatar_processing_total", Help: "Avatar processing results."}, []string{"action", "status"})
	ProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_avatar_processing_duration_seconds", Help: "Worker processing duration."}, []string{"action", "status"})
	QueuePublished     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_queue_messages_published_total", Help: "Published RabbitMQ messages."}, []string{"status"})
	QueueDepth         = prometheus.NewGauge(prometheus.GaugeOpts{Name: "gophprofile_queue_depth", Help: "RabbitMQ queue depth observed on publish/consume."})
	DBConnections      = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gophprofile_db_connections", Help: "PostgreSQL connection pool state."}, []string{"state"})
)

func init() {
	prometheus.MustRegister(HTTPRequests, HTTPDuration, Uploads, UploadDuration, StorageBytes, Processing, ProcessingDuration, QueuePublished, QueueDepth, DBConnections)
}

func ObserveHTTP(service, method, route string, status int, started time.Time) {
	s := strconv.Itoa(status)
	HTTPRequests.WithLabelValues(service, method, route, s).Inc()
	HTTPDuration.WithLabelValues(service, method, route, s).Observe(time.Since(started).Seconds())
}
