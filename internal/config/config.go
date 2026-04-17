package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServiceName string
	Environment string
	LogLevel    string
	RuntimeRole string
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Worker      WorkerConfig
	Security    SecurityConfig
	Webhooks    WebhookConfig
}

type HTTPConfig struct {
	Address      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	URL                string
	MaxConns           int32
	MinConns           int32
	MaxConnIdleTime    time.Duration
	HealthcheckTimeout time.Duration
}

type WorkerConfig struct {
	OutboxPollInterval time.Duration
	BatchSize          int
}

type SecurityConfig struct {
	AdminToken string
}

type WebhookConfig struct {
	MockPaymentSecret string
	TelegramSecret    string
}

func Load() Config {
	return Config{
		ServiceName: getEnv("SERVICE_NAME", "tg-delivery"),
		Environment: getEnv("APP_ENV", "local"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		RuntimeRole: getEnv("APP_ROLE", "api"),
		HTTP: HTTPConfig{
			Address:      getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			URL:                getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tg_delivery?sslmode=disable"),
			MaxConns:           getInt32Env("DB_MAX_CONNS", 25),
			MinConns:           getInt32Env("DB_MIN_CONNS", 2),
			MaxConnIdleTime:    getDurationEnv("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthcheckTimeout: getDurationEnv("DB_HEALTHCHECK_TIMEOUT", 2*time.Second),
		},
		Worker: WorkerConfig{
			OutboxPollInterval: getDurationEnv("WORKER_OUTBOX_POLL_INTERVAL", 2*time.Second),
			BatchSize:          getIntEnv("WORKER_BATCH_SIZE", 100),
		},
		Security: SecurityConfig{
			AdminToken: getEnv("ADMIN_TOKEN", "dev-admin-token"),
		},
		Webhooks: WebhookConfig{
			MockPaymentSecret: getEnv("MOCK_PAYMENT_WEBHOOK_SECRET", "dev-mock-payment-secret"),
			TelegramSecret:    getEnv("TELEGRAM_WEBHOOK_SECRET", "dev-telegram-secret"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getInt32Env(key string, defaultValue int32) int32 {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err == nil {
			return int32(parsed)
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}
