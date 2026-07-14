package workspace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/domain/identity"
	domain "example.com/project-template/internal/domain/workspace"
)

const ownerID = "20000000-0000-0000-0000-000000000001"

type txMarker struct{}

type txFake struct{ calls int }

func (f *txFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(context.WithValue(ctx, txMarker{}, true))
}

type workspaceRepoFake struct {
	workspaceInTx bool
	memberInTx    bool
	memberErr     error
	workspaceMade bool
}

func (f *workspaceRepoFake) Create(ctx context.Context, item domain.Workspace) (domain.Workspace, error) {
	f.workspaceInTx, _ = ctx.Value(txMarker{}).(bool)
	f.workspaceMade = true
	return item, nil
}
func (f *workspaceRepoFake) CreateMember(ctx context.Context, _ domain.Member) error {
	f.memberInTx, _ = ctx.Value(txMarker{}).(bool)
	return f.memberErr
}

type rollbackTxFake struct{ repo *workspaceRepoFake }

func (f rollbackTxFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	before := f.repo.workspaceMade
	err := fn(context.WithValue(ctx, txMarker{}, true))
	if err != nil {
		f.repo.workspaceMade = before
	}
	return err
}
func (*workspaceRepoFake) GetForUser(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}

func TestCreateWorkspaceRollsBackWhenOwnerInsertFails(t *testing.T) {
	repo := &workspaceRepoFake{memberErr: context.Canceled}
	service := NewService(repo, userLookupFake{}, rollbackTxFake{repo: repo}, noop.NewTracerProvider().Tracer("test"))
	if _, err := service.Create(context.Background(), CreateInput{ActorUserID: ownerID, Name: "Engineering"}); err == nil {
		t.Fatal("expected owner insert failure")
	}
	if repo.workspaceMade {
		t.Fatal("workspace remained after transaction rollback")
	}
}
func (*workspaceRepoFake) ListForUser(context.Context, string) ([]domain.Workspace, error) {
	return nil, nil
}
func (*workspaceRepoFake) Update(_ context.Context, item domain.Workspace) (domain.Workspace, error) {
	return item, nil
}
func (*workspaceRepoFake) Delete(context.Context, string) error         { return nil }
func (*workspaceRepoFake) LockMembership(context.Context, string) error { return nil }
func (*workspaceRepoFake) GetMemberRole(context.Context, string, string) (domain.Role, error) {
	return domain.RoleOwner, nil
}
func (*workspaceRepoFake) ListMembers(context.Context, string) ([]domain.Member, error) {
	return nil, nil
}
func (*workspaceRepoFake) GetMember(context.Context, string, string) (domain.Member, error) {
	return domain.Member{}, nil
}
func (*workspaceRepoFake) CountOwners(context.Context, string) (int, error) { return 2, nil }
func (*workspaceRepoFake) UpdateMemberRole(context.Context, string, string, domain.Role) error {
	return nil
}
func (*workspaceRepoFake) DeleteMember(context.Context, string, string) error { return nil }

type userLookupFake struct{}

func (userLookupFake) FindUserByEmail(context.Context, string) (identity.User, error) {
	return identity.User{}, identity.ErrUserNotFound
}

func TestCreateWorkspaceAndOwnerAreAtomic(t *testing.T) {
	repo := &workspaceRepoFake{}
	tx := &txFake{}
	service := NewService(repo, userLookupFake{}, tx, noop.NewTracerProvider().Tracer("test"))
	created, err := service.Create(context.Background(), CreateInput{ActorUserID: ownerID, Name: "Engineering"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.calls != 1 || !repo.workspaceInTx || !repo.memberInTx {
		t.Fatalf("transaction invariant failed: calls=%d workspace=%v member=%v", tx.calls, repo.workspaceInTx, repo.memberInTx)
	}
	if created.Role != domain.RoleOwner {
		t.Fatalf("created workspace role = %q", created.Role)
	}
}
