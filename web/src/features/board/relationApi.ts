import { api, expectData, getCsrfToken, type ApiResult } from "@/shared/api/client";
import type { LinkedWorkItem, WorkItemLinkType, WorkItemRelationshipKind, WorkItemSummary } from "./model";

const demo = import.meta.env.VITE_SITCON_DEMO === "true";
const demoDelay = 180;

export const relationshipKeys = {
	children: (issueIid: number) => ["card-relations", issueIid, "children"] as const,
	linked: (issueIid: number) => ["card-relations", issueIid, "linked"] as const,
	candidates: (issueIid: number, kind: WorkItemRelationshipKind, query: string) => ["card-relations", issueIid, "candidates", kind, query] as const
};

export async function listChildItems(issueIid: number) {
	if (demo) {
		await waitForDemo();
		const items = [...(demoChildren.get(issueIid) ?? [])];
		return { items, totalCount: items.length, nextCursor: null };
	}
	return expectData(
		await api.GET("/cards/{issueIid}/child-items", {
			params: { path: { issueIid }, query: { limit: 50 } }
		})
	);
}

export async function listLinkedItems(issueIid: number) {
	if (demo) {
		await waitForDemo();
		const items = [...(demoLinks.get(issueIid) ?? [])];
		return { items, totalCount: items.length, nextCursor: null };
	}
	return expectData(
		await api.GET("/cards/{issueIid}/linked-items", {
			params: { path: { issueIid }, query: { limit: 50 } }
		})
	);
}

export async function searchRelationshipCandidates(issueIid: number, kind: WorkItemRelationshipKind, query: string) {
	if (demo) {
		await waitForDemo();
		const normalized = query.trim().replace(/^#/, "").toLocaleLowerCase("zh-Hant");
		const currentChildren = new Set((demoChildren.get(issueIid) ?? []).map((item) => item.gitLabWorkItemId));
		const currentLinks = new Set((demoLinks.get(issueIid) ?? []).map((item) => item.gitLabWorkItemId));
		return demoCandidates.filter((item) => {
			if (kind === "child" && item.type !== "task") return false;
			if (kind === "child" && currentChildren.has(item.gitLabWorkItemId)) return false;
			if (kind === "linked" && currentLinks.has(item.gitLabWorkItemId)) return false;
			return String(item.iid) === normalized || item.title.toLocaleLowerCase("zh-Hant").includes(normalized);
		});
	}
	return expectData(
		await api.GET("/cards/{issueIid}/relationship-candidates", {
			params: { path: { issueIid }, query: { kind, query } }
		})
	).items;
}

export async function createChildItem(issueIid: number, title: string) {
	if (demo) {
		await waitForDemo();
		const item = workItem(9500 + nextDemoChildIID, nextDemoChildIID++, "task", title);
		demoChildren.set(issueIid, [...(demoChildren.get(issueIid) ?? []), item]);
		return item;
	}
	return expectData(
		await api.POST("/cards/{issueIid}/child-items", {
			params: { path: { issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { title }
		})
	);
}

export async function attachChildItem(issueIid: number, workItemId: number) {
	if (demo) {
		await waitForDemo();
		const item = demoCandidates.find((candidate) => candidate.gitLabWorkItemId === workItemId && candidate.type === "task");
		if (item) demoChildren.set(issueIid, [...(demoChildren.get(issueIid) ?? []), item]);
		return;
	}
	ensureNoContent(
		await api.PUT("/cards/{issueIid}/child-items/{workItemId}", {
			params: { path: { issueIid, workItemId }, header: { "X-CSRF-Token": await getCsrfToken() } }
		})
	);
}

export async function detachChildItem(issueIid: number, workItemId: number) {
	if (demo) {
		await waitForDemo();
		demoChildren.set(
			issueIid,
			(demoChildren.get(issueIid) ?? []).filter((item) => item.gitLabWorkItemId !== workItemId)
		);
		return;
	}
	ensureNoContent(
		await api.DELETE("/cards/{issueIid}/child-items/{workItemId}", {
			params: { path: { issueIid, workItemId }, header: { "X-CSRF-Token": await getCsrfToken() } }
		})
	);
}

export async function createLinkedItems(issueIid: number, workItemIds: number[], linkType: WorkItemLinkType) {
	if (demo) {
		await waitForDemo();
		const selected = new Set(workItemIds);
		const current = demoLinks.get(issueIid) ?? [];
		const existing = new Set(current.map((item) => item.gitLabWorkItemId));
		const additions = demoCandidates
			.filter((candidate) => selected.has(candidate.gitLabWorkItemId) && !existing.has(candidate.gitLabWorkItemId))
			.map((item) => ({ ...item, linkType }));
		demoLinks.set(issueIid, [...current, ...additions]);
		return;
	}
	ensureNoContent(
		await api.POST("/cards/{issueIid}/linked-items", {
			params: { path: { issueIid }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: { workItemIds, linkType }
		})
	);
}

export async function deleteLinkedItem(issueIid: number, workItemId: number) {
	if (demo) {
		await waitForDemo();
		demoLinks.set(
			issueIid,
			(demoLinks.get(issueIid) ?? []).filter((item) => item.gitLabWorkItemId !== workItemId)
		);
		return;
	}
	ensureNoContent(
		await api.DELETE("/cards/{issueIid}/linked-items/{workItemId}", {
			params: { path: { issueIid, workItemId }, header: { "X-CSRF-Token": await getCsrfToken() } }
		})
	);
}

function ensureNoContent(result: ApiResult<unknown>) {
	if (result.response.ok && result.error === undefined) return;
	expectData(result);
}

function waitForDemo() {
	return new Promise((resolve) => window.setTimeout(resolve, demoDelay));
}

function workItem(id: number, iid: number, type: "issue" | "task", title: string): WorkItemSummary {
	return {
		gitLabWorkItemId: id,
		iid,
		type,
		title,
		state: "open",
		webUrl: `https://gitlab.com/sitcon-tw/2027/-/work_items/${iid}`,
		status: { name: "To do", category: "to_do", color: null },
		assignees: [
			{
				gitLabUserId: 114,
				username: "yorukot",
				displayName: "Yorukot",
				avatarUrl: null,
				profileUrl: "https://gitlab.com/yorukot"
			}
		],
		startDate: null,
		dueDate: "2026-09-04",
		labels: [{ name: "Backend", color: "#428BCA", textColor: "#FFFFFF" }]
	};
}

let nextDemoChildIID = 205;
const demoChildren = new Map<number, WorkItemSummary[]>([[127, [workItem(9201, 201, "task", "補上退信重試整合測試")]]]);
const demoLinks = new Map<number, LinkedWorkItem[]>([
	[
		127,
		[
			{ ...workItem(9128, 128, "issue", "整理志工行前通知"), linkType: "relates_to" },
			{ ...workItem(9202, 202, "task", "確認 SMTP 供應商限制"), linkType: "is_blocked_by" }
		]
	]
]);
const demoCandidates: WorkItemSummary[] = [
	workItem(9203, 203, "task", "整理錯誤碼對照表"),
	workItem(9204, 204, "task", "補上寄信 metrics"),
	workItem(9129, 129, "issue", "確認議程講者資料"),
	workItem(9130, 130, "issue", "製作工作人員識別證")
];
