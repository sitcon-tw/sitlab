import type { WorkspaceMember } from "@/features/workspaces/model";
import { errorMessage, fieldError } from "@/shared/api/client";
import { SelectField, TextAreaField, TextField } from "@project-template/ui";
import { useState, type FormEvent } from "react";
import type { TaskDraft } from "./model";
import styles from "./TasksPage.module.css";

const emptyDraft: TaskDraft = { title: "", description: "", status: "todo", assigneeId: null };

export interface TaskFormProps {
	id: string;
	initial?: TaskDraft | undefined;
	members: WorkspaceMember[];
	onSubmit: (draft: TaskDraft) => Promise<void>;
	error?: unknown;
}

export function TaskForm({ id, initial = emptyDraft, members, onSubmit, error }: TaskFormProps) {
	const [draft, setDraft] = useState<TaskDraft>(initial);

	async function submit(event: FormEvent) {
		event.preventDefault();
		await onSubmit({ ...draft, title: draft.title.trim(), description: draft.description.trim() });
	}

	return (
		<form id={id} className={styles.taskForm} onSubmit={submit}>
			<TextField
				label="Title"
				required
				autoFocus
				maxLength={160}
				value={draft.title}
				error={fieldError(error, "title")}
				onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))}
			/>
			<TextAreaField
				label="Description"
				optional
				maxLength={4000}
				value={draft.description}
				error={fieldError(error, "description")}
				onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
			/>
			<SelectField
				label="Status"
				value={draft.status}
				error={fieldError(error, "status")}
				onChange={(event) => setDraft((current) => ({ ...current, status: event.target.value as TaskDraft["status"] }))}
				options={[
					{ value: "todo", label: "To do" },
					{ value: "in_progress", label: "In progress" },
					{ value: "done", label: "Done" }
				]}
			/>
			<SelectField
				label="Assignee"
				optional
				value={draft.assigneeId ?? ""}
				error={fieldError(error, "assigneeId")}
				onChange={(event) => setDraft((current) => ({ ...current, assigneeId: event.target.value || null }))}
				options={[{ value: "", label: "Unassigned" }, ...members.map((member) => ({ value: member.userId, label: member.displayName || member.email }))]}
			/>
			{error ? (
				<p className={styles.formError} role="alert">
					{errorMessage(error)}
				</p>
			) : null}
		</form>
	);
}
