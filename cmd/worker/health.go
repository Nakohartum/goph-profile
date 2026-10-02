package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type pingable interface {
	Ping(context.Context) error
}

type brokerHealth interface {
	Healthy() bool
}

type workerReadiness struct {
	repository pingable
	storage    pingable
	broker     brokerHealth
	ready      atomic.Bool
}

func newWorkerReadiness(ctx context.Context, repository, storage pingable, broker brokerHealth, deliveries <-chan amqp091.Delivery) *workerReadiness {
	r := &workerReadiness{repository: repository, storage: storage, broker: broker}
	r.ready.Store(deliveries != nil)
	go func() {
		<-ctx.Done()
		r.stop()
	}()
	return r
}

func (r *workerReadiness) stop() {
	r.ready.Store(false)
}

func (r *workerReadiness) handler(w http.ResponseWriter, _ *http.Request) {
	components := map[string]string{"database": "up", "s3": "up", "broker": "up", "deliveries": "up"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if r.repository.Ping(ctx) != nil {
		components["database"] = "down"
	}
	if r.storage.Ping(ctx) != nil {
		components["s3"] = "down"
	}
	if !r.broker.Healthy() {
		components["broker"] = "down"
		r.stop()
	}
	if !r.ready.Load() {
		components["deliveries"] = "down"
	}
	status := http.StatusOK
	for _, state := range components {
		if state != "up" {
			status = http.StatusServiceUnavailable
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "components": components})
}

func liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
