import { api, expectData, getCsrfToken } from "@/shared/api/client";
import type { BoardCard, CardComment, CreateCardCommentResult, QuickActionCommand, QuickActionSuggestion, QuickActionSuggestionKind } from "./model";

const demo = import.meta.env.VITE_SITCON_DEMO === "true";

/**
 * Demo mode resolves card mutations locally.
 *
 * Without this every save in demo mode fails, which leaves the optimistic state
 * on screen (that is what the e2e assertions read) but makes the in-place save
 * indicators unreviewable and untestable.
 */
async function demoMutation(card: BoardCard, patch: Partial<BoardCard>, operationId: string) {
	await new Promise((resolve) => setTimeout(resolve, 250));
	const next: BoardCard = { ...card, ...patch, syncState: "synced", syncError: null, pendingOperationId: null, updatedAt: new Date().toISOString() };
	return { card: next, operation: { id: operationId, kind: "update_details" as const, state: "succeeded" as const, attempts: 1, lastError: null } };
}

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

export async function deleteCard(card: BoardCard, operationId: string) {
	if (demo) {
		await new Promise((resolve) => setTimeout(resolve, 250));
		return {
			operation: {
				id: operationId,
				kind: "delete_card" as const,
				state: "pending" as const,
				attempts: 0,
				lastError: null,
				createdAt: new Date().toISOString(),
				updatedAt: new Date().toISOString()
			}
		};
	}
	return expectData(
		await api.DELETE("/cards/{issueIid}", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId }
		})
	);
}

export async function updateDetails(card: BoardCard, operationId: string, title: string, description: string) {
	if (demo) return demoMutation(card, { title, description }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/details", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, title, description }
		})
	);
}

export async function updateTeam(card: BoardCard, operationId: string, teamKey: string) {
	if (demo) return demoMutation(card, { teamKey }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/team", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, teamKey }
		})
	);
}

export async function updateAssignee(card: BoardCard, operationId: string, assigneeGitLabUserIds: number[]) {
	if (demo) return demoMutation(card, { assigneeGitLabUserIds }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/assignee", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, assigneeGitLabUserIds }
		})
	);
}

export async function updateDueDate(card: BoardCard, operationId: string, dueDate: string | null) {
	if (demo) return demoMutation(card, { dueDate }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/due-date", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, dueDate }
		})
	);
}

export async function updateStartDate(card: BoardCard, operationId: string, startDate: string | null) {
	if (demo) return demoMutation(card, { startDate }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/start-date", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, startDate }
		})
	);
}

export async function updateLabels(card: BoardCard, operationId: string, labels: string[]) {
	if (demo) return demoMutation(card, { labels }, operationId);
	return expectData(
		await api.PUT("/cards/{issueIid}/labels", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { operationId, labels }
		})
	);
}

export async function listComments(card: BoardCard): Promise<CardComment[]> {
	if (demo) return demoComments[card.issueIid] ?? [];
	return expectData(
		await api.GET("/cards/{issueIid}/comments", {
			params: { path: { issueIid: card.issueIid } }
		})
	).comments;
}

export async function createComment(card: BoardCard, body: string): Promise<CreateCardCommentResult> {
	if (demo) {
		return {
			comment: {
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
			},
			quickActionsApplied: false,
			summary: []
		};
	}
	return expectData(
		await api.POST("/cards/{issueIid}/comments", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { body }
		})
	);
}

export async function listQuickActions(issueIid?: number): Promise<QuickActionCommand[]> {
	if (demo) return demoQuickActions;
	return expectData(
		await api.GET("/quick-actions", {
			params: { query: issueIid && issueIid > 0 ? { issueIid } : {} }
		})
	).commands;
}

export async function listQuickActionSuggestions(kind: QuickActionSuggestionKind, query: string, issueIid?: number): Promise<QuickActionSuggestion[]> {
	if (demo) return [];
	return expectData(
		await api.GET("/quick-actions/suggestions", {
			params: { query: { kind, ...(query ? { query } : {}), ...(issueIid && issueIid > 0 ? { issueIid } : {}) } }
		})
	).suggestions;
}

const demoQuickActions: QuickActionCommand[] = [
	{ name: "assign", aliases: [], params: ["@user"], description: "Assign one or more users", warning: null, icon: null },
	{ name: "unassign", aliases: [], params: ["[@user]"], description: "Remove assignees", warning: null, icon: null },
	{ name: "label", aliases: [], params: ["~label"], description: "Add one or more labels", warning: null, icon: null },
	{ name: "unlabel", aliases: [], params: ["~label"], description: "Remove one or more labels", warning: null, icon: null },
	{ name: "status", aliases: [], params: ['"status"'], description: "Set lifecycle status", warning: null, icon: null },
	{ name: "due", aliases: [], params: ["<date>"], description: "Set due date", warning: null, icon: null },
	{ name: "close", aliases: [], params: [], description: "Close the work item", warning: null, icon: null },
	{ name: "reopen", aliases: [], params: [], description: "Reopen the work item", warning: null, icon: null }
];

export async function moveCard(card: BoardCard, operationId: string, listKey: string, position?: number) {
	if (demo) return demoMutation(card, position === undefined ? { listKey } : { listKey, position }, operationId);
	const body = position === undefined ? { operationId, listKey } : { operationId, listKey, position };
	return expectData(
		await api.PUT("/cards/{issueIid}/position", {
			params: { path: { issueIid: card.issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body
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
			// GitLab emits title-change system notes as inline-diff HTML.
			body: '<p>changed title from <code class="idiff">修正報名系統寄信流程</code> to <code class="idiff"><span class="idiff left right addition">[開發組] </span>修正報名系統寄信流程</code></p>',
			author: {
				gitLabUserId: 114,
				username: "yorukot",
				displayName: "Yorukot",
				avatarUrl: null,
				profileUrl: "https://gitlab.com/yorukot"
			},
			system: true,
			createdAt: "2026-07-28T08:05:00Z",
			updatedAt: "2026-07-28T08:05:00Z"
		},
		{
			id: 7003,
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
