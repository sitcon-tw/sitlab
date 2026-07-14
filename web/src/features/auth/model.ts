import type { components } from "@/shared/api/openapi";

type ApiUser = components["schemas"]["User"];

export interface User {
	id: string;
	email: string;
	displayName: string;
	createdAt: Date;
}

export interface LoginInput {
	email: string;
	password: string;
}
export interface RegisterInput extends LoginInput {
	displayName: string;
}

export function mapUser(user: ApiUser): User {
	return {
		id: user.id,
		email: user.email,
		displayName: user.displayName,
		createdAt: new Date(user.createdAt)
	};
}
