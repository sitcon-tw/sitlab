export const queryKeys = {
	auth: { me: ["auth", "me"] as const },
	workspaces: {
		all: ["workspaces"] as const,
		detail: (workspaceId: string) => ["workspaces", workspaceId] as const,
		members: (workspaceId: string) => ["workspaces", workspaceId, "members"] as const,
		tasks: (workspaceId: string, filters: object = {}) => ["workspaces", workspaceId, "tasks", filters] as const,
		task: (workspaceId: string, taskId: string) => ["workspaces", workspaceId, "tasks", taskId] as const
	}
};
