package bootstrap

import (
	"context"
	"testing"
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type authFake struct{}

func (authFake) IssueCSRF(context.Context, identity.SessionClaims) (string, error) {
	return "csrf", nil
}
func (authFake) Me(context.Context, string) (identity.User, error) {
	return identity.User{ID: "user", GitLabUserID: 123}, nil
}

type directoryFake struct{}

func (directoryFake) Snapshot(context.Context) (directory.Snapshot, error) {
	return directory.Snapshot{Teams: []directory.Team{{Key: "development"}}}, nil
}
func (directoryFake) Preferences(context.Context, string) (appdirectory.Preferences, error) {
	key := "development"
	return appdirectory.Preferences{DefaultTeamKey: &key}, nil
}

type boardFake struct{}

func (boardFake) Board(context.Context) (appboard.Snapshot, error) {
	return appboard.Snapshot{}, nil
}

type syncFake struct{}

func (syncFake) Status(context.Context) (SyncStatus, error) {
	return SyncStatus{State: "synced", LastSuccessAt: time.Unix(1, 0)}, nil
}

func TestGetAggregatesFirstRenderAndIssuesCSRF(t *testing.T) {
	t.Parallel()
	service := NewService(authFake{}, directoryFake{}, boardFake{}, syncFake{})
	result, err := service.Get(context.Background(), identity.SessionClaims{UserID: "user", SessionID: "session"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.CSRFToken != "csrf" || result.Me.GitLabUserID != 123 || len(result.Directory.Teams) != 1 || result.Preferences.DefaultTeamKey == nil || result.Sync.State != "synced" {
		t.Fatalf("Get() = %#v", result)
	}
}
