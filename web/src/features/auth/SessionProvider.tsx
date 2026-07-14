import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { currentUserQuery, useLoginMutation, useLogoutMutation, useRegisterMutation } from "./authApi";
import { SessionContext } from "./SessionContext";

export function SessionProvider({ children }: { children: ReactNode }) {
	const userQuery = useQuery(currentUserQuery);
	const loginMutation = useLoginMutation();
	const registerMutation = useRegisterMutation();
	const logoutMutation = useLogoutMutation();
	return (
		<SessionContext.Provider
			value={{
				user: userQuery.data ?? null,
				loading: userQuery.isPending,
				submitting: loginMutation.isPending || registerMutation.isPending || logoutMutation.isPending,
				login: loginMutation.mutateAsync,
				register: registerMutation.mutateAsync,
				logout: async () => {
					await logoutMutation.mutateAsync();
				}
			}}
		>
			{children}
		</SessionContext.Provider>
	);
}
