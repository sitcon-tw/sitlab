import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { createGanttViewModel, dayToPlainDate, openBoardCards, plainDateToDay } from "./ganttModel";
import type { BoardCard } from "./model";

function card(overrides: Partial<BoardCard>): BoardCard {
	return { ...demoBootstrap.board.cards[0]!, ...overrides };
}

describe("Gantt view model", () => {
	it("keeps cards in non-closed lists without relying on a list key", () => {
		const open = openBoardCards(demoBootstrap.board.cards, demoBootstrap.board.lists);
		expect(open.map((item) => item.issueIid)).not.toContain(133);
		expect(open).toHaveLength(6);
	});

	it("classifies dated and unscheduled work", () => {
		const cards = [
			card({ issueIid: 1, startDate: "2026-08-03", dueDate: "2026-08-07" }),
			card({ issueIid: 2, startDate: null, dueDate: "2026-08-10" }),
			card({ issueIid: 3, startDate: "2026-08-11", dueDate: null }),
			card({ issueIid: 4, startDate: "2026-08-20", dueDate: "2026-08-12" }),
			card({ issueIid: 5, startDate: null, dueDate: null })
		];
		const model = createGanttViewModel(cards, demoBootstrap.teams, "2026-08-06");

		expect(model.groups[0]?.scheduled.map((item) => item.kind)).toEqual(["range", "due-milestone", "start-marker", "invalid"]);
		expect(model.groups[0]?.unscheduled.map((item) => item.issueIid)).toEqual([5]);
		expect(model.orderedCards.map((item) => item.issueIid)).toEqual([1, 2, 3, 4, 5]);
	});

	it("aligns the weekly timeline to Monday with one week of padding", () => {
		const model = createGanttViewModel([card({ startDate: "2026-08-05", dueDate: "2026-08-12" })], demoBootstrap.teams, "2026-08-06");

		expect(dayToPlainDate(model.timeline!.startDay)).toBe("2026-07-27");
		expect(dayToPlainDate(model.timeline!.endDay)).toBe("2026-08-24");
		expect(model.timeline?.weeks).toHaveLength(4);
		expect(model.timeline?.weeks[0]).toEqual({ startDate: "2026-07-27", endDate: "2026-08-02" });
	});

	it("uses stable UTC day arithmetic for plain dates", () => {
		expect(dayToPlainDate(plainDateToDay("2026-08-28") + 1)).toBe("2026-08-29");
	});
});
