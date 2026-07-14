import type { components } from "@/shared/api/openapi";

type ApiTask = components["schemas"]["Task"];
export type TaskStatus = "todo" | "in_progress" | "done";

export interface Task {
	id: string;
	workspaceId: string;
	title: string;
	description: string;
	status: TaskStatus;
	assigneeId: string | null;
	createdAt: Date;
	updatedAt: Date;
}

export interface TaskDraft {
	title: string;
	description: string;
	status: TaskStatus;
	assigneeId: string | null;
}

export interface TaskFilters {
	status?: TaskStatus;
	q?: string;
}

export const taskStatusLabels: Record<TaskStatus, string> = {
	todo: "To do",
	in_progress: "In progress",
	done: "Done"
};

export function mapTask(task: ApiTask): Task {
	return {
		id: task.id,
		workspaceId: task.workspaceId,
		title: task.title,
		description: task.description,
		status: task.status,
		assigneeId: task.assigneeId ?? null,
		createdAt: new Date(task.createdAt),
		updatedAt: new Date(task.updatedAt)
	};
}

export function taskToDraft(task: Task): TaskDraft {
	return { title: task.title, description: task.description, status: task.status, assigneeId: task.assigneeId };
}
