import createClient from "openapi-fetch";
import type { components, paths } from "./openapi";

export type Problem = components["schemas"]["ProblemDetails"];

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly errors: NonNullable<Problem["errors"]>;
	readonly requestId?: string;

	constructor(problem: Problem, fallbackStatus: number) {
		super(problem.detail ?? problem.title);
		this.name = "ApiError";
		this.status = problem.status || fallbackStatus;
		this.code = problem.code;
		this.errors = problem.errors ?? [];
		if (problem.requestId) this.requestId = problem.requestId;
	}
}

export function fieldError(error: unknown, field: string) {
	if (!(error instanceof ApiError)) return undefined;
	return error.errors.find((item) => item.location === field || item.location?.endsWith(`.${field}`))?.message;
}

export const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? "/api/v1").replace(/\/$/, "");

export const api = createClient<paths>({
	baseUrl: apiBaseUrl,
	credentials: "include",
	headers: { Accept: "application/json, application/problem+json" }
});

let csrfToken: string | null = null;

export async function getCsrfToken() {
	if (csrfToken) return csrfToken;
	const result = await api.GET("/auth/csrf");
	csrfToken = expectData(result).token;
	return csrfToken;
}

export function clearCsrfToken() {
	csrfToken = null;
}

export function setCsrfToken(token: string) {
	csrfToken = token;
}

export interface ApiResult<T> {
	data?: T;
	error?: unknown;
	response: Response;
}

export function expectData<T>(result: ApiResult<T>): T {
	if (result.data !== undefined) return result.data;
	const fallback: Problem = {
		type: "about:blank",
		title: "Request failed",
		status: result.response.status,
		code: "INTERNAL_ERROR"
	};
	const problem = result.error && typeof result.error === "object" ? (result.error as Partial<Problem>) : {};
	throw new ApiError({ ...fallback, ...problem }, result.response.status);
}

export function errorMessage(error: unknown, fallback = "Something went wrong. Try again.") {
	return error instanceof Error && error.message ? error.message : fallback;
}
