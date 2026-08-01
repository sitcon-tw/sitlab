package cardactivity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/identity"
)

type Service struct {
	repo   CardRepository
	gitlab GitLab
	actors ActorTokens
	tracer trace.Tracer
}

func NewService(repo CardRepository, gitlab GitLab, actors ActorTokens, tracer trace.Tracer) *Service {
	return &Service{repo: repo, gitlab: gitlab, actors: actors, tracer: tracer}
}

func (s *Service) Labels(ctx context.Context) ([]ProjectLabel, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.list_labels")
	defer span.End()
	labels, err := s.gitlab.ProjectLabels(ctx)
	if err != nil {
		return nil, s.mapGitLabError(span, "list GitLab project labels", err)
	}
	sort.SliceStable(labels, func(i, j int) bool { return strings.ToLower(labels[i].Name) < strings.ToLower(labels[j].Name) })
	return labels, nil
}

func (s *Service) Comments(ctx context.Context, actorUserID string, issueIID int64) ([]Comment, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.list_comments")
	defer span.End()
	if err := s.requireCard(ctx, issueIID); err != nil {
		return nil, err
	}
	token, err := s.actorToken(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	comments, err := s.gitlab.Comments(ctx, issueIID, token)
	if err != nil {
		return nil, s.mapGitLabError(span, "list GitLab card comments", err)
	}
	sort.SliceStable(comments, func(i, j int) bool {
		if comments[i].CreatedAt.Equal(comments[j].CreatedAt) {
			return comments[i].ID < comments[j].ID
		}
		return comments[i].CreatedAt.Before(comments[j].CreatedAt)
	})
	return comments, nil
}

func (s *Service) CreateComment(ctx context.Context, input CreateCommentInput) (Comment, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.create_comment")
	defer span.End()
	if strings.TrimSpace(input.Body) == "" {
		return Comment{}, apperror.Invalid("VALIDATION_FAILED", "comment body is required", apperror.Field{
			Name: "body", Code: "REQUIRED", Message: "must not be empty",
		})
	}
	if err := s.requireCard(ctx, input.IssueIID); err != nil {
		return Comment{}, err
	}
	token, err := s.actorToken(ctx, input.ActorUserID)
	if err != nil {
		return Comment{}, err
	}
	comment, err := s.gitlab.CreateComment(ctx, input.IssueIID, input.Body, token)
	if err != nil {
		return Comment{}, s.mapGitLabError(span, "create GitLab card comment", err)
	}
	return comment, nil
}

func (s *Service) requireCard(ctx context.Context, issueIID int64) error {
	if issueIID <= 0 {
		return apperror.NotFound("card")
	}
	_, err := s.repo.Card(ctx, issueIID)
	if errors.Is(err, board.ErrCardNotFound) {
		return apperror.NotFound("card")
	}
	if err != nil {
		return fmt.Errorf("load card for GitLab activity: %w", err)
	}
	return nil
}

func (s *Service) actorToken(ctx context.Context, actorUserID string) (string, error) {
	if strings.TrimSpace(actorUserID) == "" {
		return "", apperror.Unauthorized("AUTH_INVALID_SESSION", "session user is invalid")
	}
	token, err := s.actors.AccessToken(ctx, actorUserID)
	if err != nil {
		if errors.Is(err, identity.ErrGitLabUnavailable) {
			return "", apperror.Unavailable("GitLab is unavailable")
		}
		return "", apperror.Unauthorized("AUTH_OAUTH_FAILED", "GitLab authorization is unavailable; sign out and sign in again")
	}
	return token, nil
}

func (s *Service) mapGitLabError(span trace.Span, action string, err error) error {
	switch {
	case errors.Is(err, board.ErrCardNotFound):
		return apperror.NotFound("card")
	case errors.Is(err, ErrGitLabForbidden):
		return apperror.Forbidden("FORBIDDEN", "GitLab access is required")
	case errors.Is(err, identity.ErrGitLabUnavailable):
		return apperror.Unavailable("GitLab is unavailable")
	default:
		span.RecordError(err)
		span.SetStatus(codes.Error, action)
		return fmt.Errorf("%s: %w", action, err)
	}
}
