import type { components } from "@/shared/api/openapi";
import type { BoardCard, Bootstrap } from "./model";

export type SyncAction = components["schemas"]["SyncDeltaResponse"]["actions"][number];

export interface SyncGuard {
	dragging: boolean;
	inflightOperationIds: { has(operationId: string): boolean };
	// A direct mutation response knows which local operation it is completing and must
	// not overwrite a newer operation that has already settled. Stream deltas omit it.
	requiredLocalOperationId?: string;
}

/**
 * Decides whether an incoming server snapshot may replace a local card.
 *
 * The same operation is newer server knowledge. A different operation only wins once
 * this tab no longer owns the local pending/failed operation.
 */
export function shouldApplyIncomingCard(local: BoardCard, incomingOperationId: string | null, guard: SyncGuard) {
	if (guard.requiredLocalOperationId !== undefined) return local.pendingOperationId === guard.requiredLocalOperationId;
	if (local.pendingOperationId === null) return true;
	if (local.pendingOperationId === incomingOperationId) return true;
	return !guard.inflightOperationIds.has(local.pendingOperationId);
}

/** Applies one authoritative delta page to the bootstrap cache. */
export function applySyncActions(current: Bootstrap, checkpoint: string, actions: SyncAction[], guard: SyncGuard): Bootstrap {
	// Drag calculations use the exact card order visible at drag start. Ignoring the
	// whole frame, including its checkpoint, lets the caller fetch it again after drop.
	if (guard.dragging) return current;
	if (compareCheckpoints(checkpoint, current.revision) <= 0) return current;

	const unseen = actions.filter((action) => compareCheckpoints(action.syncId, current.revision) > 0);
	// Keep transaction order. In particular, create completion is a negative-IID
	// delete followed by a positive-IID upsert; collapsing each entity independently
	// can erase that identity edge when a later update is present in the same replay.
	const completionRekeys = findCompletionRekeys(unseen);

	let cards = current.board.cards;
	let lists = current.board.lists;
	let teams = current.teams;
	let members = current.members;
	let preferences = current.preferences;
	let sync = current.sync;

	for (const action of unseen) {
		switch (action.entity) {
			case "card":
				cards = applyCardAction(cards, action, guard, completionRekeys.get(`${action.syncId}\u0000${action.entityId}`));
				break;
			case "cardOrder": {
				const positions = new Map(action.order.issueIids.map((issueIid, index) => [issueIid, index]));
				cards = cards.map((card) => {
					const position = positions.get(card.issueIid);
					return card.listKey === action.order.listKey && position !== undefined ? { ...card, position } : card;
				});
				break;
			}
			case "list":
				lists =
					action.operation === "delete" || action.list === null
						? removeBy(lists, (list) => list.key === action.entityId)
						: upsertBy(lists, action.list, (list) => list.key === action.list?.key);
				break;
			case "team":
				teams =
					action.operation === "delete" || action.team === null
						? removeBy(teams, (team) => team.key === action.entityId)
						: upsertBy(teams, action.team, (team) => team.key === action.team?.key);
				break;
			case "member":
				members =
					action.operation === "delete" || action.member === null
						? removeBy(members, (member) => String(member.gitLabUserId) === action.entityId)
						: upsertBy(members, action.member, (member) => member.gitLabUserId === action.member?.gitLabUserId);
				break;
			case "preference":
				preferences = action.preferences;
				break;
			case "syncStatus":
				sync = action.sync;
				break;
		}
	}

	return {
		...current,
		revision: checkpoint,
		teams,
		members,
		preferences,
		sync,
		board: { ...current.board, lists, cards }
	};
}

function applyCardAction(cards: BoardCard[], action: Extract<SyncAction, { entity: "card" }>, guard: SyncGuard, completionRekeyFrom?: number) {
	if (action.operation === "delete" || action.card === null) {
		const issueIid = Number(action.entityId);
		return cards.filter((card) => card.issueIid !== issueIid || !shouldApplyIncomingCard(card, null, guard));
	}

	const incoming = action.card;
	if (completionRekeyFrom !== undefined) {
		const rekeyIndex = cards.findIndex((card) => card.issueIid === completionRekeyFrom);
		if (rekeyIndex >= 0) {
			return cards.flatMap((card, index) => {
				if (index === rekeyIndex) return [incoming];
				return card.issueIid === incoming.issueIid ? [] : [card];
			});
		}
	}
	if (incoming.pendingOperationId !== null) {
		// The browser and server allocate temporary negative IIDs independently. Match
		// the operation first and adopt the server IID in place, for both create and the
		// later negative-to-positive completion rekey.
		const rekeyIndex = cards.findIndex((card) => card.pendingOperationId === incoming.pendingOperationId && card.issueIid !== incoming.issueIid);
		if (rekeyIndex >= 0) {
			return cards.flatMap((card, index) => {
				if (index === rekeyIndex) return [incoming];
				return card.issueIid === incoming.issueIid ? [] : [card];
			});
		}
	}

	const existingIndex = cards.findIndex((card) => card.issueIid === incoming.issueIid);
	if (existingIndex < 0) return [...cards, incoming];
	if (!shouldApplyIncomingCard(cards[existingIndex]!, incoming.pendingOperationId, guard)) return cards;
	return cards.map((card, index) => (index === existingIndex ? incoming : card));
}

function upsertBy<T>(values: T[], incoming: T, matches: (value: T) => boolean) {
	const index = values.findIndex(matches);
	if (index < 0) return [...values, incoming];
	return values.map((value, current) => (current === index ? incoming : value));
}

function removeBy<T>(values: T[], matches: (value: T) => boolean) {
	return values.filter((value) => !matches(value));
}

// Completing a create is the only transaction that deletes one negative card IID and
// upserts one positive card IID. The persisted snapshot correctly clears
// pendingOperationId, so this transaction shape is the remaining identity link when
// its SSE frame races ahead of the POST response.
function findCompletionRekeys(actions: SyncAction[]) {
	const groups = new Map<string, Array<Extract<SyncAction, { entity: "card" }>>>();
	for (const action of actions) {
		if (action.entity !== "card") continue;
		const group = groups.get(action.syncId) ?? [];
		group.push(action);
		groups.set(action.syncId, group);
	}
	const rekeys = new Map<string, number>();
	for (const [syncId, group] of groups) {
		const deleted = group.filter((action) => action.operation === "delete" && Number(action.entityId) < 0);
		const upserted = group.filter((action) => action.operation === "upsert" && action.card !== null && action.card.issueIid > 0);
		if (deleted.length === 1 && upserted.length === 1) {
			rekeys.set(`${syncId}\u0000${upserted[0]!.entityId}`, Number(deleted[0]!.entityId));
		}
	}
	return rekeys;
}

export function isImmediateSuccessor(previous: string, next: string) {
	try {
		return BigInt(next) === BigInt(previous) + 1n;
	} catch {
		return false;
	}
}

export function compareCheckpoints(left: string, right: string) {
	const normalizedLeft = normalizeCheckpoint(left);
	const normalizedRight = normalizeCheckpoint(right);
	if (normalizedLeft.length !== normalizedRight.length) return normalizedLeft.length < normalizedRight.length ? -1 : 1;
	if (normalizedLeft === normalizedRight) return 0;
	return normalizedLeft < normalizedRight ? -1 : 1;
}

function normalizeCheckpoint(value: string) {
	return /^\d+$/.test(value) ? value.replace(/^0+(?=\d)/, "") : value;
}
