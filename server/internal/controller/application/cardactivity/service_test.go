package cardactivity

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
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
	created  CommentResult
	commands []QuickActionCommand
	issueIID int64
	body     string
	token    string
	err      error
	write    ProjectLabelWrite
	labelID  int64
	deleted  bool
	writeErr error
}

func (f *gitLabFake) ProjectLabels(context.Context) ([]ProjectLabel, error) { return f.labels, f.err }
func (f *gitLabFake) Comments(_ context.Context, issueIID int64, token string) ([]Comment, error) {
	f.issueIID, f.token = issueIID, token
	return f.comments, f.err
}
func (f *gitLabFake) CreateComment(_ context.Context, issueIID int64, body, token string) (CommentResult, error) {
	f.issueIID, f.body, f.token = issueIID, body, token
	return f.created, f.err
}
func (f *gitLabFake) QuickActions(_ context.Context, issueIID int64, token string) ([]QuickActionCommand, error) {
	f.issueIID, f.token = issueIID, token
	return f.commands, f.err
}
func (f *gitLabFake) QuickActionSuggestions(_ context.Context, kind, query string, issueIID int64, token string) ([]QuickActionSuggestion, error) {
	f.issueIID, f.token = issueIID, token
	return []QuickActionSuggestion{{ID: query, Kind: kind, Value: "@alice", Label: "@alice"}}, f.err
}

type actorFake struct{ err error }

func (f actorFake) AccessToken(context.Context, string) (string, error) { return "actor-token", f.err }

func TestCommentsUseActorTokenAndSortOldestFirst(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{comments: []Comment{
		{ID: 2, CreatedAt: time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC), System: true},
		{ID: 1, CreatedAt: time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)},
	}}
	service := NewService(repositoryFake{}, directoryFake{}, gitlab, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	comments, err := service.Comments(context.Background(), "actor", 42)
	if err != nil || len(comments) != 2 || comments[0].ID != 1 || gitlab.issueIID != 42 || gitlab.token != "actor-token" {
		t.Fatalf("Comments() = %#v, %v, call=%#v", comments, err, gitlab)
	}
}

func TestCreateCommentValidatesAndKeepsBody(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{created: CommentResult{Comment: &Comment{ID: 9, Body: "  review\n"}}}
	service := NewService(repositoryFake{}, directoryFake{}, gitlab, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	result, err := service.CreateComment(context.Background(), CreateCommentInput{ActorUserID: "actor", IssueIID: 42, Body: "  review\n"})
	if err != nil || result.Comment == nil || result.Comment.ID != 9 || gitlab.body != "  review\n" {
		t.Fatalf("CreateComment() = %#v, %v, body=%q", result, err, gitlab.body)
	}
	_, err = service.CreateComment(context.Background(), CreateCommentInput{ActorUserID: "actor", IssueIID: 42, Body: " \n "})
	assertKind(t, err, apperror.KindInvalid)
}

func TestCommentsMapMissingCardAndOAuthFailure(t *testing.T) {
	t.Parallel()
	service := NewService(repositoryFake{err: board.ErrCardNotFound}, directoryFake{}, &gitLabFake{}, actorFake{}, noop.NewTracerProvider().Tracer("test"))
	_, err := service.Comments(context.Background(), "actor", 42)
	assertKind(t, err, apperror.KindNotFound)
	service = NewService(repositoryFake{}, directoryFake{}, &gitLabFake{}, actorFake{err: errors.New("expired")}, noop.NewTracerProvider().Tracer("test"))
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

func (f *gitLabFake) CreateProjectLabel(_ context.Context, write ProjectLabelWrite, token string) (ProjectLabel, error) {
	f.write, f.token = write, token
	if f.writeErr != nil {
		return ProjectLabel{}, f.writeErr
	}
	return ProjectLabel{ID: 99, Name: write.Name, Color: write.Color, TextColor: "#FFFFFF", Description: write.Description}, nil
}

func (f *gitLabFake) UpdateProjectLabel(_ context.Context, labelID int64, write ProjectLabelWrite, token string) (ProjectLabel, error) {
	f.labelID, f.write, f.token = labelID, write, token
	if f.writeErr != nil {
		return ProjectLabel{}, f.writeErr
	}
	return ProjectLabel{ID: labelID, Name: write.Name, Color: write.Color, TextColor: "#FFFFFF", Description: write.Description}, nil
}

func (f *gitLabFake) DeleteProjectLabel(_ context.Context, labelID int64, token string) error {
	f.labelID, f.token, f.deleted = labelID, token, true
	return f.writeErr
}

type directoryFake struct {
	teams []directory.Team
	err   error
}

func (f directoryFake) Snapshot(context.Context) (directory.Snapshot, error) {
	return directory.Snapshot{Teams: f.teams}, f.err
}

func labelService(gitlab *gitLabFake) *Service {
	teams := []directory.Team{{Key: "development", GitLabLabel: "Team::開發組"}}
	return NewService(repositoryFake{}, directoryFake{teams: teams}, gitlab, actorFake{}, noop.NewTracerProvider().Tracer("test"))
}

func catalog() []ProjectLabel {
	return []ProjectLabel{
		{ID: 7, Name: "Backend", Color: "#1D76DB"},
		{ID: 8, Name: "Team::開發組", Color: "#0E8A16"},
	}
}

func TestCreateLabelRejectsReservedNamesBeforeCallingGitLab(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Team::開發組", "Team::新組", "Status::Inbox", "To Do", "  "} {
		gitlab := &gitLabFake{labels: catalog()}
		_, err := labelService(gitlab).CreateLabel(context.Background(), CreateLabelInput{ActorUserID: "actor", Name: name, Color: "#123456"})
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Kind != apperror.KindInvalid {
			t.Fatalf("CreateLabel(%q) error = %v, want invalid", name, err)
		}
		if gitlab.write.Name != "" {
			t.Fatalf("CreateLabel(%q) reached GitLab with %#v", name, gitlab.write)
		}
	}
}

func TestCreateLabelRejectsMalformedColor(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{labels: catalog()}
	// GitLab would accept a CSS color name; the contract does not.
	_, err := labelService(gitlab).CreateLabel(context.Background(), CreateLabelInput{ActorUserID: "actor", Name: "Backend", Color: "red"})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindInvalid {
		t.Fatalf("CreateLabel() error = %v, want invalid", err)
	}
}

func TestCreateLabelForwardsTheActorToken(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{labels: catalog()}
	label, err := labelService(gitlab).CreateLabel(context.Background(), CreateLabelInput{ActorUserID: "actor", Name: " Priority::High ", Color: "#D73A4A"})
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if gitlab.token != "actor-token" {
		t.Fatalf("CreateLabel() token = %q, want the actor's", gitlab.token)
	}
	if gitlab.write.Name != "Priority::High" {
		t.Fatalf("CreateLabel() name = %q, want it trimmed", gitlab.write.Name)
	}
	if label.ID != 99 {
		t.Fatalf("CreateLabel() id = %d, want GitLab's", label.ID)
	}
}

func TestCreateLabelMapsConflict(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{labels: catalog(), writeErr: ErrLabelConflict}
	_, err := labelService(gitlab).CreateLabel(context.Background(), CreateLabelInput{ActorUserID: "actor", Name: "Backend", Color: "#1D76DB"})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindConflict {
		t.Fatalf("CreateLabel() error = %v, want conflict", err)
	}
}

func TestUpdateLabelRejectsRenamingAReservedLabel(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{labels: catalog()}
	// Label 8 is Team::開發組; the new name is legal but the existing one is not.
	_, err := labelService(gitlab).UpdateLabel(context.Background(), UpdateLabelInput{ActorUserID: "actor", LabelID: 8, Name: "Renamed", Color: "#123456"})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindInvalid {
		t.Fatalf("UpdateLabel() error = %v, want invalid", err)
	}
	if gitlab.write.Name != "" {
		t.Fatalf("UpdateLabel() reached GitLab with %#v", gitlab.write)
	}
}

func TestUpdateLabelRejectsUnknownID(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{labels: catalog()}
	_, err := labelService(gitlab).UpdateLabel(context.Background(), UpdateLabelInput{ActorUserID: "actor", LabelID: 4242, Name: "Renamed", Color: "#123456"})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("UpdateLabel() error = %v, want not found", err)
	}
}

func TestDeleteLabelRejectsReservedAndDeletesOtherwise(t *testing.T) {
	t.Parallel()
	reserved := &gitLabFake{labels: catalog()}
	if err := labelService(reserved).DeleteLabel(context.Background(), DeleteLabelInput{ActorUserID: "actor", LabelID: 8}); err == nil || reserved.deleted {
		t.Fatalf("DeleteLabel(team label) err = %v, deleted = %v", err, reserved.deleted)
	}
	ordinary := &gitLabFake{labels: catalog()}
	if err := labelService(ordinary).DeleteLabel(context.Background(), DeleteLabelInput{ActorUserID: "actor", LabelID: 7}); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
	if !ordinary.deleted || ordinary.labelID != 7 || ordinary.token != "actor-token" {
		t.Fatalf("DeleteLabel() call = %#v", ordinary)
	}
}
