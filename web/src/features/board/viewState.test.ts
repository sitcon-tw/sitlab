import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { parseBoardViewState, serializeBoardViewState } from "./viewState";

describe("board view URL state", () => {
	it("parses valid filters and ignores invalid directory values", () => {
		expect(
			parseBoardViewState("?q=%20Backend%20&team=development&member=101&member=101&member=999999&label=Priority%3A%3AHigh&label=&sort=due-desc", demoBootstrap)
		).toEqual({
			query: "Backend",
			teamKey: "development",
			memberIds: [101],
			labels: ["Priority::High"],
			sortMode: "due-desc"
		});

		expect(parseBoardViewState("?team=missing&member=nope&sort=sideways", demoBootstrap)).toEqual({
			query: "",
			teamKey: "",
			memberIds: [],
			labels: [],
			sortMode: "due-asc"
		});
	});

	it("serializes selections while preserving unrelated parameters and omitting defaults", () => {
		expect(
			serializeBoardViewState("?demo=1&q=old&team=old&member=2&label=old&sort=manual", {
				query: " Backend ",
				teamKey: "design",
				memberIds: [202, 101, 202],
				labels: ["Priority::High", "Backend", "Priority::High"],
				sortMode: "updated-desc"
			})
		).toBe("?demo=1&q=Backend&team=design&member=202&member=101&label=Priority%3A%3AHigh&label=Backend&sort=updated-desc");

		expect(serializeBoardViewState("?q=old&team=old", { query: "", teamKey: "", memberIds: [], labels: [], sortMode: "due-asc" })).toBe("");
		expect(serializeBoardViewState("", { query: "", teamKey: "", memberIds: [], labels: [], sortMode: "manual" })).toBe("?sort=manual");
	});
});
