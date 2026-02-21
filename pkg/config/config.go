// Package config provides application configuration loaded from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	// Server
	ServerPort string

	// Database
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Kafka
	KafkaBrokers []string

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// Rate Limiting
	RateLimit float64 // requests per second
	RateBurst int

	// Service Ports (for gateway proxying)
	AuthServiceAddr         string
	UserServiceAddr         string
	RideServiceAddr         string
	LocationServiceAddr     string
	PaymentServiceAddr      string
	NotificationServiceAddr string
	// Edge
	EnableEdgeProxy bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://rideshare:rideshare_secret@localhost:5433/rideshare?sslmode=disable"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		KafkaBrokers: getEnvSlice("KAFKA_BROKERS", "localhost:9092", ","),

		JWTSecret:     getEnv("JWT_SECRET", "rideshare-dev-secret-change-in-prod"),
		JWTExpiration: getEnvDuration("JWT_EXPIRATION", 24*time.Hour),

		RateLimit: getEnvFloat("RATE_LIMIT", 100),
		RateBurst: getEnvInt("RATE_BURST", 200),

		EnableEdgeProxy: getEnvBool("ENABLE_EDGE_PROXY", false),

		AuthServiceAddr:         getEnv("AUTH_SERVICE_ADDR", "localhost:8081"),
		UserServiceAddr:         getEnv("USER_SERVICE_ADDR", "localhost:8082"),
		RideServiceAddr:         getEnv("RIDE_SERVICE_ADDR", "localhost:8083"),
		LocationServiceAddr:     getEnv("LOCATION_SERVICE_ADDR", "localhost:8084"),
		PaymentServiceAddr:      getEnv("PAYMENT_SERVICE_ADDR", "localhost:8085"),
		NotificationServiceAddr: getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:8086"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvSlice(key, fallback, sep string) []string {
	v := os.Getenv(key)
	if v == "" {
		v = fallback
	}
	// Split and trim spaces
	parts := strings.Split(v, sep)
	var res []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
