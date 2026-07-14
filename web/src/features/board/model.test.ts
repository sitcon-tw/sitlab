import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { preferredAssignees, taipeiDateAfter, teamMembers } from "./model";

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
});
