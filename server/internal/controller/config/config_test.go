package config

import (
	"strings"
	"testing"
	"time"
)

func TestLocalDefaultsUseFourteenDayRollingSession(t *testing.T) {
	t.Setenv("SITCON_BOARD_ENV", "local")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Session.TTL != 14*24*time.Hour || cfg.GitHub.Ref != "main" || cfg.ServiceName != "sitcon-board-controller" {
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

func TestProductionRequiresGitHubDirectoryToken(t *testing.T) {
	t.Setenv("SITCON_BOARD_ENV", "production")
	t.Setenv("SITCON_BOARD_DATABASE_URL", "postgres://user:password@db.example/sitcon")
	t.Setenv("SITCON_BOARD_CSRF_ALLOWED_ORIGINS", "https://board.sitcon.org")
	t.Setenv("SITCON_BOARD_SESSION_HASH_KEY", "production-session-hash-key-value")
	t.Setenv("SITCON_BOARD_OAUTH_STATE_CIPHER_KEY", "production-oauth-cipher-key-value")
	t.Setenv("SITCON_BOARD_GITLAB_CLIENT_ID", "client-id")
	t.Setenv("SITCON_BOARD_GITLAB_CLIENT_SECRET", "client-secret")
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_ACCESS_TOKEN", "project-token")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GitHub directory token") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestProjectCannotBeConfiguredByClientOrEnvironment(t *testing.T) {
	t.Setenv("SITCON_BOARD_GITLAB_PROJECT_PATH", "other/project")
	t.Setenv("SITCON_BOARD_GITHUB_REPOSITORY", "other-repository")
	if ProjectPath != "sitcon-tw/2027" || GitHubOwner != "sitcon-tw" || GitHubRepository != "sitlab" || DirectoryFilePath != ".sitcon/board-directory.yml" {
		t.Fatalf("fixed sources changed: %s %s/%s %s", ProjectPath, GitHubOwner, GitHubRepository, DirectoryFilePath)
	}
}
