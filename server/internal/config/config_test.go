package config

import (
	"os"
	"testing"
	"time"
)

func clearEnv() {
	for _, key := range []string{
		"PORT", "LOG_LEVEL", "LOG_FORMAT", "DB_URL", "DB_MAX_CONNS",
		"JWT_SECRET", "CORS_ORIGINS", "MAX_UPLOAD_SIZE", "SESSION_TTL",
		"METRICS_ENABLED", "TLS_CERT", "TLS_KEY", "DEV",
	} {
		os.Unsetenv(key)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	clearEnv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	defer clearEnv()

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != 8080 {
		t.Errorf("Port = %d, want 8080", c.Port)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", c.LogLevel)
	}
	if c.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", c.LogFormat)
	}
	if c.DBURL != "postgres://lwts:lwts@localhost:5432/lwts?sslmode=disable" {
		t.Errorf("DBURL = %q", c.DBURL)
	}
	if c.DBMaxConns != 20 {
		t.Errorf("DBMaxConns = %d, want 20", c.DBMaxConns)
	}
	if len(c.CORSOrigins) != 1 || c.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSOrigins = %v", c.CORSOrigins)
	}
	if c.MaxUploadSize != 10485760 {
		t.Errorf("MaxUploadSize = %d", c.MaxUploadSize)
	}
	if c.SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v", c.SessionTTL)
	}
	if c.DevMode {
		t.Error("DevMode should be false by default")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "s3cret")
	os.Setenv("PORT", "9090")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("DB_URL", "postgres://other:other@db:5432/other")
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("CORS_ORIGINS", "http://a.com,http://b.com")
	os.Setenv("MAX_UPLOAD_SIZE", "5000")
	os.Setenv("SESSION_TTL", "2h")
	os.Setenv("METRICS_ENABLED", "true")
	os.Setenv("TLS_CERT", "/cert.pem")
	os.Setenv("TLS_KEY", "/key.pem")
	os.Setenv("DEV", "true")
	defer clearEnv()

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != 9090 {
		t.Errorf("Port = %d", c.Port)
	}
	if c.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", c.LogLevel)
	}
	if c.LogFormat != "json" {
		t.Errorf("LogFormat = %q", c.LogFormat)
	}
	if c.DBMaxConns != 50 {
		t.Errorf("DBMaxConns = %d", c.DBMaxConns)
	}
	if len(c.CORSOrigins) != 2 {
		t.Errorf("CORSOrigins = %v", c.CORSOrigins)
	}
	if c.MaxUploadSize != 5000 {
		t.Errorf("MaxUploadSize = %d", c.MaxUploadSize)
	}
	if c.SessionTTL != 2*time.Hour {
		t.Errorf("SessionTTL = %v", c.SessionTTL)
	}
	if !c.MetricsEnabled {
		t.Error("MetricsEnabled should be true")
	}
	if c.TLSCert != "/cert.pem" {
		t.Errorf("TLSCert = %q", c.TLSCert)
	}
	if !c.DevMode {
		t.Error("DevMode should be true")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "s")
	os.Setenv("PORT", "abc")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT")
	}
}

func TestLoad_InvalidDBMaxConns(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "s")
	os.Setenv("DB_MAX_CONNS", "abc")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DB_MAX_CONNS")
	}
}
