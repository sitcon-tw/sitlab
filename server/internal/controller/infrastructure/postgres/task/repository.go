package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	"example.com/project-template/internal/controller/infrastructure/postgres/sqlc"
	domain "example.com/project-template/internal/domain/task"
)

type Repository struct{ base *sqlc.Queries }

func New(pool *pgxpool.Pool) *Repository { return &Repository{base: sqlc.New(pool)} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return postgres.Queries(ctx, r.base) }

func (r *Repository) Create(ctx context.Context, input domain.Task) (domain.Task, error) {
	row, err := r.q(ctx).CreateTask(ctx, sqlc.CreateTaskParams{ID: uuid.MustParse(input.ID), WorkspaceID: uuid.MustParse(input.WorkspaceID), Title: input.Title, Description: input.Description, Status: string(input.Status), AssigneeUserID: optionalUUID(input.AssigneeUserID), CreatedByUserID: uuid.MustParse(input.CreatedByUserID), CreatedAt: input.CreatedAt})
	if assigneeConstraint(err) {
		return domain.Task{}, domain.ErrAssigneeNotMember
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return mapTask(row), nil
}

func (r *Repository) Get(ctx context.Context, workspaceID, taskID string) (domain.Task, error) {
	row, err := r.q(ctx).GetTask(ctx, sqlc.GetTaskParams{WorkspaceID: uuid.MustParse(workspaceID), ID: uuid.MustParse(taskID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	return mapTask(row), nil
}

func (r *Repository) List(ctx context.Context, workspaceID string, status *domain.Status) ([]domain.Task, error) {
	filter := pgtype.Text{}
	if status != nil {
		filter = pgtype.Text{String: string(*status), Valid: true}
	}
	rows, err := r.q(ctx).ListTasks(ctx, sqlc.ListTasksParams{WorkspaceID: uuid.MustParse(workspaceID), Status: filter})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	result := make([]domain.Task, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapTask(row))
	}
	return result, nil
}

func (r *Repository) Update(ctx context.Context, input domain.Task) (domain.Task, error) {
	row, err := r.q(ctx).UpdateTask(ctx, sqlc.UpdateTaskParams{WorkspaceID: uuid.MustParse(input.WorkspaceID), ID: uuid.MustParse(input.ID), Title: input.Title, Description: input.Description, Status: string(input.Status), AssigneeUserID: optionalUUID(input.AssigneeUserID), UpdatedAt: input.UpdatedAt})
	if assigneeConstraint(err) {
		return domain.Task{}, domain.ErrAssigneeNotMember
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	return mapTask(row), nil
}

func (r *Repository) Delete(ctx context.Context, workspaceID, taskID string) error {
	return r.q(ctx).DeleteTask(ctx, sqlc.DeleteTaskParams{WorkspaceID: uuid.MustParse(workspaceID), ID: uuid.MustParse(taskID)})
}

func mapTask(row sqlc.Task) domain.Task {
	var assignee *string
	if row.AssigneeUserID != nil {
		value := row.AssigneeUserID.String()
		assignee = &value
	}
	return domain.Task{ID: row.ID.String(), WorkspaceID: row.WorkspaceID.String(), Title: row.Title, Description: row.Description, Status: domain.Status(row.Status), AssigneeUserID: assignee, CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func optionalUUID(value *string) *uuid.UUID {
	if value == nil {
		return nil
	}
	parsed := uuid.MustParse(*value)
	return &parsed
}

func assigneeConstraint(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "tasks_assignee_membership_fk"
}
