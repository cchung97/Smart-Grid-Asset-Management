package config

import (
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/defined"
)

func setRequiredDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_NAME", "n")
}

func TestLoad_MaxRequestBytes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr string
	}{
		{"unset uses the default", "", defined.DefaultMaxRequestBytes, ""},
		{"explicit value", "2097152", 2097152, ""},
		{"not a number", "ten", 0, "MAX_REQUEST_BYTES must be a positive integer"},
		{"zero", "0", 0, "MAX_REQUEST_BYTES must be a positive integer"},
		{"negative", "-5", 0, "MAX_REQUEST_BYTES must be a positive integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredDBEnv(t)
			t.Setenv("MAX_REQUEST_BYTES", tc.value)
			cfg, err := Load()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.MaxRequestBytes != tc.want {
				t.Errorf("MaxRequestBytes = %d, want %d", cfg.MaxRequestBytes, tc.want)
			}
		})
	}
}

func TestLoad_ReportsEveryMissingRequiredVariable(t *testing.T) {
	for _, k := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		t.Setenv(k, "")
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded without required variables")
	}
	for _, k := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q should name %s", err, k)
		}
	}
}
