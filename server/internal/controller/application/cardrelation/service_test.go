package cardrelation

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
)

type repositoryFake struct {
	card board.Card
	err  error
}

func (f repositoryFake) Card(context.Context, int64) (board.Card, error) {
	return f.card, f.err
}

type actorFake struct{ err error }

func (f actorFake) AccessToken(context.Context, string) (string, error) {
	return "actor-token", f.err
}

type relationshipFake struct {
	pageQuery      PageQuery
	candidateQuery CandidateQuery
	kind           RelationshipKind
	parentID       int64
	targetID       int64
	targetIDs      []int64
	title          string
	linkType       LinkType
	token          string
	err            error
}

func (f *relationshipFake) ChildItems(_ context.Context, parentID int64, query PageQuery, token string) (ChildPage, error) {
	f.parentID, f.pageQuery, f.token = parentID, query, token
	return ChildPage{}, f.err
}

func (f *relationshipFake) LinkedItems(_ context.Context, sourceID int64, query PageQuery, token string) (LinkedPage, error) {
	f.parentID, f.pageQuery, f.token = sourceID, query, token
	return LinkedPage{}, f.err
}

func (f *relationshipFake) RelationshipCandidates(
	_ context.Context,
	sourceID int64,
	kind RelationshipKind,
	query CandidateQuery,
	token string,
) ([]WorkItem, error) {
	f.parentID, f.kind, f.candidateQuery, f.token = sourceID, kind, query, token
	return nil, f.err
}

func (f *relationshipFake) CreateChild(_ context.Context, parentID int64, title, token string) (WorkItem, error) {
	f.parentID, f.title, f.token = parentID, title, token
	return WorkItem{GitLabWorkItemID: 99, Title: title}, f.err
}

func (f *relationshipFake) AttachChild(_ context.Context, parentID, childID int64, token string) error {
	f.parentID, f.targetID, f.token = parentID, childID, token
	return f.err
}

func (f *relationshipFake) DetachChild(_ context.Context, parentID, childID int64, token string) error {
	f.parentID, f.targetID, f.token = parentID, childID, token
	return f.err
}

func (f *relationshipFake) AddLinks(_ context.Context, sourceID int64, targetIDs []int64, linkType LinkType, token string) error {
	f.parentID, f.targetIDs, f.linkType, f.token = sourceID, targetIDs, linkType, token
	return f.err
}

func (f *relationshipFake) RemoveLink(_ context.Context, sourceID, targetID int64, token string) error {
	f.parentID, f.targetID, f.token = sourceID, targetID, token
	return f.err
}

func newTestService(relations *relationshipFake) *Service {
	workItemID := int64(9001)
	return New(Dependencies{
		Cards:  repositoryFake{card: board.Card{IssueIID: 42, GitLabIssueID: &workItemID}},
		Reader: relations, Children: relations, Links: relations, Actors: actorFake{},
		Tracer: noop.NewTracerProvider().Tracer("test"),
	})
}

func TestService_ChildItemsUsesActorAndDefaultPage(t *testing.T) {
	t.Parallel()
	relations := &relationshipFake{}
	page, err := newTestService(relations).ChildItems(context.Background(), "actor", 42, PageQuery{})
	if err != nil {
		t.Fatalf("ChildItems() error = %v", err)
	}
	if page.Items == nil || relations.parentID != 9001 || relations.pageQuery.Limit != 50 || relations.token != "actor-token" {
		t.Fatalf("ChildItems() page = %#v, call = %#v", page, relations)
	}
}

func TestService_SearchNormalizesTitleAndIID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		query    string
		expected CandidateQuery
	}{
		{name: "title", query: "  metrics  ", expected: CandidateQuery{Text: "metrics"}},
		{name: "iid", query: "#203", expected: CandidateQuery{IID: int64Pointer(203)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			relations := &relationshipFake{}
			_, err := newTestService(relations).Search(context.Background(), SearchInput{
				ActorUserID: "actor", IssueIID: 42, Kind: RelationshipKindChild, Query: test.query,
			})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if relations.candidateQuery.Text != test.expected.Text || pointerValue(relations.candidateQuery.IID) != pointerValue(test.expected.IID) {
				t.Fatalf("candidate query = %#v, want %#v", relations.candidateQuery, test.expected)
			}
		})
	}

	relations := &relationshipFake{}
	_, err := newTestService(relations).Search(context.Background(), SearchInput{
		ActorUserID: "actor", IssueIID: 42, Kind: RelationshipKindChild, Query: "x",
	})
	assertApplicationError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
}

func TestService_CreateChildTrimsTitle(t *testing.T) {
	t.Parallel()
	relations := &relationshipFake{}
	item, err := newTestService(relations).CreateChild(context.Background(), CreateChildInput{
		ActorUserID: "actor", IssueIID: 42, Title: "  add metrics  ",
	})
	if err != nil || item.GitLabWorkItemID != 99 || relations.title != "add metrics" {
		t.Fatalf("CreateChild() = %#v, %v, call = %#v", item, err, relations)
	}
}

func TestService_AttachChildRejectsSelfAndMapsConflict(t *testing.T) {
	t.Parallel()
	relations := &relationshipFake{}
	service := newTestService(relations)
	err := service.AttachChild(context.Background(), ChildRelationInput{
		ActorUserID: "actor", IssueIID: 42, WorkItemID: 9001,
	})
	assertApplicationError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")

	relations.err = ErrRelationConflict
	err = service.AttachChild(context.Background(), ChildRelationInput{
		ActorUserID: "actor", IssueIID: 42, WorkItemID: 9203,
	})
	assertApplicationError(t, err, apperror.KindConflict, "RELATION_CONFLICT")
}

func TestService_AddLinksMapsUnavailableTier(t *testing.T) {
	t.Parallel()
	relations := &relationshipFake{err: ErrFeatureUnavailable}
	err := newTestService(relations).AddLinks(context.Background(), LinkInput{
		ActorUserID: "actor", IssueIID: 42, WorkItemIDs: []int64{9203, 9204}, LinkType: LinkTypeBlocks,
	})
	assertApplicationError(t, err, apperror.KindInvalid, "GITLAB_FEATURE_UNAVAILABLE")
	if relations.linkType != LinkTypeBlocks || len(relations.targetIDs) != 2 {
		t.Fatalf("link call = %#v", relations)
	}
}

func TestService_AddLinksRejectsInvalidBatches(t *testing.T) {
	t.Parallel()
	service := newTestService(&relationshipFake{})
	for _, workItemIDs := range [][]int64{nil, {9203, 9203}, {0}} {
		err := service.AddLinks(context.Background(), LinkInput{
			ActorUserID: "actor", IssueIID: 42, WorkItemIDs: workItemIDs, LinkType: LinkTypeRelatesTo,
		})
		assertApplicationError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
	}
}

func TestService_RequiresSyncedCard(t *testing.T) {
	t.Parallel()
	relations := &relationshipFake{}
	service := New(Dependencies{
		Cards:  repositoryFake{card: board.Card{IssueIID: 42}},
		Reader: relations, Children: relations, Links: relations, Actors: actorFake{},
		Tracer: noop.NewTracerProvider().Tracer("test"),
	})
	_, err := service.ChildItems(context.Background(), "actor", 42, PageQuery{})
	assertApplicationError(t, err, apperror.KindConflict, "RELATION_CONFLICT")

	service = New(Dependencies{
		Cards:  repositoryFake{err: board.ErrCardNotFound},
		Reader: relations, Children: relations, Links: relations, Actors: actorFake{},
		Tracer: noop.NewTracerProvider().Tracer("test"),
	})
	_, err = service.ChildItems(context.Background(), "actor", 42, PageQuery{})
	assertApplicationError(t, err, apperror.KindNotFound, "NOT_FOUND")
}

func assertApplicationError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Kind != kind || applicationError.Code != code {
		t.Fatalf("error = %#v, want kind %s code %s", err, kind, code)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func pointerValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
