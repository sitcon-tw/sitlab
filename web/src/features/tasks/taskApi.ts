import { api, expectData, getCsrfToken } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/queryKeys";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { mapTask, type TaskDraft, type TaskFilters } from "./model";

export function useTasks(workspaceId: string, filters: TaskFilters) {
	const serverFilters = filters.status ? { status: filters.status } : {};
	return useQuery({
		queryKey: queryKeys.workspaces.tasks(workspaceId, serverFilters),
		queryFn: async () => {
			const result = await api.GET("/workspaces/{workspaceId}/tasks", {
				params: { path: { workspaceId }, query: serverFilters }
			});
			return expectData(result).tasks.map(mapTask);
		},
		select: (tasks) => {
			const query = filters.q?.trim().toLocaleLowerCase();
			return query ? tasks.filter((task) => task.title.toLocaleLowerCase().includes(query) || task.description.toLocaleLowerCase().includes(query)) : tasks;
		},
		placeholderData: (previous) => previous
	});
}

export function useTask(workspaceId: string, taskId: string | undefined) {
	return useQuery({
		queryKey: queryKeys.workspaces.task(workspaceId, taskId ?? ""),
		queryFn: async () => {
			if (!taskId) throw new Error("A task ID is required.");
			const result = await api.GET("/workspaces/{workspaceId}/tasks/{taskId}", { params: { path: { workspaceId, taskId } } });
			return mapTask(expectData(result).task);
		},
		enabled: Boolean(taskId)
	});
}

export function useCreateTaskMutation(workspaceId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (draft: TaskDraft) => {
			const token = await getCsrfToken();
			const result = await api.POST("/workspaces/{workspaceId}/tasks", {
				params: { path: { workspaceId }, header: { "X-CSRF-Token": token } },
				body: draft
			});
			return mapTask(expectData(result).task);
		},
		onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...queryKeys.workspaces.detail(workspaceId), "tasks"] })
	});
}

export function useUpdateTaskMutation(workspaceId: string, taskId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (draft: TaskDraft) => {
			const token = await getCsrfToken();
			const result = await api.PATCH("/workspaces/{workspaceId}/tasks/{taskId}", {
				params: { path: { workspaceId, taskId }, header: { "X-CSRF-Token": token } },
				body: draft
			});
			return mapTask(expectData(result).task);
		},
		onMutate: async (draft) => {
			const key = queryKeys.workspaces.task(workspaceId, taskId);
			await queryClient.cancelQueries({ queryKey: key });
			const previous = queryClient.getQueryData<ReturnType<typeof mapTask>>(key);
			if (previous) {
				queryClient.setQueryData(key, {
					...previous,
					...draft,
					updatedAt: new Date()
				});
			}
			return { previous };
		},
		onError: (_error, _draft, context) => {
			if (context?.previous) queryClient.setQueryData(queryKeys.workspaces.task(workspaceId, taskId), context.previous);
		},
		onSuccess: (task) => {
			queryClient.setQueryData(queryKeys.workspaces.task(workspaceId, taskId), task);
			void queryClient.invalidateQueries({ queryKey: [...queryKeys.workspaces.detail(workspaceId), "tasks"] });
		}
	});
}

export function useDeleteTaskMutation(workspaceId: string, taskId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async () => {
			const token = await getCsrfToken();
			const result = await api.DELETE("/workspaces/{workspaceId}/tasks/{taskId}", {
				params: { path: { workspaceId, taskId }, header: { "X-CSRF-Token": token } }
			});
			if (!result.response.ok) expectData(result);
		},
		onSuccess: () => {
			queryClient.removeQueries({ queryKey: queryKeys.workspaces.task(workspaceId, taskId) });
			void queryClient.invalidateQueries({ queryKey: [...queryKeys.workspaces.detail(workspaceId), "tasks"] });
		}
	});
}
