import { api, expectData, getCsrfToken } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/queryKeys";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { mapMember, mapWorkspace } from "./model";

export const workspacesQuery = {
	queryKey: queryKeys.workspaces.all,
	queryFn: async () => {
		const result = await api.GET("/workspaces");
		return expectData(result).workspaces.map(mapWorkspace);
	},
	staleTime: 30_000
} as const;

export function useWorkspaces() {
	return useQuery(workspacesQuery);
}

export function useWorkspace(workspaceId: string | undefined) {
	const query = useWorkspaces();
	return { ...query, workspace: query.data?.find((workspace) => workspace.id === workspaceId) ?? null };
}

export function useCreateWorkspaceMutation() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (name: string) => {
			const token = await getCsrfToken();
			const result = await api.POST("/workspaces", { body: { name }, params: { header: { "X-CSRF-Token": token } } });
			return mapWorkspace(expectData(result).workspace);
		},
		onSuccess: (workspace) => {
			queryClient.setQueryData(queryKeys.workspaces.all, (current: ReturnType<typeof mapWorkspace>[] | undefined) => [...(current ?? []), workspace]);
		}
	});
}

export function useMembers(workspaceId: string, enabled = true) {
	return useQuery({
		queryKey: queryKeys.workspaces.members(workspaceId),
		queryFn: async () => {
			const result = await api.GET("/workspaces/{workspaceId}/members", { params: { path: { workspaceId } } });
			return expectData(result).members.map(mapMember);
		},
		enabled,
		staleTime: 30_000
	});
}
