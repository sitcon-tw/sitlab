package cardactivity

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
)

type repositoryFake struct{ err error }

func (f repositoryFake) Card(context.Context, int64) (board.Card, error) {
	if f.err != nil {
		return board.Card{}, f.err
	}
	return board.Card{IssueIID: 42}, nil
}

type gitLabFake struct {
	labels   []ProjectLabel
	comments []Comment
	created  Comment
	issueIID int64
	body     string
	token    string
	err      error
}

func (f *gitLabFake) ProjectLabels(context.Context) ([]ProjectLabel, error) { return f.labels, f.err }
func (f *gitLabFake) Comments(_ context.Context, issueIID int64, token string) ([]Comment, error) {
	f.issueIID, f.token = issueIID, token
	return f.comments, f.err
}
func (f *gitLabFake) CreateComment(_ context.Context, issueIID int64, body, token string) (Comment, error) {
	f.issueIID, f.body, f.token = issueIID, body, token
	return f.created, f.err
}

type actorFake struct{ err error }

func (f actorFake) AccessToken(context.Context, string) (string, error) { return "actor-token", f.err }

func TestCommentsUseActorTokenAndSortOldestFirst(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{comments: []Comment{
		{ID: 2, CreatedAt: time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC), System: true},
		{ID: 1, CreatedAt: time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)},
	}}
	service := NewService(repositoryFake{}, gitlab, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	comments, err := service.Comments(context.Background(), "actor", 42)
	if err != nil || len(comments) != 2 || comments[0].ID != 1 || gitlab.issueIID != 42 || gitlab.token != "actor-token" {
		t.Fatalf("Comments() = %#v, %v, call=%#v", comments, err, gitlab)
	}
}

func TestCreateCommentValidatesAndKeepsBody(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{created: Comment{ID: 9, Body: "  review\n"}}
	service := NewService(repositoryFake{}, gitlab, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	comment, err := service.CreateComment(context.Background(), CreateCommentInput{ActorUserID: "actor", IssueIID: 42, Body: "  review\n"})
	if err != nil || comment.ID != 9 || gitlab.body != "  review\n" {
		t.Fatalf("CreateComment() = %#v, %v, body=%q", comment, err, gitlab.body)
	}
	_, err = service.CreateComment(context.Background(), CreateCommentInput{ActorUserID: "actor", IssueIID: 42, Body: " \n "})
	assertKind(t, err, apperror.KindInvalid)
}

func TestCommentsMapMissingCardAndOAuthFailure(t *testing.T) {
	t.Parallel()
	service := NewService(repositoryFake{err: board.ErrCardNotFound}, &gitLabFake{}, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	_, err := service.Comments(context.Background(), "actor", 42)
	assertKind(t, err, apperror.KindNotFound)
	service = NewService(repositoryFake{}, &gitLabFake{}, actorFake{err: errors.New("expired")}, noop.NewTracerProvider().Tracer("test"))
	_, err = service.Comments(context.Background(), "actor", 42)
	assertKind(t, err, apperror.KindUnauthorized)
}

func assertKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != kind {
		t.Fatalf("error = %#v, want kind %s", err, kind)
	}
}
