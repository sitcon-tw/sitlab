import { CreateWorkspaceDialog } from "@/features/workspaces/CreateWorkspaceDialog";
import { useWorkspaces } from "@/features/workspaces/workspaceApi";
import { IconButton } from "@project-template/ui";
import { Plus } from "lucide-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import styles from "./AppShell.module.css";

export function WorkspaceSwitcher() {
	const { workspaceId } = useParams();
	const workspacesQuery = useWorkspaces();
	const navigate = useNavigate();
	const [creating, setCreating] = useState(false);
	return (
		<div className={styles.workspaceSwitcher}>
			<label htmlFor="workspace-switcher">Workspace</label>
			<div className={styles.workspaceControl}>
				<select
					id="workspace-switcher"
					value={workspaceId ?? ""}
					disabled={!workspacesQuery.data?.length}
					onChange={(event) => navigate(`/workspaces/${encodeURIComponent(event.target.value)}/tasks`)}
				>
					{!workspacesQuery.data?.length ? <option value="">No workspaces</option> : null}
					{workspacesQuery.data?.map((workspace) => (
						<option key={workspace.id} value={workspace.id}>
							{workspace.name}
						</option>
					))}
				</select>
				<IconButton label="Create workspace" icon={<Plus size="1rem" aria-hidden="true" />} onClick={() => setCreating(true)} />
			</div>
			<CreateWorkspaceDialog open={creating} onOpenChange={setCreating} />
		</div>
	);
}
