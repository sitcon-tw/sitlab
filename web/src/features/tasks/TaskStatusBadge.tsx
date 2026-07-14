import { Badge, type BadgeTone } from "@project-template/ui";
import { taskStatusLabels, type TaskStatus } from "./model";

const statusTone: Record<TaskStatus, BadgeTone> = {
	todo: "neutral",
	in_progress: "info",
	done: "success"
};

export function TaskStatusBadge({ status }: { status: TaskStatus }) {
	return <Badge tone={statusTone[status]}>{taskStatusLabels[status]}</Badge>;
}
