package cardactivity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/identity"
)

type Service struct {
	repo      CardRepository
	directory Directory
	gitlab    GitLab
	actors    ActorTokens
	tracer    trace.Tracer
}

func NewService(repo CardRepository, directory Directory, gitlab GitLab, actors ActorTokens, tracer trace.Tracer) *Service {
	return &Service{repo: repo, directory: directory, gitlab: gitlab, actors: actors, tracer: tracer}
}

var labelColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// validateLabelWrite checks the payload server-side. GitLab also accepts CSS
// color names; the contract does not, so they are rejected here.
func validateLabelWrite(name, color string) error {
	if strings.TrimSpace(name) == "" {
		return apperror.Invalid("VALIDATION_FAILED", "label name is required", apperror.Field{Name: "name", Code: "REQUIRED", Message: "label name is required"})
	}
	if len([]rune(name)) > 255 {
		return apperror.Invalid("VALIDATION_FAILED", "label name is too long", apperror.Field{Name: "name", Code: "VALUE_TOO_LONG", Message: "label name must be 255 characters or fewer"})
	}
	if !labelColorPattern.MatchString(color) {
		return apperror.Invalid("VALIDATION_FAILED", "label color must be a hex value", apperror.Field{Name: "color", Code: "INVALID_FORMAT", Message: "label color must look like #RRGGBB"})
	}
	return nil
}

func reservedLabelError() error {
	return apperror.Invalid("VALIDATION_FAILED", "label name is reserved", apperror.Field{
		Name:    "name",
		Code:    "RESERVED_LABEL",
		Message: "Team and legacy workflow labels are managed by the board directory",
	})
}

func (s *Service) teamLabels(ctx context.Context) ([]string, error) {
	snapshot, err := s.directory.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		if team.GitLabLabel != "" {
			labels = append(labels, team.GitLabLabel)
		}
	}
	return labels, nil
}

// findLabel resolves a label id against the catalog. ProjectLabels already
// filters deprecated names, so a deprecated label's id simply never resolves
// and falls out as NotFound. Do not "fix" that by widening the lookup.
func (s *Service) findLabel(ctx context.Context, labelID int64) (ProjectLabel, error) {
	labels, err := s.gitlab.ProjectLabels(ctx)
	if err != nil {
		return ProjectLabel{}, err
	}
	for _, label := range labels {
		if label.ID == labelID {
			return label, nil
		}
	}
	return ProjectLabel{}, apperror.NotFound("label")
}

func (s *Service) CreateLabel(ctx context.Context, input CreateLabelInput) (ProjectLabel, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.create_label")
	defer span.End()
	if err := validateLabelWrite(input.Name, input.Color); err != nil {
		return ProjectLabel{}, err
	}
	teamLabels, err := s.teamLabels(ctx)
	if err != nil {
		return ProjectLabel{}, err
	}
	if board.ReservedLabel(input.Name, teamLabels) {
		return ProjectLabel{}, reservedLabelError()
	}
	token, err := s.actorToken(ctx, input.ActorUserID)
	if err != nil {
		return ProjectLabel{}, err
	}
	write := ProjectLabelWrite{Name: strings.TrimSpace(input.Name), Color: input.Color, Description: input.Description}
	label, err := s.gitlab.CreateProjectLabel(ctx, write, token)
	if err != nil {
		return ProjectLabel{}, s.mapGitLabError(span, "create GitLab project label", err)
	}
	return label, nil
}

func (s *Service) UpdateLabel(ctx context.Context, input UpdateLabelInput) (ProjectLabel, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.update_label")
	defer span.End()
	if err := validateLabelWrite(input.Name, input.Color); err != nil {
		return ProjectLabel{}, err
	}
	teamLabels, err := s.teamLabels(ctx)
	if err != nil {
		return ProjectLabel{}, err
	}
	if board.ReservedLabel(input.Name, teamLabels) {
		return ProjectLabel{}, reservedLabelError()
	}
	existing, err := s.findLabel(ctx, input.LabelID)
	if err != nil {
		return ProjectLabel{}, err
	}
	// A reserved label may not be renamed even to an otherwise legal new name.
	if board.ReservedLabel(existing.Name, teamLabels) {
		return ProjectLabel{}, reservedLabelError()
	}
	token, err := s.actorToken(ctx, input.ActorUserID)
	if err != nil {
		return ProjectLabel{}, err
	}
	write := ProjectLabelWrite{Name: strings.TrimSpace(input.Name), Color: input.Color, Description: input.Description}
	label, err := s.gitlab.UpdateProjectLabel(ctx, input.LabelID, write, token)
	if err != nil {
		return ProjectLabel{}, s.mapGitLabError(span, "update GitLab project label", err)
	}
	return label, nil
}

func (s *Service) DeleteLabel(ctx context.Context, input DeleteLabelInput) error {
	ctx, span := s.tracer.Start(ctx, "card_activity.delete_label")
	defer span.End()
	teamLabels, err := s.teamLabels(ctx)
	if err != nil {
		return err
	}
	existing, err := s.findLabel(ctx, input.LabelID)
	if err != nil {
		return err
	}
	if board.ReservedLabel(existing.Name, teamLabels) {
		return reservedLabelError()
	}
	token, err := s.actorToken(ctx, input.ActorUserID)
	if err != nil {
		return err
	}
	if err := s.gitlab.DeleteProjectLabel(ctx, input.LabelID, token); err != nil {
		return s.mapGitLabError(span, "delete GitLab project label", err)
	}
	return nil
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

func (s *Service) CreateComment(ctx context.Context, input CreateCommentInput) (CommentResult, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.create_comment")
	defer span.End()
	if strings.TrimSpace(input.Body) == "" {
		return CommentResult{}, apperror.Invalid("VALIDATION_FAILED", "comment body is required", apperror.Field{
			Name: "body", Code: "REQUIRED", Message: "must not be empty",
		})
	}
	if err := s.requireCard(ctx, input.IssueIID); err != nil {
		return CommentResult{}, err
	}
	token, err := s.actorToken(ctx, input.ActorUserID)
	if err != nil {
		return CommentResult{}, err
	}
	result, err := s.gitlab.CreateComment(ctx, input.IssueIID, input.Body, token)
	if err != nil {
		return CommentResult{}, s.mapGitLabError(span, "create GitLab card comment", err)
	}
	return result, nil
}

func (s *Service) QuickActions(ctx context.Context, actorUserID string, issueIID int64) ([]QuickActionCommand, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.quick_actions")
	defer span.End()
	if issueIID > 0 {
		if err := s.requireCard(ctx, issueIID); err != nil {
			return nil, err
		}
	}
	token, err := s.actorToken(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	commands, err := s.gitlab.QuickActions(ctx, issueIID, token)
	if err != nil {
		return nil, s.mapGitLabError(span, "list GitLab quick actions", err)
	}
	return commands, nil
}

func (s *Service) QuickActionSuggestions(ctx context.Context, actorUserID, kind, query string, issueIID int64) ([]QuickActionSuggestion, error) {
	ctx, span := s.tracer.Start(ctx, "card_activity.quick_action_suggestions")
	defer span.End()
	if issueIID > 0 {
		if err := s.requireCard(ctx, issueIID); err != nil {
			return nil, err
		}
	}
	if !validSuggestionKind(kind) {
		return nil, apperror.Invalid("VALIDATION_FAILED", "kind is not a supported GitLab autocomplete source")
	}
	token, err := s.actorToken(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	items, err := s.gitlab.QuickActionSuggestions(ctx, kind, strings.TrimSpace(query), issueIID, token)
	if err != nil {
		return nil, s.mapGitLabError(span, "search GitLab quick action suggestions", err)
	}
	return items, nil
}

func validSuggestionKind(kind string) bool {
	switch kind {
	case "member", "label", "work_item", "merge_request", "epic", "milestone", "iteration", "snippet", "branch", "project":
		return true
	default:
		return false
	}
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
	case errors.Is(err, ErrLabelConflict):
		return apperror.Conflict("CONFLICT", "a project label with that name already exists")
	case errors.Is(err, ErrLabelRejected):
		return apperror.Invalid("VALIDATION_FAILED", "GitLab rejected the project label")
	case errors.Is(err, identity.ErrGitLabUnavailable):
		return apperror.Unavailable("GitLab is unavailable")
	default:
		span.RecordError(err)
		span.SetStatus(codes.Error, action)
		return fmt.Errorf("%s: %w", action, err)
	}
}
