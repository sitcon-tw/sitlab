import { demoBootstrap } from "@/test/demoBootstrap";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCard, moveCard, retryOperation, updateAssignee, updateDetails, updateDueDate, updateStartDate, updateTeam } from "./boardApi";
import { BoardPage } from "./BoardPage";
import type { Bootstrap } from "./model";

vi.mock("./boardApi", () => ({
	createCard: vi.fn(),
	logout: vi.fn(),
	moveCard: vi.fn(),
	retryOperation: vi.fn(),
	savePreferences: vi.fn(),
	updateAssignee: vi.fn(),
	updateDetails: vi.fn(),
	updateDueDate: vi.fn(),
	updateStartDate: vi.fn(),
	updateTeam: vi.fn()
}));

function Harness({ initial = demoBootstrap }: { initial?: Bootstrap }) {
	const [bootstrap, setBootstrap] = useState<Bootstrap>(() => structuredClone(initial));
	return <BoardPage bootstrap={bootstrap} updateBootstrap={(update) => setBootstrap((current) => update(current))} backgroundOffline={false} />;
}

describe("SITCON Board interactions", () => {
	beforeEach(() => {
		vi.mocked(createCard).mockReset();
		vi.mocked(moveCard).mockReset();
		vi.mocked(retryOperation).mockReset();
		vi.mocked(updateAssignee).mockReset();
		vi.mocked(updateDetails).mockReset();
		vi.mocked(updateDueDate).mockReset();
		vi.mocked(updateStartDate).mockReset();
		vi.mocked(updateTeam).mockReset();
	});

	it("defaults quick create to the primary team and keeps Inbox in more options", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		expect(screen.getByLabelText("新卡片組別")).toHaveValue("development");
		expect(screen.getByRole("button", { name: "選擇新卡片 Assignee" })).toHaveTextContent("Yorukot");
		expect((screen.getByLabelText("新卡片期限") as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(screen.queryByLabelText("新卡片 Status")).not.toBeInTheDocument();
		await user.click(screen.getByRole("button", { name: "更多建卡選項" }));
		const dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		expect(within(dialog).getByLabelText("新卡片 Status")).toHaveValue("inbox");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
	});

	it("discards unapplied more options and restores focus", async () => {
		const user = userEvent.setup();
		render(<Harness />);
		const moreButton = screen.getByRole("button", { name: "更多建卡選項" });

		await user.click(moreButton);
		let dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		await user.selectOptions(within(dialog).getByLabelText("新卡片 Status"), "review");
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "尚未套用");
		await user.keyboard("{Escape}");

		expect(dialog).not.toBeInTheDocument();
		expect(moreButton).toHaveFocus();
		await user.click(moreButton);
		dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		expect(within(dialog).getByLabelText("新卡片 Status")).toHaveValue("inbox");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
		await user.selectOptions(within(dialog).getByLabelText("新卡片 Status"), "doing");
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "按取消");
		await user.click(within(dialog).getByRole("button", { name: "取消" }));
		expect(moreButton).toHaveFocus();

		await user.click(moreButton);
		dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		expect(within(dialog).getByLabelText("新卡片 Status")).toHaveValue("inbox");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
		await user.selectOptions(within(dialog).getByLabelText("新卡片 Status"), "closed");
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "按關閉");
		await user.click(within(dialog).getByRole("button", { name: "Close dialog" }));
		expect(moreButton).toHaveFocus();

		await user.click(moreButton);
		dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		expect(within(dialog).getByLabelText("新卡片 Status")).toHaveValue("inbox");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
	});

	it("clears the default assignee when quick create switches to another team", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.selectOptions(screen.getByLabelText("新卡片組別"), "design");

		expect(screen.getByRole("button", { name: "選擇新卡片 Assignee" })).toHaveTextContent("未指派");
		expect(screen.getByText("已清除不屬於此組別的 Assignee")).toBeVisible();
	});

	it("renders a new card optimistically before the API responds", async () => {
		const user = userEvent.setup();
		vi.mocked(createCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("button", { name: "更多建卡選項" }));
		let dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		await user.selectOptions(within(dialog).getByLabelText("新卡片 Status"), "doing");
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "確認交接與值班時段");
		await user.click(within(dialog).getByRole("button", { name: "套用" }));
		await user.type(screen.getByLabelText("卡片標題"), "新增值班表");
		await user.click(screen.getByRole("button", { name: "建立卡片" }));

		expect(screen.getByRole("heading", { name: "[開發組] 新增值班表" })).toBeVisible();
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(within(doingLane as HTMLElement).getAllByRole("heading", { level: 3 })[0]).toHaveTextContent("[開發組] 新增值班表");
		expect(within(doingLane as HTMLElement).getByText("確認交接與值班時段")).toBeVisible();
		expect(screen.queryByText("同步中")).not.toBeInTheDocument();
		expect(createCard).toHaveBeenCalledWith(expect.objectContaining({ listKey: "doing", description: "確認交接與值班時段" }));

		await user.click(screen.getByRole("button", { name: "更多建卡選項" }));
		dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		expect(within(dialog).getByLabelText("新卡片 Status")).toHaveValue("doing");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
	});

	it("renders the configured board columns in order", () => {
		render(<Harness />);

		const board = screen.getByRole("region", { name: "SITCON 2027 工作看板" });
		expect(
			within(board)
				.getAllByRole("heading", { level: 2 })
				.map((heading) => heading.textContent)
		).toEqual(["Wating", "Inbox", "To Do", "Doing", "Review", "Closed"]);
	});

	it("filters the board to one team and clears the filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.selectOptions(screen.getByLabelText("篩選組別"), "design");

		const filters = screen.getByRole("region", { name: "篩選看板" });
		expect(within(filters).getByRole("status")).toHaveTextContent("1 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[設計組] 製作工作人員識別證" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).not.toBeInTheDocument();
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(within(doingLane as HTMLElement).getByText("1")).toBeVisible();

		await user.click(within(filters).getByRole("button", { name: "清除篩選" }));

		expect(screen.getByLabelText("篩選組別")).toHaveValue("");
		expect(within(filters).getByRole("status")).toHaveTextContent("7 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
	});

	it("filters by any selected person and combines people with the team filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("button", { name: "篩選負責人" }));
		const dialog = screen.getByRole("dialog", { name: "篩選負責人" });
		const currentUser = within(dialog).getByRole("checkbox", { name: /Yorukot/ });
		currentUser.focus();
		await user.keyboard("{Enter}");
		await user.click(within(dialog).getByRole("checkbox", { name: /林采欣/ }));
		expect(currentUser).toBeChecked();
		expect(within(dialog).getByText("已選擇 2 人")).toBeVisible();
		await user.click(within(dialog).getByRole("button", { name: "完成" }));

		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(screen.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[設計組] 製作工作人員識別證" })).not.toBeInTheDocument();
		expect(screen.getByRole("status")).toHaveTextContent("2 / 7 張卡片");

		await user.selectOptions(screen.getByLabelText("篩選組別"), "development");

		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[行政組] 整理志工行前通知" })).not.toBeInTheDocument();
		expect(screen.getByRole("status")).toHaveTextContent("1 / 7 張卡片");
	});

	it("selects an entire team in the member filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("button", { name: "篩選負責人" }));
		const dialog = screen.getByRole("dialog", { name: "篩選負責人" });
		const teamSelectAll = within(dialog).getByRole("checkbox", { name: "全選行政組" });
		expect(teamSelectAll).not.toBeChecked();
		await user.click(teamSelectAll);

		expect(teamSelectAll).toBeChecked();
		expect(within(dialog).getByText("已選擇 2 人")).toBeVisible();
		await user.click(within(dialog).getByRole("button", { name: "完成" }));
		expect(screen.getByRole("status")).toHaveTextContent("1 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
	});

	it("keeps a member selection synchronized across every team they belong to", async () => {
		const initial = structuredClone(demoBootstrap);
		const currentUser = initial.members.find((member) => member.gitLabUserId === initial.me.gitLabUserId);
		if (!currentUser) throw new Error("demo current user is missing");
		currentUser.teamKeys.push("design");
		const user = userEvent.setup();
		render(<Harness initial={initial} />);

		await user.click(screen.getByRole("button", { name: "篩選負責人" }));
		const dialog = screen.getByRole("dialog", { name: "篩選負責人" });
		const development = within(dialog).getByRole("region", { name: "開發組" });
		const design = within(dialog).getByRole("region", { name: "設計組" });
		await user.click(within(development).getByRole("checkbox", { name: /Yorukot/ }));

		expect(within(design).getByRole("checkbox", { name: /Yorukot/ })).toBeChecked();
		expect(within(design).getByRole("checkbox", { name: "全選設計組" })).toBePartiallyChecked();
	});

	it("sorts cards inside each lane and preserves sorting when filters are cleared", async () => {
		const user = userEvent.setup();
		render(<Harness />);
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		if (!doingLane) throw new Error("Doing lane is missing");
		const titles = () =>
			within(doingLane)
				.getAllByRole("heading", { level: 3 })
				.map((heading) => heading.textContent);

		expect(titles()).toEqual(["[設計組] 製作工作人員識別證", "[場務組] 盤點會場網路設備"]);
		await user.selectOptions(screen.getByLabelText("排序方式"), "due-desc");
		expect(titles()).toEqual(["[場務組] 盤點會場網路設備", "[設計組] 製作工作人員識別證"]);

		await user.selectOptions(screen.getByLabelText("篩選組別"), "design");
		await user.click(screen.getByRole("button", { name: "清除篩選" }));
		expect(screen.getByLabelText("排序方式")).toHaveValue("due-desc");
		expect(titles()).toEqual(["[場務組] 盤點會場網路設備", "[設計組] 製作工作人員識別證"]);
	});

	it("keeps team and status controls in card details and moves optimistically", async () => {
		const user = userEvent.setup();
		vi.mocked(moveCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";

		expect(screen.queryByLabelText(`${title}的狀態`)).not.toBeInTheDocument();
		await user.click(screen.getByRole("heading", { name: title }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(within(dialog).getByLabelText("組別")).toHaveValue("development");
		await user.selectOptions(within(dialog).getByLabelText("狀態"), "doing");
		await user.click(within(dialog).getByRole("button", { name: "Close drawer" }));

		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(doingLane).not.toBeNull();
		expect(within(doingLane as HTMLElement).getByRole("heading", { name: title })).toBeVisible();
		expect(moveCard).toHaveBeenCalledOnce();
	});

	it("edits GitLab Start date, previews Markdown, and keeps the detail drawer open while saving", async () => {
		const user = userEvent.setup();
		vi.mocked(updateDetails).mockReturnValue(new Promise(() => undefined));
		vi.mocked(updateStartDate).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-07-17");
		expect(within(dialog).getByLabelText("Due")).toHaveValue("2026-07-21");
		await user.clear(within(dialog).getByLabelText("Start"));
		await user.type(within(dialog).getByLabelText("Start"), "2026-07-18");
		expect(updateStartDate).toHaveBeenCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), "2026-07-18");

		await user.clear(within(dialog).getByLabelText("標題"));
		await user.type(within(dialog).getByLabelText("標題"), "完成寄信失敗重送");
		fireEvent.change(within(dialog).getByLabelText("描述"), { target: { value: "## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)" } });
		await user.click(within(dialog).getByRole("button", { name: "預覽" }));
		expect(within(dialog).getByRole("heading", { name: "驗收條件" })).toBeVisible();
		expect(within(dialog).getByRole("link", { name: "規格" })).toHaveAttribute("href", "https://example.com/spec");
		await user.click(within(dialog).getByRole("button", { name: "儲存細節" }));

		expect(updateDetails).toHaveBeenCalledWith(
			expect.objectContaining({ issueIid: 127 }),
			expect.any(String),
			"完成寄信失敗重送",
			"## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)"
		);
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
		expect(within(dialog).getByLabelText("標題")).toHaveValue("完成寄信失敗重送");
	});

	it("switches cards inside the right detail drawer", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const first = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await user.clear(within(first).getByLabelText("標題"));
		await user.type(within(first).getByLabelText("標題"), "尚未儲存的標題");
		await user.type(within(first).getByLabelText("Quick action"), "/due");
		await user.click(within(first).getByRole("button", { name: "下一張卡片" }));

		expect(screen.getByRole("dialog", { name: /130 卡片詳細資料/ })).toBeVisible();
		expect(screen.getByLabelText("標題")).toHaveValue("製作工作人員識別證");
		expect(screen.getByLabelText("Quick action")).toHaveValue("");
	});

	it("creates the same top-position card for every configured team leader", async () => {
		const user = userEvent.setup();
		vi.mocked(createCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("button", { name: "所有組長" }));
		await user.click(screen.getByRole("button", { name: "更多建卡選項" }));
		const dialog = screen.getByRole("dialog", { name: "更多建卡選項" });
		await user.selectOptions(within(dialog).getByLabelText("新卡片 Status"), "review");
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "每週例行回報");
		await user.click(within(dialog).getByRole("button", { name: "套用" }));
		await user.type(screen.getByLabelText("卡片標題"), "回報本週進度");
		await user.click(screen.getByRole("button", { name: "為所有組長建立卡片" }));

		expect(createCard).toHaveBeenCalledTimes(11);
		expect(createCard).toHaveBeenCalledWith(
			expect.objectContaining({
				title: "回報本週進度",
				description: "每週例行回報",
				teamKey: "development",
				listKey: "review",
				assigneeGitLabUserIds: [114]
			})
		);
	});

	it("executes supported slash commands through typed card mutations", async () => {
		const user = userEvent.setup();
		vi.mocked(updateDueDate).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await user.type(within(dialog).getByLabelText("Quick action"), "/due 2026-07-31");
		await user.click(within(dialog).getByRole("button", { name: "執行" }));

		expect(updateDueDate).toHaveBeenCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), "2026-07-31");
	});

	it("chooses and executes a quick action from the keyboard", async () => {
		const user = userEvent.setup();
		vi.mocked(moveCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		const input = within(dialog).getByLabelText("Quick action");
		await user.type(input, "/cl{Enter}");
		expect(input).toHaveValue("/close");
		await user.keyboard("{Enter}");

		expect(moveCard).toHaveBeenCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), "closed", expect.any(Number));
	});

	it("selects more than one assignee", async () => {
		const user = userEvent.setup();
		vi.mocked(updateAssignee).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";

		await user.click(screen.getByRole("button", { name: `變更 ${title} 的 Assignee` }));
		const dialog = screen.getByRole("dialog", { name: "選擇 Assignee" });
		expect(within(dialog).getByRole("checkbox", { name: /Yorukot/ })).toBeChecked();
		expect(within(dialog).getByRole("checkbox", { name: "全選開發組" })).toBePartiallyChecked();
		await user.type(within(dialog).getByRole("searchbox", { name: "搜尋成員" }), "沈");
		await user.click(within(dialog).getByRole("checkbox", { name: "全選開發組" }));

		expect(updateAssignee).toHaveBeenCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), [114, 115]);
		expect(within(dialog).getByText("已選擇 2 人")).toBeVisible();
	});
});
