package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goph-profile/internal/broker"
	"goph-profile/internal/config"
	"goph-profile/internal/domain"
	"goph-profile/internal/repository"
	"goph-profile/internal/storage"
	workerapp "goph-profile/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	must(err)
	repo, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
	must(err)
	defer repo.Close()
	objects, err := storage.NewMinIO(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	must(err)
	events, err := broker.NewRabbit(cfg.RabbitURL)
	must(err)
	defer events.Close()
	deliveries, err := events.Consume()
	must(err)
	processor := workerapp.NewProcessor(repo, objects)
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			var event domain.ProcessEvent
			if err = json.Unmarshal(d.Body, &event); err != nil {
				slog.Error("invalid event", "error", err)
				_ = d.Reject(false)
				continue
			}
			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = processor.Handle(ctx, event)
				if processErr == nil {
					break
				}
				if attempt < 2 && !waitForRetry(ctx, time.Duration(1<<attempt)*time.Second) {
					return
				}
			}
			if processErr != nil {
				slog.Error("processing failed", "avatar_id", event.AvatarID, "error", processErr)
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
