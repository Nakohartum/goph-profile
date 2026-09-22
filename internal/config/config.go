package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr, DatabaseURL, S3Endpoint, S3AccessKey, S3SecretKey, S3Bucket, RabbitURL string
	S3UseSSL                                                                         bool
}

func Load() Config {
	return Config{HTTPAddr: get("HTTP_ADDR", ":8080"), DatabaseURL: get("DATABASE_URL", "postgres://goph:goph@localhost:5432/gophprofile?sslmode=disable"), S3Endpoint: get("S3_ENDPOINT", "localhost:9000"), S3AccessKey: get("S3_ACCESS_KEY", "minioadmin"), S3SecretKey: get("S3_SECRET_KEY", "minioadmin"), S3Bucket: get("S3_BUCKET", "avatars"), RabbitURL: get("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), S3UseSSL: getBool("S3_USE_SSL", false)}
}
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func getBool(k string, d bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	b, e := strconv.ParseBool(v)
	if e != nil {
		return d
	}
	return b
}
