import { demoBootstrap } from "@/test/demoBootstrap";
import { describe, expect, it } from "vitest";
import type { BoardCard, ProjectLabel } from "./model";
import { buildShareCardData, layoutChipRows, shareCardFilename, wrapEllipsis, type ShareCardLabel } from "./shareCard";

function label(name: string, color: string): ProjectLabel {
	return { id: 1, name, color, textColor: "#ffffff", description: null };
}

const developmentCard = demoBootstrap.board.cards.find((card) => card.issueIid === 127)!;

function cardWith(overrides: Partial<BoardCard>): BoardCard {
	return { ...developmentCard, ...overrides };
}

describe("buildShareCardData", () => {
	it("resolves team prefix, list name, assignees, and label colors", () => {
		const metadata = new Map([
			["Priority::High", label("Priority::High", "#ED9121")],
			["Backend", label("Backend", "not-a-color")]
		]);
		const data = buildShareCardData(demoBootstrap, developmentCard, metadata);
		expect(data.iidText).toBe("#127");
		expect(data.title).toBe("[開發組] 修正報名系統寄信流程");
		expect(data.description).toBe("釐清失敗重送條件，補上整合測試與觀測紀錄。");
		expect(data.teamName).toBe("開發組");
		expect(data.listName).toBe("To do");
		expect(data.assignees).toEqual([{ name: "Yorukot", avatarUrl: null }]);
		expect(data.dueDate).toBe("2026-07-21");
		// The team label is reserved; invalid GitLab colors degrade to null.
		expect(data.labels).toEqual([
			{ name: "Priority::High", color: "#ED9121" },
			{ name: "Backend", color: null }
		]);
	});

	it("flattens description whitespace into one run", () => {
		const data = buildShareCardData(demoBootstrap, cardWith({ description: "第一行\n\n- 第二行\t結尾  " }), new Map());
		expect(data.description).toBe("第一行 - 第二行 結尾");
	});

	it("keeps an already prefixed title untouched", () => {
		const data = buildShareCardData(demoBootstrap, cardWith({ title: "[開發組] 修正報名系統寄信流程" }), new Map());
		expect(data.title).toBe("[開發組] 修正報名系統寄信流程");
	});

	it("labels an unsynced card #new", () => {
		expect(buildShareCardData(demoBootstrap, cardWith({ issueIid: -1 }), new Map()).iidText).toBe("#new");
	});

	it("drops assignee ids with no directory member", () => {
		const data = buildShareCardData(demoBootstrap, cardWith({ assigneeGitLabUserIds: [114, 999_999] }), new Map());
		expect(data.assignees).toEqual([{ name: "Yorukot", avatarUrl: null }]);
	});

	it("marks a past due date overdue only while the list is open", () => {
		expect(buildShareCardData(demoBootstrap, cardWith({ dueDate: "2000-01-01" }), new Map()).overdue).toBe(true);
		expect(buildShareCardData(demoBootstrap, cardWith({ dueDate: "2000-01-01", listKey: "closed" }), new Map()).overdue).toBe(false);
		expect(buildShareCardData(demoBootstrap, cardWith({ dueDate: "2999-12-31" }), new Map()).overdue).toBe(false);
		expect(buildShareCardData(demoBootstrap, cardWith({ dueDate: null }), new Map()).overdue).toBe(false);
	});
});

describe("shareCardFilename", () => {
	it("names the file by iid, or new before the first sync", () => {
		expect(shareCardFilename(127)).toBe("sitcon-board-127.png");
		expect(shareCardFilename(0)).toBe("sitcon-board-new.png");
	});
});

// Ten units per character keeps expectations easy to read: a maxWidth of 100
// holds exactly ten characters.
const measure = (text: string) => Array.from(text).length * 10;

describe("wrapEllipsis", () => {
	it("returns a short text as a single line", () => {
		expect(wrapEllipsis(measure, "報名系統", 100, 3)).toEqual(["報名系統"]);
	});

	it("wraps CJK text per character without word boundaries", () => {
		expect(wrapEllipsis(measure, "一二三四五六七八九十甲乙丙", 100, 3)).toEqual(["一二三四五六七八九十", "甲乙丙"]);
	});

	it("ellipsizes the last permitted line when text remains", () => {
		const lines = wrapEllipsis(measure, "字".repeat(35), 100, 3);
		expect(lines).toHaveLength(3);
		expect(lines[2]).toBe("字".repeat(9) + "…");
	});

	it("keeps the ellipsis inside maxWidth on a single-line clamp", () => {
		expect(wrapEllipsis(measure, "一二三四五六七八九十甲乙", 100, 1)).toEqual(["一二三四五六七八九…"]);
	});
});

describe("layoutChipRows", () => {
	const chip = (name: string): ShareCardLabel => ({ name, color: null });
	const chipWidth = (item: ShareCardLabel) => measure(item.name);
	const overflowChipWidth = (count: number) => measure(`+${count}`);

	it("packs everything into one row when it fits", () => {
		const layout = layoutChipRows(chipWidth, overflowChipWidth, [chip("交通"), chip("餐飲")], 100, 2, 10);
		expect(layout.rows).toEqual([[chip("交通"), chip("餐飲")]]);
		expect(layout.overflow).toBe(0);
	});

	it("wraps onto further rows", () => {
		const layout = layoutChipRows(chipWidth, overflowChipWidth, [chip("網站前端"), chip("網站後端"), chip("設計")], 100, 2, 10);
		expect(layout.rows).toEqual([[chip("網站前端"), chip("網站後端")], [chip("設計")]]);
		expect(layout.overflow).toBe(0);
	});

	it("evicts trailing chips until the +N chip fits on the last row", () => {
		const labels = [chip("甲乙丙丁"), chip("戊己庚辛"), chip("壬癸子丑"), chip("寅卯辰巳"), chip("午未申酉")];
		const layout = layoutChipRows(chipWidth, overflowChipWidth, labels, 100, 2, 10);
		expect(layout.rows).toEqual([[chip("甲乙丙丁"), chip("戊己庚辛")], [chip("壬癸子丑")]]);
		expect(layout.overflow).toBe(2);
	});
});
