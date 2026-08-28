import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { canonicalPositionForVisibleOrder, groupCardIds, groupVisibleCardIds, locateCard, planCardMove } from "./boardOrder";

describe("board order", () => {
	const cards = structuredClone(demoBootstrap.board.cards);
	const listKeys = demoBootstrap.board.lists.map((list) => list.key);

	it("groups cards in canonical manual order", () => {
		const groups = groupCardIds(cards, listKeys);

		expect(groups.doing).toEqual([130, 131]);
		expect(locateCard(groups, 131)).toEqual({ listKey: "doing", index: 1 });
	});

	it("captures the currently rendered order before switching a drag to manual sorting", () => {
		const visible = [cards.find((card) => card.issueIid === 131)!, cards.find((card) => card.issueIid === 130)!];

		expect(groupVisibleCardIds(visible, listKeys).doing).toEqual([131, 130]);
	});

	it("plans a same-lane reorder with contiguous positions", () => {
		const plan = planCardMove(cards, 130, "doing", 1);

		expect(plan?.position).toBe(1);
		expect(plan?.patches.filter((patch) => patch.listKey === "doing")).toEqual([
			{ issueIid: 131, listKey: "doing", position: 0 },
			{ issueIid: 130, listKey: "doing", position: 1 }
		]);
	});

	it("plans a cross-lane move and reindexes both lanes", () => {
		const plan = planCardMove(cards, 130, "todo", 1);

		expect(plan?.position).toBe(1);
		expect(plan?.patches.find((patch) => patch.issueIid === 130)).toEqual({ issueIid: 130, listKey: "todo", position: 1 });
		expect(plan?.patches.filter((patch) => patch.listKey === "doing")).toEqual([{ issueIid: 131, listKey: "doing", position: 0 }]);
	});

	it("maps a filtered preview back into the full lane", () => {
		const groups = groupCardIds(cards, listKeys);
		groups.todo = [127, 130, 129];

		expect(canonicalPositionForVisibleOrder(cards, groups, 130, "todo")).toBe(1);
	});

	it("places a card at the end when a filtered lane has no visible anchors", () => {
		const groups = groupCardIds(cards, listKeys);
		groups.review = [130];

		const reviewCount = cards.filter((card) => card.listKey === "review").length;
		expect(canonicalPositionForVisibleOrder(cards, groups, 130, "review")).toBe(reviewCount);
	});

	it("rejects missing cards and no-op moves", () => {
		expect(planCardMove(cards, 999_999, "doing", 0)).toBeNull();
		expect(planCardMove(cards, 130, "doing", 0)).toBeNull();
	});
});
