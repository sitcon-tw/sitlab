-- name: CreateTask :one
INSERT INTO tasks (
  id, workspace_id, title, description, status, assignee_user_id,
  created_by_user_id, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE workspace_id = $1 AND id = $2;

-- name: ListTasks :many
SELECT * FROM tasks
WHERE workspace_id = $1 AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at DESC, id DESC;

-- name: UpdateTask :one
UPDATE tasks SET title = $3, description = $4, status = $5,
  assignee_user_id = $6, updated_at = $7
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE workspace_id = $1 AND id = $2;
