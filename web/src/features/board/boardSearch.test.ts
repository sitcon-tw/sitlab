import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { boardSearchTerms, createBoardSearchIndex, matchesBoardSearch } from "./boardSearch";

describe("board card search", () => {
	const index = createBoardSearchIndex(demoBootstrap.board.cards, demoBootstrap.teams, demoBootstrap.members);

	it("matches issue numbers and normalized card content", () => {
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("＃１２７"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("報名系統"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("ＢＡＣＫＥＮＤ"))).toBe(true);
	});

	it("requires every term while allowing terms to match different fields", () => {
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("開發 Yorukot Priority"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("開發 Alice"))).toBe(false);
	});

	it("matches descriptions, usernames, and labels but not comments or dates", () => {
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("觀測紀錄"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("@yorukot"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("priority::high"))).toBe(true);
		expect(matchesBoardSearch(index.get(127), boardSearchTerms("2026-07-21"))).toBe(false);
	});
});
