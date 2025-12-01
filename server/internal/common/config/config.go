package config

import (
	"os"
	"strconv"
)

type Config struct {
	APIPort            string
	GRPCAPIPort        string
	GRPCAuditPort      string
	GRPCPersistPort    string
	PIIServiceHost     string
	PIIServicePort     string
	AuditServiceHost   string
	AuditServicePort   string
	PersistServiceHost string
	PersistServicePort string
	CacheHost          string
	CachePort          string
	CacheEnabled       bool
	DatabaseURL        string
	AuditDatabaseURL   string // Separate database for audit logs
	PGMQDatabaseURL    string // Separate database for PGMQ
	JWTSecret          string
	Environment        string

	// KEK configuration
	KEKBase64 string
}

func Load() *Config {
	return &Config{
		APIPort:            getEnv("API_PORT", "8080"),
		GRPCAPIPort:        getEnv("GRPC_API_PORT", "9080"),
		GRPCAuditPort:      getEnv("GRPC_AUDIT_PORT", "9081"),
		GRPCPersistPort:    getEnv("GRPC_PERSIST_PORT", "9082"),
		PIIServiceHost:     getEnv("PII_SERVICE_HOST", "localhost"),
		PIIServicePort:     getEnv("PII_SERVICE_PORT", "9080"),
		AuditServiceHost:   getEnv("AUDIT_SERVICE_HOST", "localhost"),
		AuditServicePort:   getEnv("AUDIT_SERVICE_PORT", "9081"),
		PersistServiceHost: getEnv("PERSIST_SERVICE_HOST", "localhost"),
		PersistServicePort: getEnv("PERSIST_SERVICE_PORT", "9082"),
		CacheHost:          getEnv("CACHE_HOST", "localhost"),
		CachePort:          getEnv("CACHE_PORT", "6379"),
		CacheEnabled:       getEnvAsBool("CACHE_ENABLED", true),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pii_vault?sslmode=disable"),
		AuditDatabaseURL:   getEnv("AUDIT_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/audit_logs?sslmode=disable"),
		PGMQDatabaseURL:    getEnv("PGMQ_DATABASE_URL", "postgres://postgres:postgres@localhost:5433/pii_queue?sslmode=disable"),
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key"),
		Environment:        getEnv("ENVIRONMENT", "production"),

		// KEK configuration
		KEKBase64: getEnv("KEK_BASE64", ""),
	}
}

// GetGRPCPort returns the gRPC port for a specific service
func (c *Config) GetGRPCPort(serviceName string) string {
	switch serviceName {
	case "api":
		return c.GRPCAPIPort
	case "audit":
		return c.GRPCAuditPort
	case "persistence":
		return c.GRPCPersistPort
	default:
		return "9000" // Default port
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsBool parses an environment variable as a boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
