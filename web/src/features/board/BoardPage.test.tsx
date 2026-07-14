import { demoBootstrap } from "@/test/demoBootstrap";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCard, moveCard, retryOperation, updateAssignee, updateDueDate, updateTeam } from "./boardApi";
import { BoardPage } from "./BoardPage";
import type { Bootstrap } from "./model";

vi.mock("./boardApi", () => ({
	createCard: vi.fn(),
	logout: vi.fn(),
	moveCard: vi.fn(),
	retryOperation: vi.fn(),
	savePreferences: vi.fn(),
	updateAssignee: vi.fn(),
	updateDueDate: vi.fn(),
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
		vi.mocked(updateDueDate).mockReset();
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
		expect(screen.getByText("同步中")).toBeVisible();
		expect(createCard).toHaveBeenCalledOnce();
	});

	it("moves a card immediately through its keyboard-accessible status control", async () => {
		const user = userEvent.setup();
		vi.mocked(moveCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";

		await user.selectOptions(screen.getByLabelText(`${title}的狀態`), "doing");

		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(doingLane).not.toBeNull();
		expect(within(doingLane as HTMLElement).getByRole("heading", { name: title })).toBeVisible();
		expect(moveCard).toHaveBeenCalledOnce();
	});
});
