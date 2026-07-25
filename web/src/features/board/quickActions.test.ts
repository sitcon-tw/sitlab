import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import { parseQuickAction } from "./quickActions";

const card = demoBootstrap.board.cards[0]!;

describe("GitLab-style quick actions", () => {
	it("resolves assignees by GitLab username", () => {
		expect(parseQuickAction("/assign @ming", demoBootstrap, card)).toEqual({ action: { kind: "assign", memberIds: [114, 115] } });
		expect(parseQuickAction("/unassign @yorukot", demoBootstrap, card)).toEqual({ action: { kind: "assign", memberIds: [] } });
		expect(parseQuickAction("/unassign", demoBootstrap, { ...card, assigneeGitLabUserIds: [114, 115] })).toEqual({ action: { kind: "assign", memberIds: [] } });
		expect(parseQuickAction("/assign @missing", demoBootstrap, card)).toEqual({ error: "找不到 @missing" });
	});

	it("validates dates and maps removal commands", () => {
		expect(parseQuickAction("/due 2026-07-31", demoBootstrap, card)).toEqual({ action: { kind: "due", value: "2026-07-31" } });
		expect(parseQuickAction("/due 2026-02-30", demoBootstrap, card)).toEqual({ error: "日期不存在" });
		expect(parseQuickAction("/start_date 2026-07-20", demoBootstrap, card)).toEqual({ action: { kind: "start", value: "2026-07-20" } });
		expect(parseQuickAction("/remove_start_date", demoBootstrap, card)).toEqual({ action: { kind: "start", value: null } });
	});

	it("maps close and reopen to board lists", () => {
		expect(parseQuickAction("/close", demoBootstrap, card)).toEqual({ action: { kind: "move", listKey: "closed" } });
		expect(parseQuickAction("/reopen", demoBootstrap, { ...card, listKey: "closed" })).toEqual({ action: { kind: "move", listKey: "wating" } });
	});
});
