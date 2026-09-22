package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("S3_USE_SSL", "true")
	c := Load()
	if c.HTTPAddr != ":9999" || !c.S3UseSSL {
		t.Fatalf("unexpected config: %#v", c)
	}
	_ = os.Unsetenv("S3_USE_SSL")
	if getBool("S3_USE_SSL", false) {
		t.Fatal("default ignored")
	}
	t.Setenv("S3_USE_SSL", "broken")
	if !getBool("S3_USE_SSL", true) {
		t.Fatal("invalid bool should return default")
	}
}
