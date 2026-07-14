import { errorMessage } from "@/shared/api/client";
import { useToast } from "@/shared/toast/toastContext";
import { Button, Dialog, TextField } from "@project-template/ui";
import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import styles from "./Workspace.module.css";
import { useCreateWorkspaceMutation } from "./workspaceApi";

export interface CreateWorkspaceDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function CreateWorkspaceDialog({ open, onOpenChange }: CreateWorkspaceDialogProps) {
	const [name, setName] = useState("");
	const navigate = useNavigate();
	const { notify } = useToast();
	const mutation = useCreateWorkspaceMutation();

	async function submit(event: FormEvent) {
		event.preventDefault();
		const workspace = await mutation.mutateAsync(name.trim());
		setName("");
		onOpenChange(false);
		notify("Workspace created", { description: `${workspace.name} is ready.`, tone: "success" });
		navigate(`/workspaces/${encodeURIComponent(workspace.id)}/tasks`);
	}

	return (
		<Dialog
			open={open}
			onOpenChange={onOpenChange}
			title="Create workspace"
			description="Workspaces keep tasks and access boundaries separate."
			footer={
				<>
					<Button variant="secondary" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button type="submit" form="create-workspace" loading={mutation.isPending} disabled={!name.trim()}>
						Create workspace
					</Button>
				</>
			}
		>
			<form id="create-workspace" className={styles.form} onSubmit={submit}>
				<TextField label="Workspace name" autoFocus required maxLength={80} value={name} onChange={(event) => setName(event.target.value)} />
				{mutation.error ? (
					<p className={styles.formError} role="alert">
						{errorMessage(mutation.error)}
					</p>
				) : null}
			</form>
		</Dialog>
	);
}
