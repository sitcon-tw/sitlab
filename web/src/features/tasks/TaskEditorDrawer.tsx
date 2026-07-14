import { useMembers } from "@/features/workspaces/workspaceApi";
import { useToast } from "@/shared/toast/toastContext";
import { Button, Drawer } from "@project-template/ui";
import { taskToDraft, type Task } from "./model";
import { useCreateTaskMutation, useUpdateTaskMutation } from "./taskApi";
import { TaskForm } from "./TaskForm";

interface CommonProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	workspaceId: string;
	onSaved: (task: Task) => void;
}
type TaskEditorDrawerProps = CommonProps & ({ mode: "create"; task?: never } | { mode: "edit"; task: Task });

export function TaskEditorDrawer(props: TaskEditorDrawerProps) {
	const { open, onOpenChange, workspaceId, onSaved } = props;
	const createMutation = useCreateTaskMutation(workspaceId);
	const updateMutation = useUpdateTaskMutation(workspaceId, props.mode === "edit" ? props.task.id : "unused");
	const membersQuery = useMembers(workspaceId, open);
	const { notify } = useToast();
	const mutation = props.mode === "create" ? createMutation : updateMutation;
	const formId = `${props.mode}-task-form`;

	return (
		<Drawer
			open={open}
			onOpenChange={onOpenChange}
			title={props.mode === "create" ? "Create task" : "Edit task"}
			description={props.mode === "create" ? "Add a focused unit of work to this workspace." : "Update the task details and ownership."}
			footer={
				<>
					<Button variant="secondary" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button type="submit" form={formId} loading={mutation.isPending}>
						{props.mode === "create" ? "Create task" : "Save changes"}
					</Button>
				</>
			}
		>
			<TaskForm
				id={formId}
				initial={props.mode === "edit" ? taskToDraft(props.task) : undefined}
				members={membersQuery.data ?? []}
				error={mutation.error}
				onSubmit={async (draft) => {
					const task = await mutation.mutateAsync(draft);
					onOpenChange(false);
					notify(props.mode === "create" ? "Task created" : "Task updated", { description: task.title, tone: "success" });
					onSaved(task);
				}}
			/>
		</Drawer>
	);
}
