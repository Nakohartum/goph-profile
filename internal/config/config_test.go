package config

import (
	"os"
	"testing"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("RABBITMQ_URL", "amqp://test")
}

func TestLoad(t *testing.T) {
	setRequired(t)
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("S3_USE_SSL", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":9999" || !c.S3UseSSL {
		t.Fatalf("unexpected config: %#v", c)
	}
	_ = os.Unsetenv("HTTP_ADDR")
	if got := get("HTTP_ADDR", ":8080"); got != ":8080" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("HTTP_ADDR", "")
	if got := get("HTTP_ADDR", ":8080"); got != "" {
		t.Fatalf("empty value was replaced: %q", got)
	}
}

func TestLoadRejectsMissingOrInvalidValues(t *testing.T) {
	setRequired(t)
	t.Setenv("S3_SECRET_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected required variable error")
	}
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_USE_SSL", "broken")
	if _, err := Load(); err == nil {
		t.Fatal("expected boolean parse error")
	}
}
