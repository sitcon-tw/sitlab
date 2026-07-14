-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, created_by_user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4)
RETURNING *;

-- name: CreateWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4);

-- name: GetWorkspaceForUser :one
SELECT w.*, wm.role FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE w.id = $1 AND wm.user_id = $2;

-- name: ListWorkspacesForUser :many
SELECT w.*, wm.role FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1
ORDER BY w.updated_at DESC, w.id DESC;

-- name: UpdateWorkspace :one
UPDATE workspaces SET name = $2, updated_at = $3 WHERE id = $1 RETURNING *;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = $1;

-- name: GetWorkspaceMemberRole :one
SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- name: LockWorkspaceMembership :one
SELECT id FROM workspaces WHERE id = $1 FOR UPDATE;

-- name: ListWorkspaceMembers :many
SELECT wm.workspace_id, wm.user_id, u.email, u.display_name, wm.role, wm.joined_at
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
ORDER BY wm.joined_at, wm.user_id;

-- name: GetWorkspaceMember :one
SELECT wm.workspace_id, wm.user_id, u.email, u.display_name, wm.role, wm.joined_at
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1 AND wm.user_id = $2;

-- name: CountWorkspaceOwners :one
SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'owner';

-- name: UpdateWorkspaceMemberRole :exec
UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2;

-- name: DeleteWorkspaceMember :exec
DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;
