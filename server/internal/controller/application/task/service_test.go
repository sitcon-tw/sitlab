package task

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/task"
	"example.com/project-template/internal/domain/workspace"
)

const (
	testWorkspaceID = "10000000-0000-0000-0000-000000000001"
	testActorID     = "10000000-0000-0000-0000-000000000002"
	testAssigneeID  = "10000000-0000-0000-0000-000000000003"
)

type taskRepoFake struct {
	createCalls int
	getCalls    int
	updateCalls int
	deleteCalls int
}

func (f *taskRepoFake) Create(_ context.Context, item domain.Task) (domain.Task, error) {
	f.createCalls++
	return item, nil
}
func (f *taskRepoFake) Get(context.Context, string, string) (domain.Task, error) {
	f.getCalls++
	return domain.Task{}, domain.ErrNotFound
}
func (*taskRepoFake) List(context.Context, string, *domain.Status) ([]domain.Task, error) {
	return nil, nil
}
func (f *taskRepoFake) Update(_ context.Context, item domain.Task) (domain.Task, error) {
	f.updateCalls++
	return item, nil
}
func (f *taskRepoFake) Delete(context.Context, string, string) error {
	f.deleteCalls++
	return nil
}

type permissionsFake struct{}

func (permissionsFake) GetMemberRole(_ context.Context, _ string, userID string) (workspace.Role, error) {
	if userID == testActorID {
		return workspace.RoleEditor, nil
	}
	return "", workspace.ErrMemberNotFound
}

func TestCreateRejectsAssigneeOutsideWorkspace(t *testing.T) {
	repo := &taskRepoFake{}
	service := NewService(repo, permissionsFake{}, noop.NewTracerProvider().Tracer("test"))
	_, err := service.Create(context.Background(), CreateInput{ActorUserID: testActorID, WorkspaceID: testWorkspaceID, Title: "Review contract", AssigneeUserID: pointer(testAssigneeID)})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != "TASK_ASSIGNEE_NOT_MEMBER" || appErr.Kind != apperror.KindInvalid {
		t.Fatalf("unexpected error: %#v", err)
	}
	if repo.createCalls != 0 {
		t.Fatal("task persisted before assignee membership was verified")
	}
}

type viewerPermissionsFake struct{}

func (viewerPermissionsFake) GetMemberRole(context.Context, string, string) (workspace.Role, error) {
	return workspace.RoleViewer, nil
}

func TestViewerCannotWriteTasks(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *Service) error
	}{
		{
			name: "create",
			run: func(ctx context.Context, service *Service) error {
				_, err := service.Create(ctx, CreateInput{ActorUserID: testActorID, WorkspaceID: testWorkspaceID, Title: "Read only"})
				return err
			},
		},
		{
			name: "update",
			run: func(ctx context.Context, service *Service) error {
				title := "Still read only"
				_, err := service.Update(ctx, UpdateInput{ActorUserID: testActorID, WorkspaceID: testWorkspaceID, TaskID: testAssigneeID, Title: &title})
				return err
			},
		},
		{
			name: "delete",
			run: func(ctx context.Context, service *Service) error {
				return service.Delete(ctx, testWorkspaceID, testAssigneeID, testActorID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &taskRepoFake{}
			service := NewService(repo, viewerPermissionsFake{}, noop.NewTracerProvider().Tracer("test"))
			err := tt.run(context.Background(), service)
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Kind != apperror.KindForbidden || appErr.Code != "INSUFFICIENT_ROLE" {
				t.Fatalf("unexpected error: %#v", err)
			}
			if repo.createCalls+repo.getCalls+repo.updateCalls+repo.deleteCalls != 0 {
				t.Fatalf("repository called before authorization: %#v", repo)
			}
		})
	}
}

func pointer(value string) *string { return &value }
