import { describe, expect, it } from "vitest";
import type { BoardCard, Bootstrap } from "./model";
import { applySyncActions, compareCheckpoints, isImmediateSuccessor, type SyncAction, type SyncGuard } from "./syncActions";

const idleGuard: SyncGuard = { dragging: false, inflightOperationIds: new Set() };

function card(overrides: Partial<BoardCard> = {}): BoardCard {
	return {
		issueIid: 1,
		issueId: 101,
		title: "Local title",
		description: "",
		webUrl: "https://gitlab.com/sitcon-tw/2027/-/issues/1",
		listKey: "inbox",
		position: 0,
		teamKey: "development",
		assigneeGitLabUserIds: [],
		startDate: null,
		dueDate: null,
		labels: ["Team::Development"],
		gitLabStatusName: "Inbox",
		syncState: "synced",
		syncError: null,
		pendingOperationId: null,
		createdAt: "2026-08-28T00:00:00Z",
		updatedAt: "2026-08-28T00:00:00Z",
		...overrides
	};
}

function bootstrap(cards: BoardCard[] = [card()]): Bootstrap {
	return {
		revision: "10",
		me: {
			id: "11111111-1111-1111-1111-111111111111",
			gitLabUserId: 1,
			username: "tester",
			displayName: "Tester",
			avatarUrl: null,
			profileUrl: "https://gitlab.com/tester",
			accessLevel: 40
		},
		csrfToken: "test-token",
		teams: [],
		members: [],
		milestones: [],
		board: {
			lists: [
				{ key: "inbox", name: "Inbox", position: 0, closed: false, color: "#000000" },
				{ key: "doing", name: "Doing", position: 1, closed: false, color: "#111111" }
			],
			cards,
			syncedAt: "2026-08-28T00:00:00Z"
		},
		preferences: { defaultTeamKey: null, confirmedAt: "2026-08-28T00:00:00Z", directoryTeamKeys: [] },
		sync: { state: "synced", lastSuccessAt: "2026-08-28T00:00:00Z", message: null }
	};
}

function cardAction(syncId: string, incoming: BoardCard): SyncAction {
	return {
		entity: "card",
		syncId,
		entityId: String(incoming.issueIid),
		operation: "upsert",
		actorGitLabUserId: null,
		occurredAt: "2026-08-28T00:00:00Z",
		card: incoming
	};
}

describe("applySyncActions", () => {
	it("ignores the entire frame, including its checkpoint, while dragging", () => {
		const current = bootstrap();
		const next = applySyncActions(current, "11", [cardAction("11", card({ title: "Server title" }))], {
			dragging: true,
			inflightOperationIds: new Set()
		});
		expect(next).toBe(current);
		expect(next.revision).toBe("10");
	});

	it("keeps a locally owned operation but still advances the checkpoint", () => {
		const local = card({ pendingOperationId: "11111111-1111-1111-1111-111111111112", syncState: "failed", syncError: "retry me" });
		const incoming = card({ title: "Other tab", pendingOperationId: "11111111-1111-1111-1111-111111111113" });
		const next = applySyncActions(bootstrap([local]), "11", [cardAction("11", incoming)], {
			dragging: false,
			inflightOperationIds: new Set([local.pendingOperationId!])
		});
		expect(next.revision).toBe("11");
		expect(next.board.cards[0]).toEqual(local);
	});

	it("keeps a failed local card when a delete does not identify its operation", () => {
		const operationId = "11111111-1111-1111-1111-111111111112";
		const local = card({ pendingOperationId: operationId, syncState: "failed", syncError: "retry me" });
		const deletion: SyncAction = {
			entity: "card",
			syncId: "11",
			entityId: "1",
			operation: "delete",
			actorGitLabUserId: null,
			occurredAt: "2026-08-28T00:00:00Z",
			card: null
		};
		const next = applySyncActions(bootstrap([local]), "11", [deletion], {
			dragging: false,
			inflightOperationIds: new Set([operationId])
		});
		expect(next.board.cards).toEqual([local]);
	});

	it("accepts newer server state for the same operation", () => {
		const operationId = "11111111-1111-1111-1111-111111111112";
		const incoming = card({ title: "Saved", pendingOperationId: operationId, syncState: "processing" });
		const next = applySyncActions(bootstrap([card({ pendingOperationId: operationId, syncState: "pending" })]), "11", [cardAction("11", incoming)], {
			dragging: false,
			inflightOperationIds: new Set([operationId])
		});
		expect(next.board.cards[0]).toEqual(incoming);
	});

	it("rekeys a temporary card in place by operation identity", () => {
		const operationId = "11111111-1111-1111-1111-111111111112";
		const local = card({ issueIid: -1, issueId: null, pendingOperationId: operationId, syncState: "pending" });
		const serverTemporary = card({ issueIid: -7, issueId: null, pendingOperationId: operationId, syncState: "pending" });
		const next = applySyncActions(bootstrap([local, card({ issueIid: 2, issueId: 102 })]), "11", [cardAction("11", serverTemporary)], {
			dragging: false,
			inflightOperationIds: new Set([operationId])
		});
		expect(next.board.cards).toHaveLength(2);
		expect(next.board.cards[0]).toEqual(serverTemporary);
		expect(next.board.cards.some((item) => item.issueIid === -1)).toBe(false);
	});

	it("rekeys a completed create when the stream beats the POST response", () => {
		const operationId = "11111111-1111-1111-1111-111111111112";
		const serverTemporary = card({ issueIid: -7, issueId: null, pendingOperationId: operationId, syncState: "pending" });
		const completed = card({ issueIid: 127, issueId: 9127, title: "Created", pendingOperationId: null, syncState: "synced" });
		const actions: SyncAction[] = [
			{
				entity: "card",
				syncId: "11",
				entityId: "-7",
				operation: "delete",
				actorGitLabUserId: null,
				occurredAt: "2026-08-28T00:00:00Z",
				card: null
			},
			cardAction("11", completed)
		];
		const next = applySyncActions(bootstrap([serverTemporary, card({ issueIid: 2, issueId: 102 })]), "11", actions, {
			dragging: false,
			inflightOperationIds: new Set([operationId])
		});
		expect(next.board.cards).toHaveLength(2);
		expect(next.board.cards[0]).toEqual(completed);
		expect(next.board.cards.some((item) => item.issueIid === -7)).toBe(false);
	});

	it("applies lane order without rebuilding card payloads", () => {
		const current = bootstrap([card({ issueIid: 1, position: 0 }), card({ issueIid: 2, issueId: 102, position: 1 })]);
		const order: SyncAction = {
			entity: "cardOrder",
			syncId: "11",
			entityId: "inbox",
			operation: "upsert",
			actorGitLabUserId: null,
			occurredAt: "2026-08-28T00:00:00Z",
			order: { listKey: "inbox", issueIids: [2, 1] }
		};
		const next = applySyncActions(current, "11", [order], idleGuard);
		expect(next.board.cards.map((item) => [item.issueIid, item.position])).toEqual([
			[1, 1],
			[2, 0]
		]);
	});

	it("ignores overlapping actions that are not newer than the cache", () => {
		const next = applySyncActions(bootstrap(), "12", [cardAction("10", card({ title: "Stale" }))], idleGuard);
		expect(next.revision).toBe("12");
		expect(next.board.cards[0]?.title).toBe("Local title");
	});

	it("replaces the milestone calendar wholesale", () => {
		const calendar: SyncAction = {
			entity: "milestone",
			syncId: "11",
			entityId: "directory",
			operation: "upsert",
			actorGitLabUserId: null,
			occurredAt: "2026-08-28T00:00:00Z",
			milestones: [{ name: "一籌", date: "2026-08-29", kind: "organizing" }]
		};
		const next = applySyncActions(bootstrap(), "11", [calendar], idleGuard);
		expect(next.milestones).toEqual([{ name: "一籌", date: "2026-08-29", kind: "organizing" }]);

		const stale = applySyncActions(next, "12", [{ ...calendar, syncId: "10", milestones: [] }], idleGuard);
		expect(stale.milestones).toEqual(next.milestones);
	});
});

describe("compareCheckpoints", () => {
	it("compares decimal checkpoints numerically without losing bigint precision", () => {
		expect(compareCheckpoints("9", "10")).toBeLessThan(0);
		expect(compareCheckpoints("100000000000000000001", "100000000000000000000")).toBeGreaterThan(0);
		expect(compareCheckpoints("00012", "12")).toBe(0);
		expect(isImmediateSuccessor("999999999999999999999", "1000000000000000000000")).toBe(true);
	});
});
