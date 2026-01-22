package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

// App holds the runtime configuration loaded from environment variables.
type App struct {
	Env             string
	HTTPPort        string
	GRPCPort        string
	DatabaseURL     string
	RedisAddr       string
	JWTIssuer       string
	JWTSigningKey   string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	FaceServiceURL  string
	FaceSkip        bool
	QueueBackend    string
	RateLimitPerMin int
}

// Load returns application config populated from environment variables with sensible defaults.
func Load() App {
	return App{
		Env:             getEnv("APP_ENV", "dev"),
		HTTPPort:        getEnv("HTTP_PORT", "8081"),
		GRPCPort:        getEnv("GRPC_PORT", "9090"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://attendance:attendance@localhost:5433/attendance?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		JWTIssuer:       getEnv("JWT_ISSUER", "attendance-engine"),
		JWTSigningKey:   getEnv("JWT_SIGNING_KEY", "dev-signing-secret-change"),
		AccessTTL:       durationEnv("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:      durationEnv("REFRESH_TTL", 24*time.Hour),
		FaceServiceURL:  getEnv("FACE_SERVICE_URL", "http://localhost:8000"),
		FaceSkip:        boolEnv("FACE_SKIP", true),
		QueueBackend:    getEnv("QUEUE_BACKEND", "redis"),
		RateLimitPerMin: intEnv("RATE_LIMIT_PER_MIN", 120),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		d, err := time.ParseDuration(val)
		if err != nil {
			log.Printf("invalid duration for %s: %v, using fallback %s", key, err, fallback)
			return fallback
		}
		return d
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if val == "1" || val == "true" || val == "TRUE" {
			return true
		}
		if val == "0" || val == "false" || val == "FALSE" {
			return false
		}
		log.Printf("invalid bool for %s, using fallback %v", key, fallback)
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		var parsed int
		if _, err := fmt.Sscanf(val, "%d", &parsed); err == nil {
			return parsed
		}
		log.Printf("invalid int for %s, using fallback %d", key, fallback)
	}
	return fallback
}
