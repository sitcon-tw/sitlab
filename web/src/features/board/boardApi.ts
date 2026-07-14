import { api, expectData, getCsrfToken } from "@/shared/api/client";
import type { BoardCard } from "./model";

export async function createCard(input: {
	operationId: string;
	title: string;
	description: string;
	teamKey: string;
	assigneeGitLabUserIds: number[];
	startDate: string | null;
	dueDate: string | null;
}) {
	return expectData(
		await api.POST("/cards", {
			params: { header: { "X-CSRF-Token": await getCsrfToken() } },
			body: input
		})
	);
}

export async function updateDetails(card: BoardCard, operationId: string, title: string, description: string) {
	return expectData(
		await api.PUT("/cards/{issueIid}/details", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, title, description }
		})
	);
}

export async function updateTeam(card: BoardCard, operationId: string, teamKey: string) {
	return expectData(
		await api.PUT("/cards/{issueIid}/team", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, teamKey }
		})
	);
}

export async function updateAssignee(card: BoardCard, operationId: string, assigneeGitLabUserIds: number[]) {
	return expectData(
		await api.PUT("/cards/{issueIid}/assignee", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, assigneeGitLabUserIds }
		})
	);
}

export async function updateDueDate(card: BoardCard, operationId: string, dueDate: string | null) {
	return expectData(
		await api.PUT("/cards/{issueIid}/due-date", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, dueDate }
		})
	);
}

export async function updateStartDate(card: BoardCard, operationId: string, startDate: string | null) {
	return expectData(
		await api.PUT("/cards/{issueIid}/start-date", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, startDate }
		})
	);
}

export async function moveCard(card: BoardCard, operationId: string, listKey: string, position: number) {
	return expectData(
		await api.PUT("/cards/{issueIid}/position", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, listKey, position }
		})
	);
}

export async function retryOperation(operationId: string) {
	return expectData(
		await api.POST("/operations/{operationId}/retry", {
			params: { path: { operationId }, header: { "X-CSRF-Token": await getCsrfToken() } }
		})
	);
}

export async function savePreferences(defaultTeamKey: string) {
	return expectData(
		await api.PUT("/me/preferences", {
			params: { header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { defaultTeamKey }
		})
	);
}

export async function logout() {
	await api.POST("/auth/logout", { params: { header: { "X-CSRF-Token": await getCsrfToken() } } });
}
