package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goph-profile/internal/api"
	"goph-profile/internal/broker"
	"goph-profile/internal/config"
	"goph-profile/internal/repository"
	"goph-profile/internal/service"
	"goph-profile/internal/storage"
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
	svc := service.NewAvatarService(repo, objects, events)
	logger := slog.Default()
	handler := api.NewHandler(svc, logger, func() map[string]string {
		out := map[string]string{"database": "up", "s3": "up", "broker": "up"}
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if repo.Ping(c) != nil {
			out["database"] = "down"
		}
		if objects.Ping(c) != nil {
			out["s3"] = "down"
		}
		if !events.Healthy() {
			out["broker"] = "down"
		}
		return out
	})
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("server started", "addr", cfg.HTTPAddr)
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			slog.Error("server failed", "error", e)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := srv.Shutdown(shutdown); e != nil {
		slog.Error("shutdown failed", "error", e)
	}
}
func must(err error) {
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}
