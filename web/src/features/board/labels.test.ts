import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { canonicalClientLabels, isDeprecatedLabel, isReservedLabel } from "./labels";

// This table mirrors server/internal/domain/board/labels_test.go so drift
// between the two implementations shows up as a diff on both sides.
const reservedCases: Array<[string, boolean, string]> = [
	["Team::開發組", true, "a configured team label"],
	["Team::新組", true, "the Team:: prefix is reserved even when unconfigured"],
	["Status::Inbox", true, "lifecycle labels are owned by the board"],
	["To Do", true, "legacy workflow label"],
	["組別::開發", true, "legacy team label"],
	["   ", true, "blank names are never valid"],
	["Backend", false, "an ordinary label"],
	["Priority::High", false, "a namespaced label that is not Team:: or Status::"]
];

describe("label rules", () => {
	it.each(reservedCases)("isReservedLabel(%s) === %s (%s)", (name, expected) => {
		expect(isReservedLabel(demoBootstrap, name)).toBe(expected);
	});

	it("treats legacy workflow and lifecycle names as deprecated", () => {
		expect(isDeprecatedLabel("Status::Doing")).toBe(true);
		expect(isDeprecatedLabel("Wating")).toBe(true);
		expect(isDeprecatedLabel("Backend")).toBe(false);
	});

	it("keeps general labels and swaps in exactly the active team's label", () => {
		const team = demoBootstrap.teams[0]!;
		const other = demoBootstrap.teams[1]!;
		const result = canonicalClientLabels(demoBootstrap, ["Backend", other.gitLabLabel, "Status::Inbox", "To Do"], team.key);
		expect(result).toEqual(["Backend", team.gitLabLabel]);
	});

	it("drops the team label when the target team is unknown", () => {
		expect(canonicalClientLabels(demoBootstrap, ["Backend", "Team::開發組"], "not-a-team")).toEqual(["Backend"]);
	});
});
