import { canManageTasks } from "@/features/workspaces/model";
import { useMembers, useWorkspace } from "@/features/workspaces/workspaceApi";
import { errorMessage } from "@/shared/api/client";
import { Button, DataTable, EmptyState, PageShell, Panel, SpinnerLayout, type DataColumn } from "@project-template/ui";
import { CheckSquare2, Plus } from "lucide-react";
import { useState } from "react";
import { Link, Navigate, useNavigate, useParams, useSearchParams } from "react-router-dom";
import type { Task, TaskFilters as TaskFilterValue, TaskStatus } from "./model";
import { useTasks } from "./taskApi";
import { TaskDetail } from "./TaskDetail";
import { TaskEditorDrawer } from "./TaskEditorDrawer";
import { TaskFilters } from "./TaskFilters";
import styles from "./TasksPage.module.css";
import { TaskStatusBadge } from "./TaskStatusBadge";

const validStatuses = new Set<TaskStatus>(["todo", "in_progress", "done"]);

export function TasksPage() {
	const { workspaceId, taskId } = useParams();
	const [searchParams, setSearchParams] = useSearchParams();
	const [creating, setCreating] = useState(false);
	const navigate = useNavigate();
	const resolvedWorkspaceId = workspaceId ?? "";
	const workspaceQuery = useWorkspace(workspaceId);
	const membersQuery = useMembers(resolvedWorkspaceId, Boolean(workspaceId));

	const statusValue = searchParams.get("status");
	const filters: TaskFilterValue = {};
	if (statusValue && validStatuses.has(statusValue as TaskStatus)) filters.status = statusValue as TaskStatus;
	const queryValue = searchParams.get("q")?.trim();
	if (queryValue) filters.q = queryValue;
	const tasksQuery = useTasks(resolvedWorkspaceId, filters);

	if (!workspaceId) return <Navigate to="/" replace />;

	if (workspaceQuery.isPending) return <SpinnerLayout label="Loading workspace" />;
	if (!workspaceQuery.workspace) {
		return (
			<PageShell title="Workspace unavailable">
				<Panel>
					<EmptyState
						title="Workspace not found"
						description="It may have been removed, or your access may have changed."
						action={<Button onClick={() => navigate("/")}>Choose a workspace</Button>}
					/>
				</Panel>
			</PageShell>
		);
	}

	const workspace = workspaceQuery.workspace;
	const canEdit = canManageTasks(workspace.role);
	const memberNames = new Map((membersQuery.data ?? []).map((member) => [member.userId, member.displayName || member.email]));
	const listPath = `/workspaces/${encodeURIComponent(workspaceId)}/tasks`;
	const columns: Array<DataColumn<Task>> = [
		{
			key: "title",
			header: "Task",
			cell: (task) => (
				<Link
					className={styles.taskLink}
					aria-current={task.id === taskId ? "page" : undefined}
					to={`${listPath}/${encodeURIComponent(task.id)}${searchParams.size ? `?${searchParams.toString()}` : ""}`}
				>
					{task.title}
				</Link>
			)
		},
		{ key: "status", header: "Status", width: "8rem", cell: (task) => <TaskStatusBadge status={task.status} /> },
		{
			key: "assignee",
			header: "Assignee",
			width: "10rem",
			cell: (task) => (task.assigneeId ? (memberNames.get(task.assigneeId) ?? "Unknown member") : <span className={styles.muted}>Unassigned</span>)
		}
	];

	function updateFilters(next: TaskFilterValue) {
		const params = new URLSearchParams();
		if (next.status) params.set("status", next.status);
		if (next.q) params.set("q", next.q);
		setSearchParams(params, { replace: true });
	}

	return (
		<PageShell
			title="Tasks"
			description={`${workspace.name} · ${workspace.role.charAt(0).toUpperCase()}${workspace.role.slice(1)} access`}
			actions={
				canEdit ? (
					<Button leadingIcon={<Plus size="1rem" aria-hidden="true" />} onClick={() => setCreating(true)}>
						Create task
					</Button>
				) : undefined
			}
		>
			<div className={styles.workspace} data-has-detail={Boolean(taskId)}>
				<Panel className={styles.listPanel} title="Workspace tasks" description="Filter, scan, and select a task to inspect its details.">
					<TaskFilters filters={filters} onChange={updateFilters} />
					{tasksQuery.isPending ? <SpinnerLayout label="Loading tasks" /> : null}
					{tasksQuery.isError ? (
						<div className={styles.queryError} role="alert">
							{errorMessage(tasksQuery.error, "Tasks could not be loaded.")}
						</div>
					) : null}
					{tasksQuery.data ? (
						<DataTable
							label="Workspace tasks"
							rows={tasksQuery.data}
							columns={columns}
							getRowKey={(task) => task.id}
							empty={
								<EmptyState
									title={filters.q || filters.status ? "No matching tasks" : "No tasks yet"}
									description={
										filters.q || filters.status
											? "Try clearing or changing the current filters."
											: "Create the first task to start tracking work in this workspace."
									}
									icon={<CheckSquare2 size="2rem" />}
									action={canEdit && !filters.q && !filters.status ? <Button onClick={() => setCreating(true)}>Create task</Button> : undefined}
								/>
							}
						/>
					) : null}
				</Panel>
				<div className={styles.detailSlot}>
					{taskId ? (
						<TaskDetail workspaceId={workspaceId} taskId={taskId} canEdit={canEdit} />
					) : (
						<Panel className={styles.detailPlaceholder}>
							<EmptyState
								title="Select a task"
								description="Choose a task from the list to see its description, status, and assignee."
								icon={<CheckSquare2 size="2rem" />}
							/>
						</Panel>
					)}
				</div>
			</div>
			{canEdit ? (
				<TaskEditorDrawer
					mode="create"
					workspaceId={workspaceId}
					open={creating}
					onOpenChange={setCreating}
					onSaved={(task) => navigate(`${listPath}/${encodeURIComponent(task.id)}`)}
				/>
			) : null}
		</PageShell>
	);
}
