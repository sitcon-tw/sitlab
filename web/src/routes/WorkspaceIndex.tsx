import { CreateWorkspaceDialog } from "@/features/workspaces/CreateWorkspaceDialog";
import { useWorkspaces } from "@/features/workspaces/workspaceApi";
import { Button, EmptyState, PageShell, Panel, SpinnerLayout } from "@project-template/ui";
import { Boxes, Plus } from "lucide-react";
import { useState } from "react";
import { Navigate } from "react-router-dom";

export function WorkspaceIndex() {
	const workspacesQuery = useWorkspaces();
	const [creating, setCreating] = useState(false);
	if (workspacesQuery.isPending) return <SpinnerLayout label="Loading workspaces" />;
	const first = workspacesQuery.data?.[0];
	if (first) return <Navigate to={`/workspaces/${encodeURIComponent(first.id)}/tasks`} replace />;
	return (
		<PageShell title="Workspaces" description="Create a workspace to organize tasks and access.">
			<Panel>
				<EmptyState
					title="Create your first workspace"
					description="A workspace is the boundary for tasks, members, and permissions."
					icon={<Boxes size="2rem" />}
					action={
						<Button leadingIcon={<Plus size="1rem" aria-hidden="true" />} onClick={() => setCreating(true)}>
							Create workspace
						</Button>
					}
				/>
			</Panel>
			<CreateWorkspaceDialog open={creating} onOpenChange={setCreating} />
		</PageShell>
	);
}
