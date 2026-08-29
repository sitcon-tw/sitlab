package config

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func validWebhookToken() string {
	return "whsec_" + base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestLocalDefaultsUseFourteenDayRollingSession(t *testing.T) {
	t.Setenv("SITCON_BOARD_ENV", "local")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Session.TTL != 14*24*time.Hour || cfg.Directory.FilePath != LocalDirectoryPath || cfg.ServiceName != "sitcon-board-controller" ||
		cfg.Sync.BoardInterval != 10*time.Second || cfg.Sync.BoardPresenceInterval != 5*time.Minute ||
		cfg.Sync.BoardDeepInterval != time.Hour || cfg.Sync.BoardDeltaOverlap != 2*time.Minute {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestProductionRequiresGitLabAndSecureKeys(t *testing.T) {
	t.Setenv("SITCON_BOARD_ENV", "production")
	t.Setenv("SITCON_BOARD_DATABASE_URL", "postgres://user:password@db.example/sitcon")
	t.Setenv("SITCON_BOARD_CSRF_ALLOWED_ORIGINS", "https://board.sitcon.org")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GitLab OAuth") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestProjectCannotBeConfiguredByClientOrEnvironment(t *testing.T) {
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_PATH", "other/project")
	if ProjectPath != "sitcon-tw/2027" || DirectoryFilePath != ".sitcon/board-directory.yml" {
		t.Fatalf("fixed sources changed: %s %s", ProjectPath, DirectoryFilePath)
	}
}

func TestWebhookSigningTokenMustEncodeThirtyTwoBytes(t *testing.T) {
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_WEBHOOK_SIGNING_TOKEN", "whsec_invalid")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "32-byte key") {
		t.Fatalf("Load() error = %v", err)
	}
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_WEBHOOK_SIGNING_TOKEN", validWebhookToken())
	if _, err := Load(); err != nil {
		t.Fatalf("Load() with valid signing token error = %v", err)
	}
}

func TestProductionRequiresHTTPSAndMatchingRedirectOrigin(t *testing.T) {
	t.Setenv("SITCON_BOARD_ENV", "production")
	t.Setenv("SITCON_BOARD_DATABASE_URL", "postgres://user:password@db.example/sitcon")
	t.Setenv("SITCON_BOARD_SESSION_HASH_KEY", strings.Repeat("a", 64))
	t.Setenv("SITCON_BOARD_OAUTH_STATE_CIPHER_KEY", strings.Repeat("b", 64))
	t.Setenv("SITCON_BOARD_GITLAB_CLIENT_ID", "client")
	t.Setenv("SITCON_BOARD_GITLAB_CLIENT_SECRET", "secret")
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_ACCESS_TOKEN", "project-token")
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_WEBHOOK_SIGNING_TOKEN", webhookToken(1))
	t.Setenv("SITCON_BOARD_GITLAB_GROUP_WEBHOOK_SIGNING_TOKEN", webhookToken(2))
	t.Setenv("SITCON_BOARD_CSRF_ALLOWED_ORIGINS", "https://board.sitcon.org")
	t.Setenv("SITCON_BOARD_GITLAB_OAUTH_REDIRECT_URL", "https://board.sitcon.org/api/v1/auth/gitlab/callback")

	t.Setenv("SITCON_BOARD_GITLAB_BASE_URL", "http://gitlab.example")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("insecure GitLab base error = %v", err)
	}
	t.Setenv("SITCON_BOARD_GITLAB_BASE_URL", "https://gitlab.example")
	t.Setenv("SITCON_BOARD_GITLAB_OAUTH_REDIRECT_URL", "https://other.example/api/v1/auth/gitlab/callback")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "must be a CSRF allowed origin") {
		t.Fatalf("mismatched redirect origin error = %v", err)
	}
	t.Setenv("SITCON_BOARD_GITLAB_OAUTH_REDIRECT_URL", "https://board.sitcon.org/api/v1/auth/gitlab/callback")
	t.Setenv("SITCON_BOARD_CSRF_ALLOWED_ORIGINS", "http://board.sitcon.org")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "allowed origins must use HTTPS") {
		t.Fatalf("insecure CSRF origin error = %v", err)
	}
}

func webhookToken(seed byte) string {
	return "whsec_" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
}
