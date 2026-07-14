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

function Harness() {
	const [bootstrap, setBootstrap] = useState<Bootstrap>(() => structuredClone(demoBootstrap));
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

	it("defaults quick create to the primary team and current user", () => {
		render(<Harness />);

		expect(screen.getByLabelText("新卡片組別")).toHaveValue("development");
		expect(screen.getByRole("button", { name: "選擇新卡片 Assignee" })).toHaveTextContent("Yorukot");
		expect((screen.getByLabelText("新卡片期限") as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/);
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

		await user.type(screen.getByLabelText("卡片標題"), "新增值班表");
		await user.click(screen.getByRole("button", { name: "建立卡片" }));

		expect(screen.getByRole("heading", { name: "[開發組] 新增值班表" })).toBeVisible();
		expect(screen.queryByText("同步中")).not.toBeInTheDocument();
		expect(createCard).toHaveBeenCalledOnce();
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
		await user.click(within(dialog).getByRole("button", { name: "Close dialog" }));

		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(doingLane).not.toBeNull();
		expect(within(doingLane as HTMLElement).getByRole("heading", { name: title })).toBeVisible();
		expect(moveCard).toHaveBeenCalledOnce();
	});

	it("edits GitLab Start date and previews Markdown planning", async () => {
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
		await user.click(within(dialog).getByRole("button", { name: "Close dialog" }));
		expect(screen.getByRole("heading", { name: "[開發組] 完成寄信失敗重送" })).toBeVisible();
	});

	it("selects more than one assignee", async () => {
		const user = userEvent.setup();
		vi.mocked(updateAssignee).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";

		await user.click(screen.getByRole("button", { name: `變更 ${title} 的 Assignee` }));
		const dialog = screen.getByRole("dialog", { name: "選擇 Assignee" });
		expect(within(dialog).getByRole("checkbox", { name: /Yorukot/ })).toBeChecked();
		await user.click(within(dialog).getByRole("checkbox", { name: /沈明軒/ }));

		expect(updateAssignee).toHaveBeenCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), [114, 115]);
		expect(within(dialog).getByText("已選擇 2 人")).toBeVisible();
	});
});
