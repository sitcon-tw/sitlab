package directory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/directory"
)

type Service struct {
	repo   Repository
	now    func() time.Time
	tracer trace.Tracer
}

func NewService(repo Repository, tracer trace.Tracer) *Service {
	return &Service{repo: repo, now: time.Now, tracer: tracer}
}

func (s *Service) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	ctx, span := s.tracer.Start(ctx, "directory.snapshot")
	defer span.End()
	snapshot, err := s.repo.Snapshot(ctx)
	if errors.Is(err, domain.ErrSnapshotNotFound) {
		return domain.Snapshot{}, apperror.Unavailable("directory snapshot is not ready")
	}
	if err != nil {
		return domain.Snapshot{}, technical(span, "load directory snapshot", err)
	}
	return snapshot, nil
}

func (s *Service) Preferences(ctx context.Context, userID string) (Preferences, error) {
	ctx, span := s.tracer.Start(ctx, "directory.preferences")
	defer span.End()
	if _, err := uuid.Parse(userID); err != nil {
		return Preferences{}, apperror.Unauthorized("AUTH_INVALID_SESSION", "session user is invalid")
	}
	preferences, err := s.repo.Preferences(ctx, userID)
	if errors.Is(err, domain.ErrPreferencesNotFound) {
		return Preferences{DirectoryTeamKeys: []string{}}, nil
	}
	if err != nil {
		return Preferences{}, technical(span, "load user preferences", err)
	}
	if preferences.DirectoryTeamKeys == nil {
		preferences.DirectoryTeamKeys = []string{}
	}
	return preferences, nil
}

func (s *Service) Update(ctx context.Context, userID, teamKey string) (Preferences, error) {
	ctx, span := s.tracer.Start(ctx, "directory.update_preferences")
	defer span.End()
	if _, err := uuid.Parse(userID); err != nil {
		return Preferences{}, apperror.Unauthorized("AUTH_INVALID_SESSION", "session user is invalid")
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return Preferences{}, err
	}
	if !snapshot.TeamExists(teamKey) {
		return Preferences{}, apperror.Invalid("TEAM_NOT_FOUND", "team does not exist or is inactive", apperror.Field{Name: "defaultTeamKey", Code: "UNKNOWN_TEAM", Message: "must identify an active team"})
	}
	preferences, err := s.repo.SetPreferences(ctx, userID, teamKey, s.now().UTC())
	if err != nil {
		return Preferences{}, technical(span, "store user preferences", err)
	}
	return preferences, nil
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
