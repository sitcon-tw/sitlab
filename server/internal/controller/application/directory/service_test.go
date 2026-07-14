package directory

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/directory"
)

const testUserID = "10000000-0000-0000-0000-000000000001"

type repositoryFake struct {
	snapshot domain.Snapshot
	prefs    Preferences
	setCalls int
	setTeam  string
}

func (f *repositoryFake) Snapshot(context.Context) (domain.Snapshot, error) {
	if len(f.snapshot.Teams) == 0 {
		return domain.Snapshot{}, domain.ErrSnapshotNotFound
	}
	return f.snapshot, nil
}

func (f *repositoryFake) Preferences(context.Context, string) (Preferences, error) {
	if f.prefs.DefaultTeamKey == nil {
		return Preferences{}, domain.ErrPreferencesNotFound
	}
	return f.prefs, nil
}

func (f *repositoryFake) SetPreferences(_ context.Context, _ string, teamKey string, confirmedAt time.Time) (Preferences, error) {
	f.setCalls++
	f.setTeam = teamKey
	return Preferences{DefaultTeamKey: &teamKey, ConfirmedAt: &confirmedAt, DirectoryTeamKeys: []string{"administration"}}, nil
}

func TestPreferencesWithoutSelectionSupportsOnboarding(t *testing.T) {
	t.Parallel()
	service := NewService(&repositoryFake{}, noop.NewTracerProvider().Tracer("test"))
	preferences, err := service.Preferences(context.Background(), testUserID)
	if err != nil {
		t.Fatalf("Preferences() error = %v", err)
	}
	if preferences.DefaultTeamKey != nil || preferences.DirectoryTeamKeys == nil {
		t.Fatalf("Preferences() = %#v", preferences)
	}
}

func TestUpdateValidatesAndPersistsPrimaryTeam(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{snapshot: domain.Snapshot{Teams: []domain.Team{{Key: "development", Active: true}}}}
	service := NewService(repo, noop.NewTracerProvider().Tracer("test"))
	service.now = func() time.Time { return time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC) }

	preferences, err := service.Update(context.Background(), testUserID, "development")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.setCalls != 1 || repo.setTeam != "development" || preferences.DefaultTeamKey == nil || *preferences.DefaultTeamKey != "development" {
		t.Fatalf("Update() = %#v, repository = %#v", preferences, repo)
	}
}

func TestUpdateRejectsUnknownTeam(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{snapshot: domain.Snapshot{Teams: []domain.Team{{Key: "development", Active: true}}}}
	service := NewService(repo, noop.NewTracerProvider().Tracer("test"))
	_, err := service.Update(context.Background(), testUserID, "unknown")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != "TEAM_NOT_FOUND" {
		t.Fatalf("Update() error = %#v", err)
	}
	if repo.setCalls != 0 {
		t.Fatal("unknown team was persisted")
	}
}
