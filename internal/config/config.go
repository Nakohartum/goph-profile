package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr, DatabaseURL, S3Endpoint, S3AccessKey, S3SecretKey, S3Bucket, RabbitURL string
	OTLPEndpoint, LogLevel                                                           string
	S3UseSSL                                                                         bool
}

func Load() (Config, error) {
	databaseURL, err := required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	s3AccessKey, err := required("S3_ACCESS_KEY")
	if err != nil {
		return Config{}, err
	}
	s3SecretKey, err := required("S3_SECRET_KEY")
	if err != nil {
		return Config{}, err
	}
	rabbitURL, err := required("RABBITMQ_URL")
	if err != nil {
		return Config{}, err
	}
	s3UseSSL, err := getBool("S3_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}
	return Config{HTTPAddr: get("HTTP_ADDR", ":8080"), DatabaseURL: databaseURL, S3Endpoint: get("S3_ENDPOINT", "localhost:9000"), S3AccessKey: s3AccessKey, S3SecretKey: s3SecretKey, S3Bucket: get("S3_BUCKET", "avatars"), RabbitURL: rabbitURL, S3UseSSL: s3UseSSL, OTLPEndpoint: get("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger:4318"), LogLevel: get("LOG_LEVEL", "INFO")}, nil
}
func get(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return d
}
func required(k string) (string, error) {
	v, ok := os.LookupEnv(k)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("required environment variable %s is not set", k)
	}
	return v, nil
}
func getBool(k string, d bool) (bool, error) {
	v, ok := os.LookupEnv(k)
	if !ok {
		return d, nil
	}
	b, e := strconv.ParseBool(v)
	if e != nil {
		return false, fmt.Errorf("parse %s: %w", k, e)
	}
	return b, nil
}
