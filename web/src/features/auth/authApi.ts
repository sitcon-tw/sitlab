import { api, clearCsrfToken, expectData, getCsrfToken } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/queryKeys";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { mapUser, type LoginInput, type RegisterInput } from "./model";

export const currentUserQuery = {
	queryKey: queryKeys.auth.me,
	queryFn: async () => {
		const result = await api.GET("/auth/me");
		if (result.response.status === 401) return null;
		return mapUser(expectData(result).user);
	},
	retry: false,
	staleTime: 60_000
} as const;

export function useLoginMutation() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (input: LoginInput) => {
			const result = await api.POST("/auth/login", { body: input });
			return mapUser(expectData(result).user);
		},
		onSuccess: (user) => queryClient.setQueryData(queryKeys.auth.me, user)
	});
}

export function useRegisterMutation() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (input: RegisterInput) => {
			const result = await api.POST("/auth/register", { body: input });
			return mapUser(expectData(result).user);
		},
		onSuccess: (user) => queryClient.setQueryData(queryKeys.auth.me, user)
	});
}

export function useLogoutMutation() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async () => {
			const token = await getCsrfToken();
			const result = await api.POST("/auth/logout", { params: { header: { "X-CSRF-Token": token } } });
			if (!result.response.ok) expectData(result);
		},
		onSuccess: () => {
			clearCsrfToken();
			queryClient.clear();
			queryClient.setQueryData(queryKeys.auth.me, null);
		}
	});
}
