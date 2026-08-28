package cardrelation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/identity"
)

const (
	defaultPageLimit int32 = 50
	maxPageLimit     int32 = 100
	maxLinkBatch           = 20
	maxTitleLength         = 255
)

var iidQueryPattern = regexp.MustCompile(`^#?([0-9]+)$`)

type Dependencies struct {
	Cards    CardRepository
	Reader   RelationshipReader
	Children ChildWriter
	Links    LinkWriter
	Actors   ActorTokens
	Tracer   trace.Tracer
}

type Service struct {
	cards    CardRepository
	reader   RelationshipReader
	children ChildWriter
	links    LinkWriter
	actors   ActorTokens
	tracer   trace.Tracer
}

func New(dependencies Dependencies) *Service {
	return &Service{
		cards: dependencies.Cards, reader: dependencies.Reader,
		children: dependencies.Children, links: dependencies.Links,
		actors: dependencies.Actors, tracer: dependencies.Tracer,
	}
}

func (s *Service) ChildItems(
	ctx context.Context,
	actorUserID string,
	issueIID int64,
	query PageQuery,
) (ChildPage, error) {
	ctx, span := s.tracer.Start(ctx, "card_relation.list_children")
	defer span.End()
	workItemID, token, err := s.authorize(ctx, actorUserID, issueIID)
	if err != nil {
		return ChildPage{}, err
	}
	query, err = normalizePageQuery(query)
	if err != nil {
		return ChildPage{}, err
	}
	page, err := s.reader.ChildItems(ctx, workItemID, query, token)
	if err != nil {
		return ChildPage{}, s.mapGitLabError(span, "list GitLab child items", err)
	}
	if page.Items == nil {
		page.Items = []WorkItem{}
	}
	return page, nil
}

func (s *Service) LinkedItems(
	ctx context.Context,
	actorUserID string,
	issueIID int64,
	query PageQuery,
) (LinkedPage, error) {
	ctx, span := s.tracer.Start(ctx, "card_relation.list_links")
	defer span.End()
	workItemID, token, err := s.authorize(ctx, actorUserID, issueIID)
	if err != nil {
		return LinkedPage{}, err
	}
	query, err = normalizePageQuery(query)
	if err != nil {
		return LinkedPage{}, err
	}
	page, err := s.reader.LinkedItems(ctx, workItemID, query, token)
	if err != nil {
		return LinkedPage{}, s.mapGitLabError(span, "list GitLab linked items", err)
	}
	if page.Items == nil {
		page.Items = []LinkedItem{}
	}
	return page, nil
}

func (s *Service) Search(ctx context.Context, input SearchInput) ([]WorkItem, error) {
	ctx, span := s.tracer.Start(ctx, "card_relation.search_candidates")
	defer span.End()
	if input.Kind != RelationshipKindChild && input.Kind != RelationshipKindLinked {
		return nil, apperror.Invalid("VALIDATION_FAILED", "relationship kind is invalid", apperror.Field{
			Name: "query.kind", Code: "INVALID_ENUM", Message: "must be child or linked",
		})
	}
	query, err := normalizeCandidateQuery(input.Query)
	if err != nil {
		return nil, err
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return nil, err
	}
	items, err := s.reader.RelationshipCandidates(ctx, workItemID, input.Kind, query, token)
	if err != nil {
		return nil, s.mapGitLabError(span, "search GitLab relationship candidates", err)
	}
	if items == nil {
		items = []WorkItem{}
	}
	return items, nil
}

func (s *Service) CreateChild(ctx context.Context, input CreateChildInput) (WorkItem, error) {
	ctx, span := s.tracer.Start(ctx, "card_relation.create_child")
	defer span.End()
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return WorkItem{}, apperror.Invalid("VALIDATION_FAILED", "child title is required", apperror.Field{
			Name: "title", Code: "REQUIRED", Message: "must not be empty",
		})
	}
	if len([]rune(title)) > maxTitleLength {
		return WorkItem{}, apperror.Invalid("VALIDATION_FAILED", "child title is too long", apperror.Field{
			Name: "title", Code: "VALUE_TOO_LONG", Message: "must be 255 characters or fewer",
		})
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return WorkItem{}, err
	}
	item, err := s.children.CreateChild(ctx, workItemID, title, token)
	if err != nil {
		return WorkItem{}, s.mapGitLabError(span, "create GitLab child item", err)
	}
	return item, nil
}

func (s *Service) AttachChild(ctx context.Context, input ChildRelationInput) error {
	ctx, span := s.tracer.Start(ctx, "card_relation.attach_child")
	defer span.End()
	if input.WorkItemID <= 0 {
		return invalidWorkItemID()
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return err
	}
	if workItemID == input.WorkItemID {
		return apperror.Invalid("VALIDATION_FAILED", "a card cannot be its own child")
	}
	if err := s.children.AttachChild(ctx, workItemID, input.WorkItemID, token); err != nil {
		return s.mapGitLabError(span, "attach GitLab child item", err)
	}
	return nil
}

func (s *Service) DetachChild(ctx context.Context, input ChildRelationInput) error {
	ctx, span := s.tracer.Start(ctx, "card_relation.detach_child")
	defer span.End()
	if input.WorkItemID <= 0 {
		return invalidWorkItemID()
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return err
	}
	if err := s.children.DetachChild(ctx, workItemID, input.WorkItemID, token); err != nil {
		return s.mapGitLabError(span, "detach GitLab child item", err)
	}
	return nil
}

func (s *Service) AddLinks(ctx context.Context, input LinkInput) error {
	ctx, span := s.tracer.Start(ctx, "card_relation.add_link")
	defer span.End()
	if err := validateLinkTargets(input.WorkItemIDs); err != nil {
		return err
	}
	if !validLinkType(input.LinkType) {
		return apperror.Invalid("VALIDATION_FAILED", "link type is invalid", apperror.Field{
			Name: "linkType", Code: "INVALID_ENUM", Message: "must be relates_to, blocks, or is_blocked_by",
		})
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return err
	}
	for _, targetID := range input.WorkItemIDs {
		if workItemID == targetID {
			return apperror.Invalid("VALIDATION_FAILED", "a card cannot link to itself")
		}
	}
	if err := s.links.AddLinks(ctx, workItemID, input.WorkItemIDs, input.LinkType, token); err != nil {
		return s.mapGitLabError(span, "add GitLab linked items", err)
	}
	return nil
}

func (s *Service) RemoveLink(ctx context.Context, input ChildRelationInput) error {
	ctx, span := s.tracer.Start(ctx, "card_relation.remove_link")
	defer span.End()
	if input.WorkItemID <= 0 {
		return invalidWorkItemID()
	}
	workItemID, token, err := s.authorize(ctx, input.ActorUserID, input.IssueIID)
	if err != nil {
		return err
	}
	if err := s.links.RemoveLink(ctx, workItemID, input.WorkItemID, token); err != nil {
		return s.mapGitLabError(span, "remove GitLab linked item", err)
	}
	return nil
}

func normalizePageQuery(query PageQuery) (PageQuery, error) {
	if query.Limit == 0 {
		query.Limit = defaultPageLimit
	}
	if query.Limit < 1 || query.Limit > maxPageLimit {
		return PageQuery{}, apperror.Invalid("VALIDATION_FAILED", "page limit is invalid", apperror.Field{
			Name: "query.limit", Code: "INVALID_VALUE", Message: "must be between 1 and 100",
		})
	}
	return query, nil
}

func normalizeCandidateQuery(value string) (CandidateQuery, error) {
	value = strings.TrimSpace(value)
	if match := iidQueryPattern.FindStringSubmatch(value); match != nil {
		iid, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || iid <= 0 {
			return CandidateQuery{}, invalidCandidateQuery()
		}
		return CandidateQuery{IID: &iid}, nil
	}
	if len([]rune(value)) < 2 {
		return CandidateQuery{}, invalidCandidateQuery()
	}
	return CandidateQuery{Text: value}, nil
}

func invalidCandidateQuery() error {
	return apperror.Invalid("VALIDATION_FAILED", "relationship search is too short", apperror.Field{
		Name: "query.query", Code: "VALUE_TOO_SHORT", Message: "enter at least 2 characters or a numeric IID",
	})
}

func invalidWorkItemID() error {
	return apperror.Invalid("VALIDATION_FAILED", "work item ID is invalid", apperror.Field{
		Name: "path.workItemId", Code: "INVALID_VALUE", Message: "must be a positive integer",
	})
}

func validateLinkTargets(workItemIDs []int64) error {
	if len(workItemIDs) < 1 || len(workItemIDs) > maxLinkBatch {
		return apperror.Invalid("VALIDATION_FAILED", "linked work item count is invalid", apperror.Field{
			Name: "workItemIds", Code: "INVALID_LENGTH", Message: "must contain between 1 and 20 work item IDs",
		})
	}
	seen := make(map[int64]struct{}, len(workItemIDs))
	for _, workItemID := range workItemIDs {
		if workItemID <= 0 {
			return apperror.Invalid("VALIDATION_FAILED", "linked work item ID is invalid", apperror.Field{
				Name: "workItemIds", Code: "INVALID_VALUE", Message: "must contain only positive integers",
			})
		}
		if _, duplicate := seen[workItemID]; duplicate {
			return apperror.Invalid("VALIDATION_FAILED", "linked work item IDs must be unique", apperror.Field{
				Name: "workItemIds", Code: "DUPLICATE_VALUE", Message: "must not contain duplicate work item IDs",
			})
		}
		seen[workItemID] = struct{}{}
	}
	return nil
}

func validLinkType(linkType LinkType) bool {
	return linkType == LinkTypeRelatesTo || linkType == LinkTypeBlocks || linkType == LinkTypeIsBlockedBy
}

func (s *Service) authorize(ctx context.Context, actorUserID string, issueIID int64) (int64, string, error) {
	if issueIID <= 0 {
		return 0, "", apperror.NotFound("card")
	}
	card, err := s.cards.Card(ctx, issueIID)
	if errors.Is(err, board.ErrCardNotFound) {
		return 0, "", apperror.NotFound("card")
	}
	if err != nil {
		return 0, "", fmt.Errorf("load card for GitLab relationship: %w", err)
	}
	if card.GitLabIssueID == nil || *card.GitLabIssueID <= 0 {
		return 0, "", apperror.Conflict("RELATION_CONFLICT", "card must finish syncing before relationships can be managed")
	}
	if strings.TrimSpace(actorUserID) == "" {
		return 0, "", apperror.Unauthorized("AUTH_INVALID_SESSION", "session user is invalid")
	}
	token, err := s.actors.AccessToken(ctx, actorUserID)
	if err != nil {
		if errors.Is(err, identity.ErrGitLabUnavailable) {
			return 0, "", apperror.UnavailableWithCode("GITLAB_UNAVAILABLE", "GitLab is unavailable")
		}
		return 0, "", apperror.Unauthorized("AUTH_OAUTH_FAILED", "GitLab authorization is unavailable; sign out and sign in again")
	}
	return *card.GitLabIssueID, token, nil
}

func (s *Service) mapGitLabError(span trace.Span, action string, err error) error {
	switch {
	case errors.Is(err, ErrGitLabForbidden):
		return apperror.Forbidden("FORBIDDEN", "GitLab access is required")
	case errors.Is(err, ErrWorkItemNotFound):
		return apperror.NotFoundWithCode("WORK_ITEM_NOT_FOUND", "work item")
	case errors.Is(err, ErrRelationConflict):
		return apperror.Conflict("RELATION_CONFLICT", "the relationship changed in GitLab; refresh and try again")
	case errors.Is(err, ErrInvalidRelation):
		return apperror.Invalid("VALIDATION_FAILED", "GitLab rejected this relationship")
	case errors.Is(err, ErrFeatureUnavailable):
		return apperror.Invalid("GITLAB_FEATURE_UNAVAILABLE", "this GitLab tier does not support blocking relationships")
	case errors.Is(err, identity.ErrGitLabUnavailable):
		return apperror.UnavailableWithCode("GITLAB_UNAVAILABLE", "GitLab is unavailable")
	default:
		span.RecordError(err)
		span.SetStatus(codes.Error, action)
		return fmt.Errorf("%s: %w", action, err)
	}
}
