package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           int
	LogLevel       string
	LogFormat      string
	DBURL          string
	DBMaxConns     int
	JWTSecret      string
	CORSOrigins    []string
	MaxUploadSize  int64
	SessionTTL     time.Duration
	MetricsEnabled bool
	TLSCert        string
	TLSKey         string
	DevMode        bool
	LambdaDemo     bool
	DemoDBPath     string
	StaticDir      string

	// Optional semantic search via pgvector + an external OpenAI-compatible
	// embeddings endpoint. All three are blank by default; the feature is
	// disabled unless EmbeddingAPIURL is set AND the workspace setting
	// search_mode is flipped to "semantic".
	EmbeddingAPIURL string
	EmbeddingAPIKey string
	EmbeddingModel  string
	EmbeddingDim    int
}

func Load() (*Config, error) {
	c := &Config{
		Port:           8080,
		LogLevel:       "info",
		LogFormat:      "text",
		DBURL:          "postgres://lwts:lwts@localhost:5432/lwts?sslmode=disable",
		DBMaxConns:     20,
		CORSOrigins:    []string{"http://localhost:5173"},
		MaxUploadSize:  10485760,
		SessionTTL:     24 * time.Hour,
		MetricsEnabled: false,
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		c.Port = p
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		c.LogFormat = v
	}
	if v := os.Getenv("DB_URL"); v != "" {
		c.DBURL = v
	}
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
		}
		c.DBMaxConns = n
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		c.CORSOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("MAX_UPLOAD_SIZE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid MAX_UPLOAD_SIZE: %w", err)
		}
		c.MaxUploadSize = n
	}
	if v := os.Getenv("SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SESSION_TTL: %w", err)
		}
		c.SessionTTL = d
	}
	if v := os.Getenv("METRICS_ENABLED"); v != "" {
		c.MetricsEnabled = v == "true" || v == "1"
	}
	c.TLSCert = os.Getenv("TLS_CERT")
	c.TLSKey = os.Getenv("TLS_KEY")
	c.DevMode = os.Getenv("DEV") == "true"
	c.LambdaDemo = os.Getenv("LAMBDA_DEMO") == "true" || os.Getenv("LAMBDA_DEMO") == "1"
	c.DemoDBPath = os.Getenv("DEMO_DB_PATH")
	if c.DemoDBPath == "" {
		c.DemoDBPath = "/demo.db"
	}
	c.StaticDir = os.Getenv("STATIC_DIR")
	if c.StaticDir == "" {
		c.StaticDir = "/static"
	}

	c.JWTSecret = os.Getenv("JWT_SECRET")
	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	c.EmbeddingAPIURL = os.Getenv("EMBEDDING_API_URL")
	c.EmbeddingAPIKey = os.Getenv("EMBEDDING_API_KEY")
	c.EmbeddingModel = os.Getenv("EMBEDDING_MODEL")
	c.EmbeddingDim = 384
	if v := os.Getenv("EMBEDDING_DIM"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid EMBEDDING_DIM: %s", v)
		}
		c.EmbeddingDim = n
	}

	return c, nil
}
