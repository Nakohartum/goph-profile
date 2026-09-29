package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"goph-profile/internal/broker"
	"goph-profile/internal/config"
	"goph-profile/internal/domain"
	"goph-profile/internal/observability"
	"goph-profile/internal/repository"
	"goph-profile/internal/storage"
	workerapp "goph-profile/internal/worker"
)

func main() {
	var eventsReady atomic.Bool
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	must(err)
	logger := observability.NewLogger("goph-profile-worker", cfg.LogLevel)
	slog.SetDefault(logger)
	shutdownTracing, err := observability.Setup(ctx, "goph-profile-worker", cfg.OTLPEndpoint)
	must(err)
	defer observability.Shutdown(context.Background(), shutdownTracing, logger)
	metrics, err := observability.NewMetrics(prometheus.DefaultRegisterer)
	must(err)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	metricsMux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !eventsReady.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	metricsServer := &http.Server{Addr: ":9091", Handler: metricsMux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := metricsServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("metrics server failed", "error", serveErr)
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()
	repo, err := repository.NewPostgres(ctx, cfg.DatabaseURL, metrics)
	must(err)
	defer repo.Close()
	objects, err := storage.NewMinIO(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	must(err)
	events, err := broker.NewRabbit(cfg.RabbitURL, metrics)
	must(err)
	defer events.Close()
	deliveries, err := events.Consume()
	must(err)
	eventsReady.Store(true)
	processor := workerapp.NewProcessor(repo, objects, metrics)
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			messageCtx := broker.ExtractContext(ctx, d.Headers)
			var event domain.ProcessEvent
			if err = json.Unmarshal(d.Body, &event); err != nil {
				observability.WithTrace(messageCtx, logger).Error("invalid event", "error", err)
				_ = d.Reject(false)
				continue
			}
			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = processor.Handle(messageCtx, event)
				if processErr == nil {
					break
				}
				if attempt < 2 && !waitForRetry(ctx, time.Duration(1<<attempt)*time.Second) {
					return
				}
			}
			if processErr != nil {
				observability.WithTrace(messageCtx, logger).Error("processing failed", "avatar_id", event.AvatarID, "error", processErr)
				_ = d.Nack(false, true)
			} else {
				_ = d.Ack(false)
			}
		}
	}
}
func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
func must(err error) {
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}
