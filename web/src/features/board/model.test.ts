import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import {
	compareBoardCards,
	limitRecentBoardCards,
	preferredAssignees,
	taipeiDateAfter,
	teamLeaders,
	teamMembers,
	type BoardCard,
	type BoardSortMode
} from "./model";

describe("board defaults", () => {
	it("uses the current user only for their primary team", () => {
		expect(preferredAssignees(demoBootstrap, "development")).toEqual([demoBootstrap.me.gitLabUserId]);
		expect(preferredAssignees(demoBootstrap, "design")).toEqual([]);
	});

	it("formats the seven-day default in the Taipei calendar", () => {
		const beforeTaipeiMidnight = new Date("2026-07-14T15:30:00Z");
		expect(taipeiDateAfter(7, beforeTaipeiMidnight)).toBe("2026-07-21");
	});

	it("keeps assignment members sourced from active directory data", () => {
		expect(teamMembers(demoBootstrap, "development").map((member) => member.username)).toEqual(["yorukot", "ming", "anita"]);
	});

	it("uses only explicitly configured active team leaders", () => {
		expect(teamLeaders(demoBootstrap, "development").map((member) => member.username)).toEqual(["yorukot"]);
	});

	it.each<[BoardSortMode, number[]]>([
		["manual", [3, 2, 1]],
		["due-asc", [1, 2, 3]],
		["due-desc", [2, 1, 3]],
		["start-asc", [1, 2, 3]],
		["start-desc", [2, 1, 3]],
		["updated-desc", [2, 3, 1]],
		["updated-asc", [1, 3, 2]]
	])("sorts cards by %s with missing dates last", (sortMode, expected) => {
		const template = demoBootstrap.board.cards[0]!;
		const cards: BoardCard[] = [
			{ ...template, issueIid: 1, position: 2, dueDate: "2026-07-20", startDate: "2026-07-10", updatedAt: "2026-07-14T01:00:00Z" },
			{ ...template, issueIid: 2, position: 1, dueDate: "2026-07-22", startDate: "2026-07-12", updatedAt: "2026-07-14T03:00:00Z" },
			{ ...template, issueIid: 3, position: 0, dueDate: null, startDate: null, updatedAt: "2026-07-14T02:00:00Z" }
		];

		expect(cards.sort((a, b) => compareBoardCards(a, b, sortMode)).map((card) => card.issueIid)).toEqual(expected);
	});

	it("uses GitLab IID instead of manual position to break non-manual ties", () => {
		const template = demoBootstrap.board.cards[0]!;
		const cards: BoardCard[] = [
			{ ...template, issueIid: 2, position: 0, dueDate: "2026-07-20" },
			{ ...template, issueIid: 1, position: 1, dueDate: "2026-07-20" }
		];

		expect([...cards].sort((a, b) => compareBoardCards(a, b, "manual")).map((card) => card.issueIid)).toEqual([2, 1]);
		expect([...cards].sort((a, b) => compareBoardCards(a, b, "due-asc")).map((card) => card.issueIid)).toEqual([1, 2]);
	});

	it("limits a lane to the most recently updated cards without changing presentation order", () => {
		const template = demoBootstrap.board.cards[0]!;
		const cards: BoardCard[] = [
			{ ...template, issueIid: 3, updatedAt: "2026-07-14T02:00:00Z" },
			{ ...template, issueIid: 1, updatedAt: "2026-07-14T01:00:00Z" },
			{ ...template, issueIid: 2, updatedAt: "2026-07-14T03:00:00Z" }
		];

		expect(limitRecentBoardCards(cards, 2).map((card) => card.issueIid)).toEqual([3, 2]);
		expect(limitRecentBoardCards(cards, 3)).toBe(cards);
	});
});
