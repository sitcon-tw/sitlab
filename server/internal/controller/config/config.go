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

const (
	prefix            = "SITCON_BOARD_"
	ProjectPath       = "sitcon-tw/2027"
	DirectoryFilePath = ".sitcon/board-directory.yml"
	SessionTTL        = 14 * 24 * time.Hour
)

type Config struct {
	Env             string
	ServiceName     string
	Version         string
	LogLevel        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	HTTP            HTTP
	Session         Session
	GitLab          GitLab
	Sync            Sync
	Observability   Observability
}

type HTTP struct {
	Addr           string
	WebDir         string
	RequestTimeout time.Duration
	AllowedOrigins []string
}

type Session struct {
	CookieName    string
	CookieSecure  bool
	HashKey       string
	CipherKey     string
	TTL           time.Duration
	OAuthStateTTL time.Duration
}

type GitLab struct {
	BaseURL            string
	ClientID           string
	ClientSecret       string
	OAuthRedirectURL   string
	ProjectAccessToken string
	Branch             string
}

type Sync struct {
	DirectoryInterval time.Duration
	BoardInterval     time.Duration
	OperationInterval time.Duration
}

type Observability struct {
	OTLPTracesEndpoint string
}

func Load() (Config, error) {
	env := value("ENV", "local")
	secure, err := boolValue("SESSION_COOKIE_SECURE", env == "production")
	if err != nil {
		return Config{}, err
	}
	cookieDefault := "sitcon_board_session"
	if env == "production" {
		cookieDefault = "__Host-sitcon_board_session"
	}
	cfg := Config{
		Env: env, ServiceName: value("SERVICE_NAME", "sitcon-board-controller"),
		Version: value("VERSION", "0.1.0"), LogLevel: value("LOG_LEVEL", "info"),
		DatabaseURL:     value("DATABASE_URL", "postgres://sitcon:sitcon@localhost:5432/sitcon_board?sslmode=disable"),
		ShutdownTimeout: durationValue("SHUTDOWN_TIMEOUT", 10*time.Second),
		HTTP: HTTP{
			Addr: value("HTTP_ADDR", ":8080"), WebDir: value("WEB_DIR", ""),
			RequestTimeout: durationValue("REQUEST_TIMEOUT", 15*time.Second),
			AllowedOrigins: csvValue("CSRF_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:8080"),
		},
		Session: Session{
			CookieName: cookieDefault, CookieSecure: secure,
			HashKey:   value("SESSION_HASH_KEY", "local-development-session-hash-key-change-me"),
			CipherKey: value("OAUTH_STATE_CIPHER_KEY", "local-development-oauth-cipher-key-change-me"),
			TTL:       SessionTTL, OAuthStateTTL: 10 * time.Minute,
		},
		GitLab: GitLab{
			BaseURL:  value("GITLAB_BASE_URL", "https://gitlab.com"),
			ClientID: value("GITLAB_CLIENT_ID", ""), ClientSecret: value("GITLAB_CLIENT_SECRET", ""),
			OAuthRedirectURL:   value("GITLAB_OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/auth/gitlab/callback"),
			ProjectAccessToken: value("GITLAB_PROJECT_ACCESS_TOKEN", ""), Branch: value("GITLAB_BRANCH", "main"),
		},
		Sync: Sync{
			DirectoryInterval: durationValue("DIRECTORY_SYNC_INTERVAL", 5*time.Minute),
			BoardInterval:     durationValue("BOARD_SYNC_INTERVAL", 30*time.Second),
			OperationInterval: durationValue("OPERATION_POLL_INTERVAL", 500*time.Millisecond),
		},
		Observability: Observability{OTLPTracesEndpoint: value("OTEL_TRACES_ENDPOINT", "")},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadDatabaseURL() (string, error) {
	databaseURL := value("DATABASE_URL", "postgres://sitcon:sitcon@localhost:5432/sitcon_board?sslmode=disable")
	if strings.TrimSpace(databaseURL) == "" {
		return "", errors.New("SITCON_BOARD_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(databaseURL); err != nil {
		return "", fmt.Errorf("SITCON_BOARD_DATABASE_URL is invalid: %w", err)
	}
	return databaseURL, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("SITCON_BOARD_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(c.DatabaseURL); err != nil {
		return fmt.Errorf("SITCON_BOARD_DATABASE_URL is invalid: %w", err)
	}
	if len(c.Session.HashKey) < 32 || len(c.Session.CipherKey) < 32 {
		return errors.New("session hash and OAuth cipher keys must contain at least 32 characters")
	}
	if c.Session.TTL != SessionTTL {
		return errors.New("session TTL must remain 14 days")
	}
	if c.Sync.DirectoryInterval <= 0 || c.Sync.BoardInterval <= 0 || c.Sync.OperationInterval <= 0 {
		return errors.New("sync intervals must be positive")
	}
	for name, raw := range map[string]string{
		"SITCON_BOARD_GITLAB_BASE_URL":           c.GitLab.BaseURL,
		"SITCON_BOARD_GITLAB_OAUTH_REDIRECT_URL": c.GitLab.OAuthRedirectURL,
	} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	for _, raw := range c.HTTP.AllowedOrigins {
		origin, err := url.Parse(raw)
		if err != nil || origin.Scheme == "" || origin.Host == "" || origin.Path != "" {
			return fmt.Errorf("invalid SITCON_BOARD_CSRF_ALLOWED_ORIGINS entry %q", raw)
		}
	}
	if c.Env == "production" {
		if !c.Session.CookieSecure || !strings.HasPrefix(c.Session.CookieName, "__Host-") {
			return errors.New("production session cookie must be Secure and use the __Host- prefix")
		}
		if c.GitLab.ClientID == "" || c.GitLab.ClientSecret == "" || c.GitLab.ProjectAccessToken == "" {
			return errors.New("GitLab OAuth and project access credentials are required in production")
		}
		if c.Session.HashKey == "local-development-session-hash-key-change-me" || c.Session.CipherKey == "local-development-oauth-cipher-key-change-me" {
			return errors.New("development security keys must be changed in production")
		}
	}
	return nil
}

func value(key, fallback string) string {
	if result, ok := os.LookupEnv(prefix + key); ok {
		return strings.TrimSpace(result)
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
