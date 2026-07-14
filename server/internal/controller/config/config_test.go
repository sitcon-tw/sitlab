package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadUsesPrefixedEnvironment(t *testing.T) {
	t.Setenv("PROJECT_TEMPLATE_HTTP_ADDR", ":9090")
	t.Setenv("HTTP_ADDR", ":9999")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("HTTP addr = %q", cfg.HTTP.Addr)
	}
}

func TestLocalUsesDevelopmentDatabaseAndCSRFDefaults(t *testing.T) {
	t.Setenv("PROJECT_TEMPLATE_ENV", "local")
	unsetEnvironmentForTest(t, "PROJECT_TEMPLATE_DATABASE_URL")
	unsetEnvironmentForTest(t, "PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://project:project@localhost:5432/project?sslmode=disable" {
		t.Fatalf("database URL = %q", cfg.DatabaseURL)
	}
	if len(cfg.HTTP.AllowedOrigins) != 2 {
		t.Fatalf("allowed origins = %#v", cfg.HTTP.AllowedOrigins)
	}
}

func TestProductionRequiresExplicitDatabaseAndCSRFOrigins(t *testing.T) {
	tests := []struct {
		name           string
		databaseURL    string
		allowedOrigins string
		want           string
	}{
		{"database", "", "https://app.example.com", "PROJECT_TEMPLATE_DATABASE_URL"},
		{"csrf origins", "postgres://app:secret@db:5432/app?sslmode=require", "", "PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidProductionEnv(t)
			t.Setenv("PROJECT_TEMPLATE_DATABASE_URL", tt.databaseURL)
			t.Setenv("PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS", tt.allowedOrigins)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "explicitly set in production") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProductionRejectsMissingDatabaseAndCSRFOrigins(t *testing.T) {
	for _, key := range []string{"PROJECT_TEMPLATE_DATABASE_URL", "PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS"} {
		t.Run(key, func(t *testing.T) {
			setValidProductionEnv(t)
			unsetEnvironmentForTest(t, key)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProductionLoadsWithExplicitSecurityConfiguration(t *testing.T) {
	setValidProductionEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL == "" || len(cfg.HTTP.AllowedOrigins) != 1 || !cfg.Session.CookieSecure {
		t.Fatalf("unexpected production config: %#v", cfg)
	}
}

func TestProductionRejectsCSRFOriginListWithoutAnOrigin(t *testing.T) {
	setValidProductionEnv(t)
	t.Setenv("PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS", ", ,")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must contain at least one origin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProductionMigrationRequiresExplicitDatabaseURL(t *testing.T) {
	t.Setenv("PROJECT_TEMPLATE_ENV", "production")
	t.Setenv("PROJECT_TEMPLATE_DATABASE_URL", "")
	_, err := LoadDatabaseURL()
	if err == nil || !strings.Contains(err.Error(), "PROJECT_TEMPLATE_DATABASE_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProductionRejectsDevelopmentSessionKey(t *testing.T) {
	setValidProductionEnv(t)
	t.Setenv("PROJECT_TEMPLATE_SESSION_HASH_KEY", "local-development-session-hash-key-change-me")
	_, err := Load()
	if err == nil {
		t.Fatal("expected production validation error")
	}
}

func TestProductionSecretValidationDoesNotLeakSecret(t *testing.T) {
	secret := "short-private-value"
	setValidProductionEnv(t)
	t.Setenv("PROJECT_TEMPLATE_SESSION_HASH_KEY", secret)
	_, err := Load()
	if err == nil {
		t.Fatal("expected short secret validation error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("validation error leaked secret: %v", err)
	}
}

func setValidProductionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PROJECT_TEMPLATE_ENV", "production")
	t.Setenv("PROJECT_TEMPLATE_DATABASE_URL", "postgres://app:secret@db:5432/app?sslmode=require")
	t.Setenv("PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("PROJECT_TEMPLATE_SESSION_COOKIE_SECURE", "true")
	t.Setenv("PROJECT_TEMPLATE_SESSION_HASH_KEY", "01234567890123456789012345678901")
}

func unsetEnvironmentForTest(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "temporary")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
