import { move } from "@dnd-kit/helpers";
import type { DragEndEvent, DragOverEvent, DragStartEvent } from "@dnd-kit/react";
import { useEffect, useRef, useState } from "react";
import { canonicalPositionForVisibleOrder, groupCardIds, locateCard, type CardGroups } from "./boardOrder";
import type { BoardCard } from "./model";

export interface BoardDragOptions {
	cards: BoardCard[];
	visibleCards: BoardCard[];
	listKeys: string[];
	enabled: boolean;
	onMove: (cardIid: number, listKey: string, position: number) => void;
	onDraggingChange: (dragging: boolean) => void;
}

export function useBoardDrag({ cards, visibleCards, listKeys, enabled, onMove, onDraggingChange }: BoardDragOptions) {
	const [activeCardIid, setActiveCardIid] = useState<number | null>(null);
	const [dragGroups, setDragGroups] = useState<CardGroups | null>(null);
	const dragGroupsRef = useRef<CardGroups | null>(null);

	const clearDrag = () => {
		dragGroupsRef.current = null;
		setDragGroups(null);
		setActiveCardIid(null);
		onDraggingChange(false);
	};

	const onDragStart = (event: DragStartEvent) => {
		if (!enabled || typeof event.operation.source?.id !== "number") return;
		const groups = groupCardIds(visibleCards, listKeys);
		dragGroupsRef.current = groups;
		setDragGroups(groups);
		setActiveCardIid(event.operation.source.id);
		onDraggingChange(true);
	};

	const onDragOver = (event: DragOverEvent) => {
		const current = dragGroupsRef.current;
		if (!enabled || !current) return;
		const next = move(current, event) as CardGroups;
		if (next === current) return;
		dragGroupsRef.current = next;
		setDragGroups(next);
	};

	const onDragEnd = (event: DragEndEvent) => {
		const groups = dragGroupsRef.current;
		const sourceId = event.operation.source?.id;
		if (!event.canceled && groups && typeof sourceId === "number") {
			const target = locateCard(groups, sourceId);
			if (target) {
				const position = canonicalPositionForVisibleOrder(cards, groups, sourceId, target.listKey);
				if (position !== null) onMove(sourceId, target.listKey, position);
			}
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
