namespace ProjectTemplate;

alias WorkspaceRole = "owner" | "editor" | "viewer";
alias AssignableWorkspaceRole = WorkspaceRole;

@example(#{
	id: "22222222-2222-2222-2222-222222222222",
	name: "Product Engineering",
	role: "owner",
	createdByUserId: "11111111-1111-1111-1111-111111111111",
	createdAt: utcDateTime.fromISO("2026-07-10T08:10:00Z"),
	updatedAt: utcDateTime.fromISO("2026-07-10T08:10:00Z"),
})
model Workspace {
	id: uuid;

	@minLength(1)
	@maxLength(80)
	name: string;

	role: WorkspaceRole;
	createdByUserId: uuid;
	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

@example(#{ name: "Product Engineering" })
model CreateWorkspaceRequest {
	@minLength(1)
	@maxLength(80)
	name: string;
}

@example(#{ name: "Platform Engineering" })
model UpdateWorkspaceRequest {
	@minLength(1)
	@maxLength(80)
	name: string;
}

model WorkspaceResponse {
	workspace: Workspace;
}

model WorkspaceListResponse {
	workspaces: Workspace[];
}

@example(#{
	userId: "11111111-1111-1111-1111-111111111111",
	email: "owner@example.com",
	displayName: "Alex Morgan",
	role: "owner",
	createdAt: utcDateTime.fromISO("2026-07-10T08:10:00Z"),
})
model WorkspaceMember {
	userId: uuid;
	email: email;
	displayName: string;
	role: WorkspaceRole;
	createdAt: utcDateTime;
}

@example(#{ email: "editor@example.com", role: "editor" })
model AddWorkspaceMemberRequest {
	email: email;
	role: AssignableWorkspaceRole;
}

@example(#{ role: "viewer" })
model UpdateWorkspaceMemberRequest {
	role: AssignableWorkspaceRole;
}

model WorkspaceMemberResponse {
	member: WorkspaceMember;
}

model WorkspaceMemberListResponse {
	members: WorkspaceMember[];
}
