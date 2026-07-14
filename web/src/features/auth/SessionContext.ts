import { createContext, useContext } from "react";
import type { LoginInput, RegisterInput, User } from "./model";

export interface SessionValue {
	user: User | null;
	loading: boolean;
	submitting: boolean;
	login: (input: LoginInput) => Promise<User>;
	register: (input: RegisterInput) => Promise<User>;
	logout: () => Promise<void>;
}

export const SessionContext = createContext<SessionValue | null>(null);

export function useSession() {
	const value = useContext(SessionContext);
	if (!value) throw new Error("useSession must be used inside SessionProvider");
	return value;
}
