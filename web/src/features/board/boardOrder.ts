import { compareBoardCards, type BoardCard } from "./model";

export type CardGroups = Record<string, number[]>;

export interface CardPositionPatch {
	issueIid: number;
	listKey: string;
	position: number;
}

export interface CardMovePlan {
	card: BoardCard;
	listKey: string;
	position: number;
	patches: CardPositionPatch[];
}

export function groupCardIds(cards: BoardCard[], listKeys: string[]): CardGroups {
	return Object.fromEntries(
		listKeys.map((listKey) => [
			listKey,
			cards
				.filter((card) => card.listKey === listKey)
				.sort((a, b) => compareBoardCards(a, b, "manual"))
				.map((card) => card.issueIid)
		])
	);
}

export function locateCard(groups: CardGroups, cardIid: number) {
	for (const [listKey, cardIids] of Object.entries(groups)) {
		const index = cardIids.indexOf(cardIid);
		if (index >= 0) return { listKey, index };
	}
	return null;
}

export function canonicalPositionForVisibleOrder(cards: BoardCard[], groups: CardGroups, cardIid: number, listKey: string) {
	const visible = groups[listKey];
	if (!visible) return null;
	const visibleIndex = visible.indexOf(cardIid);
	if (visibleIndex < 0) return null;

	const destination = cards.filter((card) => card.listKey === listKey && card.issueIid !== cardIid).sort((a, b) => compareBoardCards(a, b, "manual"));
	const nextVisibleIid = visible[visibleIndex + 1];
	if (nextVisibleIid !== undefined) {
		const nextIndex = destination.findIndex((card) => card.issueIid === nextVisibleIid);
		if (nextIndex >= 0) return nextIndex;
	}
	const previousVisibleIid = visible[visibleIndex - 1];
	if (previousVisibleIid !== undefined) {
		const previousIndex = destination.findIndex((card) => card.issueIid === previousVisibleIid);
		if (previousIndex >= 0) return previousIndex + 1;
	}
	return destination.length;
}

export function planCardMove(cards: BoardCard[], cardIid: number, listKey: string, requestedPosition: number): CardMovePlan | null {
	const card = cards.find((item) => item.issueIid === cardIid);
	if (!card) return null;

	const sourceCards = cards
		.filter((item) => item.listKey === card.listKey && item.issueIid !== card.issueIid)
		.sort((a, b) => compareBoardCards(a, b, "manual"));
	const destinationCards =
		listKey === card.listKey
			? sourceCards
			: cards.filter((item) => item.listKey === listKey && item.issueIid !== card.issueIid).sort((a, b) => compareBoardCards(a, b, "manual"));
	const position = Math.max(0, Math.min(requestedPosition, destinationCards.length));
	if (card.listKey === listKey && position === card.position) return null;

	const nextDestination = [...destinationCards];
	nextDestination.splice(position, 0, card);
	const patches: CardPositionPatch[] = [];
	if (card.listKey !== listKey) {
		sourceCards.forEach((item, index) => patches.push({ issueIid: item.issueIid, listKey: card.listKey, position: index }));
	}
	nextDestination.forEach((item, index) => patches.push({ issueIid: item.issueIid, listKey, position: index }));

	return { card, listKey, position, patches };
}
