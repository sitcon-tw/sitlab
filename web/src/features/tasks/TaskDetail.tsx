import { useMembers } from "@/features/workspaces/workspaceApi";
import { errorMessage } from "@/shared/api/client";
import { useToast } from "@/shared/toast/toastContext";
import { Button, ConfirmDialog, EmptyState, Panel, SpinnerLayout } from "@project-template/ui";
import { ArrowLeft, FileQuestion, Pencil, Trash2, UserRound } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useDeleteTaskMutation, useTask } from "./taskApi";
import { TaskEditorDrawer } from "./TaskEditorDrawer";
import styles from "./TasksPage.module.css";
import { TaskStatusBadge } from "./TaskStatusBadge";

export interface TaskDetailProps {
	workspaceId: string;
	taskId: string;
	canEdit: boolean;
}

const dateFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

export function TaskDetail({ workspaceId, taskId, canEdit }: TaskDetailProps) {
	const taskQuery = useTask(workspaceId, taskId);
	const membersQuery = useMembers(workspaceId);
	const deleteMutation = useDeleteTaskMutation(workspaceId, taskId);
	const navigate = useNavigate();
	const { notify } = useToast();
	const [editing, setEditing] = useState(false);
	const [confirmingDelete, setConfirmingDelete] = useState(false);
	const listPath = `/workspaces/${encodeURIComponent(workspaceId)}/tasks`;

	if (taskQuery.isPending)
		return (
			<Panel>
				<SpinnerLayout label="Loading task" />
			</Panel>
		);
	if (taskQuery.isError || !taskQuery.data) {
		return (
			<Panel>
				<EmptyState
					title="Task unavailable"
					description={errorMessage(taskQuery.error, "This task may have been removed or you may not have access.")}
					icon={<FileQuestion size="2rem" />}
					action={
						<Button variant="secondary" onClick={() => navigate(listPath)}>
							Back to tasks
						</Button>
					}
				/>
			</Panel>
		);
	}
	const task = taskQuery.data;
	const assignee = membersQuery.data?.find((member) => member.userId === task.assigneeId);
	return (
		<>
			<Panel className={styles.detailPanel}>
				<div className={styles.detailHeader}>
					<Link className={styles.mobileBack} to={listPath}>
						<ArrowLeft size="1rem" aria-hidden="true" />
						Tasks
					</Link>
					<div className={styles.detailTitleRow}>
						<div>
							<TaskStatusBadge status={task.status} />
							<h2>{task.title}</h2>
						</div>
						{canEdit ? (
							<div className={styles.detailActions}>
								<Button size="sm" variant="secondary" leadingIcon={<Pencil size="1rem" aria-hidden="true" />} onClick={() => setEditing(true)}>
									Edit
								</Button>
								<Button size="sm" variant="ghost" leadingIcon={<Trash2 size="1rem" aria-hidden="true" />} onClick={() => setConfirmingDelete(true)}>
									Delete
								</Button>
							</div>
						) : null}
					</div>
				</div>
				<div className={styles.detailBody}>
					<section>
						<h3>Description</h3>
						{task.description ? <p className={styles.description}>{task.description}</p> : <p className={styles.muted}>No description provided.</p>}
					</section>
					<dl className={styles.metadata}>
						<div>
							<dt>Assignee</dt>
							<dd>
								<UserRound size="1rem" aria-hidden="true" />
								{assignee?.displayName ?? assignee?.email ?? "Unassigned"}
							</dd>
						</div>
						<div>
							<dt>Created</dt>
							<dd>{dateFormatter.format(task.createdAt)}</dd>
						</div>
						<div>
							<dt>Updated</dt>
							<dd>{dateFormatter.format(task.updatedAt)}</dd>
						</div>
					</dl>
					{!canEdit ? <p className={styles.readOnlyNotice}>You have viewer access. Task changes are disabled.</p> : null}
				</div>
			</Panel>
			{canEdit ? (
				<>
					<TaskEditorDrawer mode="edit" task={task} workspaceId={workspaceId} open={editing} onOpenChange={setEditing} onSaved={() => setEditing(false)} />
					<ConfirmDialog
						open={confirmingDelete}
						onOpenChange={setConfirmingDelete}
						title="Delete task?"
						description={`“${task.title}” will be permanently removed.`}
						confirmLabel="Delete task"
						destructive
						busy={deleteMutation.isPending}
						onConfirm={() =>
							deleteMutation.mutate(undefined, {
								onSuccess: () => {
									notify("Task deleted", { description: task.title, tone: "success" });
									navigate(listPath, { replace: true });
								}
							})
						}
					/>
				</>
			) : null}
		</>
	);
}
