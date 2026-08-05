import { api, expectData, getCsrfToken } from "@/shared/api/client";
import type { BoardCard, CardComment, ProjectLabel } from "./model";

export async function createCard(input: {
	operationId: string;
	title: string;
	description: string;
	teamKey: string;
	listKey: string;
	assigneeGitLabUserIds: number[];
	labels: string[];
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

export async function listProjectLabels(): Promise<ProjectLabel[]> {
	if (import.meta.env.VITE_SITCON_DEMO === "true") return demoLabels;
	return expectData(await api.GET("/cards/labels")).labels;
}

export async function updateLabels(card: BoardCard, operationId: string, labels: string[]) {
	return expectData(
		await api.PUT("/cards/{issueIid}/labels", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, labels }
		})
	);
}

export async function listComments(card: BoardCard): Promise<CardComment[]> {
	if (import.meta.env.VITE_SITCON_DEMO === "true") return demoComments[card.issueIid] ?? [];
	return expectData(
		await api.GET("/cards/{issueIid}/comments", {
			params: { path: { issueIid: card.issueIid } }
		})
	).comments;
}

export async function createComment(card: BoardCard, body: string): Promise<CardComment> {
	if (import.meta.env.VITE_SITCON_DEMO === "true") {
		return {
			id: Date.now(),
			body,
			author: {
				gitLabUserId: 114,
				username: "yorukot",
				displayName: "Yorukot",
				avatarUrl: null,
				profileUrl: "https://gitlab.com/yorukot"
			},
			system: false,
			createdAt: new Date().toISOString(),
			updatedAt: new Date().toISOString()
		};
	}
	return expectData(
		await api.POST("/cards/{issueIid}/comments", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { body }
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

const demoLabels: ProjectLabel[] = [
	{ name: "組別::開發", color: "#0E8A16", textColor: "#FFFFFF", description: "開發組" },
	{ name: "組別::設計", color: "#B60205", textColor: "#FFFFFF", description: "設計組" },
	{ name: "Status::Inbox", color: "#64748B", textColor: "#FFFFFF", description: "收件匣" },
	{ name: "Status::To Do", color: "#0891B2", textColor: "#FFFFFF", description: "待處理" },
	{ name: "Status::Doing", color: "#2563EB", textColor: "#FFFFFF", description: "進行中" },
	{ name: "Priority::High", color: "#D73A4A", textColor: "#FFFFFF", description: "優先處理" },
	{ name: "Backend", color: "#1D76DB", textColor: "#FFFFFF", description: null }
];

const demoComments: Record<number, CardComment[]> = {
	127: [
		{
			id: 7001,
			body: "changed status to **To Do**",
			author: {
				gitLabUserId: 114,
				username: "yorukot",
				displayName: "Yorukot",
				avatarUrl: null,
				profileUrl: "https://gitlab.com/yorukot"
			},
			system: true,
			createdAt: "2026-07-28T08:00:00Z",
			updatedAt: "2026-07-28T08:00:00Z"
		},
		{
			id: 7002,
			body: "重送條件已整理完成，請協助 review。",
			author: {
				gitLabUserId: 115,
				username: "ming",
				displayName: "沈明軒",
				avatarUrl: null,
				profileUrl: "https://gitlab.com/ming"
			},
			system: false,
			createdAt: "2026-07-29T06:30:00Z",
			updatedAt: "2026-07-29T06:30:00Z"
		}
	]
};
