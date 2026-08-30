import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { autocompleteAt, suggestionRequestAt } from "./quickActionAutocomplete";

const commands = [
	{ name: "assign", aliases: [], params: ["@user"], description: "Assign users", warning: null, icon: null },
	{ name: "label", aliases: ["labels"], params: ["~label"], description: "Add labels", warning: null, icon: null },
	{ name: "status", aliases: [], params: ['"status"'], description: "Set status", warning: null, icon: null }
];
const labels = [
	{
		id: 1,
		name: "Needs review",
		color: "var(--md-sys-color-primary)",
		textColor: "var(--md-sys-color-on-primary)",
		description: null
	}
];
const context = { bootstrap: demoBootstrap, commands, labels };

describe("GitLab editor autocomplete", () => {
	it("suggests commands only at the beginning of a line", () => {
		expect(autocompleteAt("notes\n/as", 9, context)?.items[0]?.label).toContain("/assign");
		expect(autocompleteAt("text /as", 8, context)).toBeNull();
	});

	it("suggests command parameters and quotes labels", () => {
		expect(autocompleteAt("/assign ", 8, context)?.items[0]?.insertText).toMatch(/^@/);
		expect(autocompleteAt("/label ~Needs", 13, context)?.items[0]?.insertText).toBe('~"Needs review"');
		expect(autocompleteAt("/status ", 8, context)?.items.map((item) => item.insertText)).toContain('"Doing"');
	});

	it("supports references in ordinary Markdown text", () => {
		expect(autocompleteAt("ping @yo", 8, context)?.items[0]?.insertText).toBe("@yorukot");
		expect(autocompleteAt(`see ${"#"}12`, 7, context)?.items.some((item) => item.insertText === `${"#"}127`)).toBe(true);
	});

	it("routes GitLab reference characters and command parameters to typed providers", () => {
		expect(suggestionRequestAt("release %Spr", 12, commands)).toEqual({ kind: "milestone", query: "Spr" });
		expect(suggestionRequestAt("/assign ", 8, commands)).toEqual({ kind: "member", query: "" });
		expect(
			autocompleteAt("release %Spr", 12, {
				...context,
				suggestions: [{ id: "7", kind: "milestone", value: '%"Sprint 1"', label: "Sprint 1", detail: "Active milestone", avatarUrl: null, color: null }]
			})?.items[0]?.insertText
		).toBe('%"Sprint 1"');
	});
});
