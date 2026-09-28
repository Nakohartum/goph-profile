package observability

import (
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	HTTPRequests       *prometheus.CounterVec
	HTTPDuration       *prometheus.HistogramVec
	Uploads            *prometheus.CounterVec
	UploadDuration     *prometheus.HistogramVec
	UploadedBytes      prometheus.Counter
	Processing         *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
	QueuePublished     *prometheus.CounterVec
	QueueDepth         prometheus.Gauge
	DBConnections      *prometheus.GaugeVec
}

func NewMetrics(registerer prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		HTTPRequests:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_http_requests_total", Help: "HTTP requests."}, []string{"service", "method", "route", "status"}),
		HTTPDuration:       prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"service", "method", "route", "status"}),
		Uploads:            prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_avatar_uploads_total", Help: "Avatar uploads."}, []string{"status"}),
		UploadDuration:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_avatar_upload_duration_seconds", Help: "Avatar upload duration."}, []string{"status"}),
		UploadedBytes:      prometheus.NewCounter(prometheus.CounterOpts{Name: "gophprofile_avatar_uploaded_bytes_total", Help: "Total bytes successfully uploaded to object storage."}),
		Processing:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_avatar_processing_total", Help: "Avatar processing results."}, []string{"action", "status"}),
		ProcessingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gophprofile_avatar_processing_duration_seconds", Help: "Worker processing duration."}, []string{"action", "status"}),
		QueuePublished:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gophprofile_queue_messages_published_total", Help: "Published RabbitMQ messages."}, []string{"status"}),
		QueueDepth:         prometheus.NewGauge(prometheus.GaugeOpts{Name: "gophprofile_queue_depth", Help: "RabbitMQ queue depth observed on publish/consume."}),
		DBConnections:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gophprofile_db_connections", Help: "PostgreSQL connection pool state."}, []string{"state"}),
	}
	collectors := []prometheus.Collector{m.HTTPRequests, m.HTTPDuration, m.Uploads, m.UploadDuration, m.UploadedBytes, m.Processing, m.ProcessingDuration, m.QueuePublished, m.QueueDepth, m.DBConnections}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("register metrics: %w", err)
		}
	}
	return m, nil
}

func (m *Metrics) ObserveHTTP(service, method, route string, status int, started time.Time) {
	s := strconv.Itoa(status)
	m.HTTPRequests.WithLabelValues(service, method, route, s).Inc()
	m.HTTPDuration.WithLabelValues(service, method, route, s).Observe(time.Since(started).Seconds())
}
