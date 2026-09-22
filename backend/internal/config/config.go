// Package config reads and validates every environment-derived setting
// the server needs at startup, in one place, so main.go and other
// packages never call os.Getenv directly.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"smart-grid-asset-management/backend/internal/defined"
)

// DBConfig holds the discrete Postgres connection parameters. Kept as
// separate fields — not a single DATABASE_URL — so each one is legible
// and independently overridable in .env/CI, and a password never needs
// URL-escaping.
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN renders the libpq key=value connection string lib/pq expects.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// Config holds every setting read from the environment. Load it once at
// startup and pass it down — nothing past main() should call os.Getenv.
type Config struct {
	BackendPort string
	DB          DBConfig
	APIKey      string
	CORSOrigins []string // empty = allow any origin (see middleware.CORS)

	// MaxRequestBytes bounds every request body, and therefore the largest
	// CSV upload (see middleware.RequestSizeLimit).
	MaxRequestBytes int64
}

// Load reads and validates configuration from the environment. It fails
// fast on anything required so startup errors surface immediately,
// rather than as a confusing failure several layers down.
func Load() (Config, error) {
	cfg := Config{
		BackendPort: getEnvDefault("BACKEND_PORT", "8080"),
		DB: DBConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     getEnvDefault("DB_PORT", "5432"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
			SSLMode:  getEnvDefault("DB_SSLMODE", "disable"),
		},
		APIKey:      os.Getenv("API_KEY"),
		CORSOrigins: splitAndTrim(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}

	maxBytes, err := parseMaxRequestBytes(os.Getenv("MAX_REQUEST_BYTES"))
	if err != nil {
		return Config{}, err
	}
	cfg.MaxRequestBytes = maxBytes

	required := []struct{ name, value string }{
		{"DB_HOST", cfg.DB.Host},
		{"DB_USER", cfg.DB.User},
		{"DB_PASSWORD", cfg.DB.Password},
		{"DB_NAME", cfg.DB.Name},
	}
	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

// parseMaxRequestBytes parses MAX_REQUEST_BYTES; empty means the default.
func parseMaxRequestBytes(v string) (int64, error) {
	if v == "" {
		return defined.DefaultMaxRequestBytes, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("MAX_REQUEST_BYTES must be a positive integer, got %q", v)
	}
	return n, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitAndTrim splits a comma-separated env value into a trimmed,
// non-empty slice ("" -> nil).
func splitAndTrim(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for part := range strings.SplitSeq(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
