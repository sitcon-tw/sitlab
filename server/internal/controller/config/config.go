package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const prefix = "PROJECT_TEMPLATE_"

type Config struct {
	Env             string
	ServiceName     string
	Version         string
	LogLevel        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	HTTP            HTTP
	Session         Session
	Observability   Observability
}

type HTTP struct {
	Addr           string
	WebDir         string
	RequestTimeout time.Duration
	AllowedOrigins []string
}

type Session struct {
	CookieName   string
	CookieSecure bool
	HashKey      string
	IdleTTL      time.Duration
	AbsoluteTTL  time.Duration
	TouchAfter   time.Duration
}

type Observability struct {
	OTLPTracesEndpoint string
}

func Load() (Config, error) {
	env := value("ENV", "local")
	if err := requireProductionValues(env, "DATABASE_URL", "CSRF_ALLOWED_ORIGINS"); err != nil {
		return Config{}, err
	}
	secure, err := boolValue("SESSION_COOKIE_SECURE", env == "production")
	if err != nil {
		return Config{}, err
	}
	cookieDefault := "project_template_session"
	if env == "production" {
		cookieDefault = "__Host-project_template_session"
	}
	cfg := Config{
		Env:             env,
		ServiceName:     value("SERVICE_NAME", "project-template-controller"),
		Version:         value("VERSION", "0.1.0"),
		LogLevel:        value("LOG_LEVEL", "info"),
		DatabaseURL:     value("DATABASE_URL", "postgres://project:project@localhost:5432/project?sslmode=disable"),
		ShutdownTimeout: durationValue("SHUTDOWN_TIMEOUT", 10*time.Second),
		HTTP: HTTP{
			Addr:           value("HTTP_ADDR", ":8080"),
			WebDir:         value("WEB_DIR", ""),
			RequestTimeout: durationValue("REQUEST_TIMEOUT", 15*time.Second),
			AllowedOrigins: csvValue("CSRF_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:8080"),
		},
		Session: Session{
			CookieName:   value("SESSION_COOKIE_NAME", cookieDefault),
			CookieSecure: secure,
			HashKey:      value("SESSION_HASH_KEY", "local-development-session-hash-key-change-me"),
			IdleTTL:      durationValue("SESSION_IDLE_TTL", 12*time.Hour),
			AbsoluteTTL:  durationValue("SESSION_ABSOLUTE_TTL", 7*24*time.Hour),
			TouchAfter:   5 * time.Minute,
		},
		Observability: Observability{OTLPTracesEndpoint: value("OTEL_TRACES_ENDPOINT", "")},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadDatabaseURL() (string, error) {
	env := value("ENV", "local")
	if err := requireProductionValues(env, "DATABASE_URL"); err != nil {
		return "", err
	}
	databaseURL := value("DATABASE_URL", "postgres://project:project@localhost:5432/project?sslmode=disable")
	if strings.TrimSpace(databaseURL) == "" {
		return "", errors.New("PROJECT_TEMPLATE_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(databaseURL); err != nil {
		return "", fmt.Errorf("PROJECT_TEMPLATE_DATABASE_URL is invalid: %w", err)
	}
	return databaseURL, nil
}

func requireProductionValues(env string, keys ...string) error {
	if env != "production" {
		return nil
	}
	for _, key := range keys {
		raw, ok := os.LookupEnv(prefix + key)
		if !ok || strings.TrimSpace(raw) == "" {
			return fmt.Errorf("%s%s must be explicitly set in production", prefix, key)
		}
	}
	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("PROJECT_TEMPLATE_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(c.DatabaseURL); err != nil {
		return fmt.Errorf("PROJECT_TEMPLATE_DATABASE_URL is invalid: %w", err)
	}
	if c.Session.IdleTTL <= 0 || c.Session.AbsoluteTTL < c.Session.IdleTTL {
		return errors.New("session TTLs are invalid")
	}
	if len(c.Session.HashKey) < 32 {
		return errors.New("PROJECT_TEMPLATE_SESSION_HASH_KEY must contain at least 32 characters")
	}
	if c.Env == "production" {
		if !c.Session.CookieSecure {
			return errors.New("PROJECT_TEMPLATE_SESSION_COOKIE_SECURE must be true in production")
		}
		if !strings.HasPrefix(c.Session.CookieName, "__Host-") {
			return errors.New("production session cookie name must use the __Host- prefix")
		}
		if c.Session.HashKey == "local-development-session-hash-key-change-me" {
			return errors.New("PROJECT_TEMPLATE_SESSION_HASH_KEY must be changed in production")
		}
		if len(c.HTTP.AllowedOrigins) == 0 {
			return errors.New("PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS must contain at least one origin in production")
		}
	}
	for _, raw := range c.HTTP.AllowedOrigins {
		origin, err := url.Parse(raw)
		if err != nil || origin.Scheme == "" || origin.Host == "" || origin.Path != "" {
			return fmt.Errorf("invalid PROJECT_TEMPLATE_CSRF_ALLOWED_ORIGINS entry %q", raw)
		}
	}
	return nil
}

func value(key, fallback string) string {
	if value, ok := os.LookupEnv(prefix + key); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func boolValue(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(prefix + key)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s%s must be a boolean", prefix, key)
	}
	return parsed, nil
}

func durationValue(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(prefix + key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return -1
	}
	return parsed
}

func csvValue(key, fallback string) []string {
	raw := value(key, fallback)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, strings.TrimRight(part, "/"))
		}
	}
	return result
}
