import { move } from "@dnd-kit/helpers";
import type { DragEndEvent, DragOverEvent, DragStartEvent } from "@dnd-kit/react";
import { useEffect, useRef, useState } from "react";
import { groupCardsForStatusPreview, groupVisibleCardIds, locateCard, planCardDrop, type CardGroups } from "./boardOrder";
import type { BoardCard, BoardSortMode } from "./model";

export interface BoardDragOptions {
	cards: BoardCard[];
	visibleCards: BoardCard[];
	listKeys: string[];
	sortMode: BoardSortMode;
	onMove: (cardIid: number, listKey: string, position?: number) => void;
	onDraggingChange: (dragging: boolean) => void;
}

export function useBoardDrag({ cards, visibleCards, listKeys, sortMode, onMove, onDraggingChange }: BoardDragOptions) {
	const [activeCardIid, setActiveCardIid] = useState<number | null>(null);
	const [dragGroups, setDragGroups] = useState<CardGroups | null>(null);
	const dragGroupsRef = useRef<CardGroups | null>(null);
	const initialGroupsRef = useRef<CardGroups | null>(null);
	const sourceListKeyRef = useRef<string | null>(null);

	const clearDrag = () => {
		dragGroupsRef.current = null;
		initialGroupsRef.current = null;
		sourceListKeyRef.current = null;
		setDragGroups(null);
		setActiveCardIid(null);
		onDraggingChange(false);
	};

	const onDragStart = (event: DragStartEvent) => {
		if (typeof event.operation.source?.id !== "number") return;
		const groups = groupVisibleCardIds(visibleCards, listKeys);
		const source = locateCard(groups, event.operation.source.id);
		if (!source) return;
		dragGroupsRef.current = groups;
		initialGroupsRef.current = groups;
		sourceListKeyRef.current = source.listKey;
		setDragGroups(groups);
		setActiveCardIid(event.operation.source.id);
		onDraggingChange(true);
	};

	const onDragOver = (event: DragOverEvent) => {
		const current = dragGroupsRef.current;
		if (!current) return;
		const sourceId = event.operation.source?.id;
		if (typeof sourceId !== "number") return;

		let next: CardGroups;
		if (sortMode === "manual") {
			next = move(current, event) as CardGroups;
		} else {
			const sourceListKey = sourceListKeyRef.current;
			const initial = initialGroupsRef.current;
			if (!sourceListKey || !initial) return;
			const targetListKey = listKeyForTarget(current, event.operation.target?.id);
			if (!targetListKey) return;
			next = targetListKey === sourceListKey ? initial : groupCardsForStatusPreview(visibleCards, listKeys, sourceId, targetListKey, sortMode);
		}
		if (sameGroups(current, next, listKeys)) return;
		dragGroupsRef.current = next;
		setDragGroups(next);
	};

	const onDragEnd = (event: DragEndEvent) => {
		const groups = dragGroupsRef.current;
		const sourceId = event.operation.source?.id;
		if (!event.canceled && groups && typeof sourceId === "number") {
			const sourceListKey = sourceListKeyRef.current;
			const intent = sourceListKey ? planCardDrop(cards, groups, sourceId, sourceListKey, sortMode) : null;
			if (intent) onMove(sourceId, intent.listKey, intent.position);
		}
		clearDrag();
	};

	useEffect(
		() => () => {
			if (dragGroupsRef.current) onDraggingChange(false);
		},
		[onDraggingChange]
	);

	return { activeCardIid, dragGroups, onDragStart, onDragOver, onDragEnd };
}

function listKeyForTarget(groups: CardGroups, targetId: unknown) {
	if (typeof targetId === "string" && Object.hasOwn(groups, targetId)) return targetId;
	if (typeof targetId === "number") {
		for (const [listKey, cardIids] of Object.entries(groups)) {
			if (cardIids.includes(targetId)) return listKey;
		}
	}
	return null;
}

function sameGroups(a: CardGroups, b: CardGroups, listKeys: string[]) {
	return listKeys.every((listKey) => {
		const left = a[listKey] ?? [];
		const right = b[listKey] ?? [];
		return left.length === right.length && left.every((cardIid, index) => cardIid === right[index]);
	});
}
