package main

import (
	"context"
	"testing"
	"time"
)

func TestWaitForRetry(t *testing.T) {
	if !waitForRetry(context.Background(), time.Millisecond) {
		t.Fatal("timer did not fire")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if waitForRetry(ctx, time.Hour) {
		t.Fatal("cancelled context must stop retry")
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("context cancellation was not handled immediately")
	}
}
