import type { components } from "@/shared/api/openapi";

type ApiWorkspace = components["schemas"]["Workspace"];
type ApiMember = components["schemas"]["WorkspaceMember"];

export type WorkspaceRole = "owner" | "editor" | "viewer";

export interface Workspace {
	id: string;
	name: string;
	role: WorkspaceRole;
	createdAt: Date;
}

export interface WorkspaceMember {
	userId: string;
	displayName: string;
	email: string;
	role: WorkspaceRole;
}

export function mapWorkspace(workspace: ApiWorkspace): Workspace {
	return { id: workspace.id, name: workspace.name, role: workspace.role, createdAt: new Date(workspace.createdAt) };
}

export function mapMember(member: ApiMember): WorkspaceMember {
	return { userId: member.userId, displayName: member.displayName, email: member.email, role: member.role };
}

export function canManageTasks(role: WorkspaceRole) {
	return role === "owner" || role === "editor";
}

export function canManageMembers(role: WorkspaceRole) {
	return role === "owner";
}
