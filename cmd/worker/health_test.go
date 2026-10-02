package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rabbitmq/amqp091-go"
)

type healthPing struct {
	err error
}

func (p healthPing) Ping(context.Context) error { return p.err }

type healthBroker struct {
	healthy bool
}

func (b healthBroker) Healthy() bool { return b.healthy }

func TestWorkerReadiness(t *testing.T) {
	deliveries := make(chan amqp091.Delivery)
	ctx, cancel := context.WithCancel(context.Background())
	r := newWorkerReadiness(ctx, healthPing{}, healthPing{}, healthBroker{healthy: true}, deliveries)

	recorder := httptest.NewRecorder()
	r.handler(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("healthy worker returned %d", recorder.Code)
	}

	r.repository = healthPing{err: errors.New("database unavailable")}
	recorder = httptest.NewRecorder()
	r.handler(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("degraded worker returned %d", recorder.Code)
	}

	r.repository = healthPing{}
	r.broker = healthBroker{healthy: false}
	recorder = httptest.NewRecorder()
	r.handler(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable || r.ready.Load() {
		t.Fatal("worker remained ready after broker disconnect")
	}

	cancel()
	r.stop()
	if r.ready.Load() {
		t.Fatal("worker remained ready after shutdown")
	}
}
