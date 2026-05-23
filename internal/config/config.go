package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for drover-registry.
// Populated from environment variables (DROVER_REGISTRY_* prefix).
type Config struct {
	// Database
	DatabaseURL string

	// Server
	ServerPort int
	ServerHost string

	// Storage backend selection: "local", "s3", "gcs"
	StorageBackend string

	// Local storage (when StorageBackend == "local")
	StorageLocalRoot string

	// S3 / S3-compatible (MinIO)
	S3Bucket          string
	S3Region          string
	S3Endpoint        string // for MinIO or custom endpoint (leave empty for AWS)
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool // true for MinIO

	// GCS
	GCSBucket         string
	GCSProjectID      string
	GCSCredentialsFile string // path to service account JSON (optional in prod if using ADC)

	// Zitadel OIDC / JWT validation
	ZitadelIssuer   string
	ZitadelClientID string // optional, for future audience checks

	// Dev / testing
	EnableDevAuth bool // when true and no Zitadel, accept X-Org-ID header for tenant

	// Webhook to drover-muster
	MusterWebhookURL string
	MusterWebhookSecret string // for HMAC if desired

	// Logging
	LogLevel string

	// Feature flags
	EnableOCI bool // whether to mount OCI distribution compatible routes
}

// LoadConfig reads from environment with sensible defaults for local development.
func LoadConfig() Config {
	return Config{
		DatabaseURL:         getEnv("DROVER_REGISTRY_DB_URL", "postgres://drover:drover_dev_password@localhost:5432/drover_registry?sslmode=disable"),
		ServerPort:          getEnvInt("DROVER_REGISTRY_SERVER_PORT", 8080),
		ServerHost:          getEnv("DROVER_REGISTRY_SERVER_HOST", "0.0.0.0"),
		StorageBackend:      getEnv("DROVER_REGISTRY_STORAGE_BACKEND", "local"),
		StorageLocalRoot:    getEnv("DROVER_REGISTRY_STORAGE_LOCAL_ROOT", "./storage"),
		S3Bucket:            getEnv("DROVER_REGISTRY_S3_BUCKET", "drover-registry-dev"),
		S3Region:            getEnv("DROVER_REGISTRY_S3_REGION", "us-east-1"),
		S3Endpoint:          getEnv("DROVER_REGISTRY_S3_ENDPOINT", "http://localhost:9000"),
		S3AccessKeyID:       getEnv("DROVER_REGISTRY_S3_ACCESS_KEY_ID", "minioadmin"),
		S3SecretAccessKey:   getEnv("DROVER_REGISTRY_S3_SECRET_ACCESS_KEY", "minioadmin"),
		S3UsePathStyle:      getEnvBool("DROVER_REGISTRY_S3_USE_PATH_STYLE", true),
		GCSBucket:           getEnv("DROVER_REGISTRY_GCS_BUCKET", ""),
		GCSProjectID:        getEnv("DROVER_REGISTRY_GCS_PROJECT_ID", ""),
		GCSCredentialsFile:  getEnv("DROVER_REGISTRY_GCS_CREDENTIALS_FILE", ""),
		ZitadelIssuer:       getEnv("DROVER_REGISTRY_ZITADEL_ISSUER", ""),
		ZitadelClientID:     getEnv("DROVER_REGISTRY_ZITADEL_CLIENT_ID", ""),
		EnableDevAuth:       getEnvBool("DROVER_REGISTRY_ENABLE_DEV_AUTH", true),
		MusterWebhookURL:    getEnv("DROVER_REGISTRY_MUSTER_WEBHOOK_URL", "http://localhost:8081/webhooks/registry"),
		MusterWebhookSecret: getEnv("DROVER_REGISTRY_MUSTER_WEBHOOK_SECRET", ""),
		LogLevel:            getEnv("DROVER_REGISTRY_LOG_LEVEL", "info"),
		EnableOCI:           getEnvBool("DROVER_REGISTRY_ENABLE_OCI", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return strings.ToLower(value) == "true" || strings.ToLower(value) == "1"
	}
	return defaultValue
}
