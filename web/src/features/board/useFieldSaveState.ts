import { useCallback, useEffect, useRef, useState } from "react";

export type SaveField = "details" | "team" | "status" | "assignee" | "startDate" | "dueDate" | "labels";
export type SaveState = "saving" | "saved" | "failed";

export interface FieldSave {
	state: SaveState;
	operationId: string;
	error?: string;
}

const fieldNames: Record<SaveField, string> = {
	details: "標題與描述",
	team: "組別",
	status: "狀態",
	assignee: "Assignee",
	startDate: "Start",
	dueDate: "Due",
	labels: "Labels"
};

const stateNames: Record<SaveState, string> = { saving: "儲存中", saved: "已儲存", failed: "儲存失敗" };

function keyOf(issueIid: number, field: SaveField) {
	return `${issueIid}:${field}`;
}

/**
 * Per-field save state for the card detail drawer.
 *
 * The card's own `syncState` cannot drive this. It is per card, and
 * `pendingOperationId` only remembers the most recent operation — edit Due and
 * then Labels 200ms later and the first response no longer matches, so a
 * syncState-derived indicator would hang on "saving" forever. The 5s bootstrap
 * poll also overwrites syncState wholesale, which would flash "saved" on every
 * field at once.
 *
 * Entries are therefore keyed per field and settled by the operationId that
 * runCardMutation already mints.
 *
 * Hoist this into BoardPage, not CardDetail: the drawer unmounts on close, and
 * the state has to survive closing, reopening, and card-to-card navigation.
 */
export function useFieldSaveState(savedHoldMs = 1600) {
	const [saves, setSaves] = useState<Record<string, FieldSave>>({});
	const [announcement, setAnnouncement] = useState("");
	const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());

	useEffect(() => {
		const pending = timers.current;
		return () => {
			for (const timer of pending.values()) clearTimeout(timer);
			pending.clear();
		};
	}, []);

	const clearTimer = useCallback((key: string) => {
		const timer = timers.current.get(key);
		if (timer === undefined) return;
		clearTimeout(timer);
		timers.current.delete(key);
	}, []);

	const begin = useCallback(
		(issueIid: number, field: SaveField, operationId: string) => {
			const key = keyOf(issueIid, field);
			clearTimer(key);
			setSaves((current) => ({ ...current, [key]: { state: "saving", operationId } }));
			setAnnouncement(`${fieldNames[field]}${stateNames.saving}`);
		},
		[clearTimer]
	);

	const settle = useCallback(
		(operationId: string, state: "saved" | "failed", error?: string) => {
			setSaves((current) => {
				const entry = Object.entries(current).find(([, save]) => save.operationId === operationId);
				if (!entry) return current;
				const [key, save] = entry;
				const field = key.slice(key.indexOf(":") + 1) as SaveField;
				setAnnouncement(`${fieldNames[field]}${stateNames[state]}`);
				if (state === "saved") {
					clearTimer(key);
					timers.current.set(
						key,
						setTimeout(() => {
							timers.current.delete(key);
							setSaves((latest) => {
								if (latest[key]?.operationId !== operationId) return latest;
								const next = { ...latest };
								delete next[key];
								return next;
							});
						}, savedHoldMs)
					);
				}
				return { ...current, [key]: { ...save, state, ...(error ? { error } : {}) } };
			});
		},
		[clearTimer, savedHoldMs]
	);

	/** Flip a failed entry back to saving when the user retries its card. */
	const retry = useCallback((operationId: string | null) => {
		if (!operationId) return;
		setSaves((current) => {
			const entry = Object.entries(current).find(([, save]) => save.operationId === operationId);
			if (!entry) return current;
			const [key, save] = entry;
			return { ...current, [key]: { ...save, state: "saving" } };
		});
	}, []);

	const get = useCallback((issueIid: number, field: SaveField) => saves[keyOf(issueIid, field)], [saves]);

	return { get, begin, settle, retry, announcement };
}

export type FieldSaveState = ReturnType<typeof useFieldSaveState>;
