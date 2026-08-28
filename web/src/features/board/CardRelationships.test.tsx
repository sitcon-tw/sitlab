import { demoBootstrap } from "@/test/demoBootstrap";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CardRelationships } from "./CardRelationships";
import type { LinkedWorkItem, WorkItemSummary } from "./model";
import {
	attachChildItem,
	createChildItem,
	createLinkedItems,
	deleteLinkedItem,
	detachChildItem,
	listChildItems,
	listLinkedItems,
	searchRelationshipCandidates
} from "./relationApi";

vi.mock("./relationApi", () => ({
	relationshipKeys: {
		children: (issueIid: number) => ["card-relations", issueIid, "children"],
		linked: (issueIid: number) => ["card-relations", issueIid, "linked"],
		candidates: (issueIid: number, kind: string, query: string) => ["card-relations", issueIid, "candidates", kind, query]
	},
	attachChildItem: vi.fn(),
	createChildItem: vi.fn(),
	createLinkedItems: vi.fn(),
	deleteLinkedItem: vi.fn(),
	detachChildItem: vi.fn(),
	listChildItems: vi.fn(),
	listLinkedItems: vi.fn(),
	searchRelationshipCandidates: vi.fn()
}));

const task = workItem(9201, 201, "task", "Add retry metrics");
const localIssue = workItem(9128, 128, "issue", "整理志工行前通知");
const externalIssue = workItem(9199, 199, "issue", "Not on the board");

function workItem(id: number, iid: number, type: "issue" | "task", title: string): WorkItemSummary {
	return {
		gitLabWorkItemId: id,
		iid,
		type,
		title,
		state: "open",
		webUrl: `https://gitlab.example/work_items/${iid}`,
		status: { name: "To do", category: "to_do", color: null },
		assignees: [
			{
				gitLabUserId: 114,
				username: "yorukot",
				displayName: "Yorukot",
				avatarUrl: null,
				profileUrl: "https://gitlab.example/yorukot"
			}
		],
		startDate: "2026-08-28",
		dueDate: "2026-09-04",
		labels: [{ name: "Backend", color: "#1D76DB", textColor: "#FFFFFF" }]
	};
}

function renderRelationships(onOpenBoardCard = vi.fn()) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
	render(
		<QueryClientProvider client={client}>
			<CardRelationships card={demoBootstrap.board.cards[0]!} bootstrap={demoBootstrap} onOpenBoardCard={onOpenBoardCard} />
		</QueryClientProvider>
	);
	return onOpenBoardCard;
}

describe("CardRelationships", () => {
	beforeEach(() => {
		vi.mocked(attachChildItem).mockReset().mockResolvedValue();
		vi.mocked(createChildItem).mockReset().mockResolvedValue(task);
		vi.mocked(createLinkedItems).mockReset().mockResolvedValue();
		vi.mocked(deleteLinkedItem).mockReset().mockResolvedValue();
		vi.mocked(detachChildItem).mockReset().mockResolvedValue();
		vi.mocked(searchRelationshipCandidates).mockReset().mockResolvedValue([task, externalIssue]);
		vi.mocked(listChildItems)
			.mockReset()
			.mockResolvedValue({ items: [task], totalCount: 1, nextCursor: null });
		vi.mocked(listLinkedItems)
			.mockReset()
			.mockResolvedValue({
				items: [
					{ ...localIssue, linkType: "relates_to" },
					{ ...externalIssue, linkType: "blocks" }
				] as LinkedWorkItem[],
				totalCount: 2,
				nextCursor: null
			});
	});

	it("renders fixed metadata and opens board issues locally", async () => {
		const user = userEvent.setup();
		const onOpenBoardCard = renderRelationships();

		expect(await screen.findByText("Add retry metrics")).toBeVisible();
		expect(screen.getAllByText("To do").length).toBeGreaterThan(0);
		expect(screen.getAllByText("Backend").length).toBeGreaterThan(0);
		expect(screen.queryByText("Is blocked by")).not.toBeInTheDocument();
		expect(screen.getByRole("link", { name: /Not on the board/ })).toHaveAttribute("target", "_blank");

		await user.click(screen.getByRole("button", { name: /整理志工行前通知/ }));
		expect(onOpenBoardCard).toHaveBeenCalledWith(128);
	});

	it("creates a child task and keeps the title when GitLab rejects it", async () => {
		const user = userEvent.setup();
		vi.mocked(createChildItem).mockRejectedValueOnce(new Error("GitLab rejected this relationship"));
		renderRelationships();
		await screen.findByText("Add retry metrics");

		await user.click(screen.getByRole("button", { name: "新增 Child item" }));
		await user.click(await screen.findByRole("menuitem", { name: "建立新 Task" }));
		const dialog = await screen.findByRole("dialog", { name: "建立 Child Task" });
		const input = within(dialog).getByLabelText("Task 標題");
		await user.type(input, "Instrument retry queue");
		await user.click(within(dialog).getByRole("button", { name: "建立 Task" }));

		expect(await within(dialog).findByRole("alert")).toHaveTextContent("GitLab rejected this relationship");
		expect(input).toHaveValue("Instrument retry queue");
	});

	it("selects multiple candidates and preserves linked-item input on tier errors", async () => {
		const user = userEvent.setup();
		vi.mocked(createLinkedItems).mockRejectedValueOnce(new Error("this GitLab tier does not support blocking relationships"));
		renderRelationships();
		await screen.findByText("Add retry metrics");

		await user.click(screen.getByRole("button", { name: "新增 Linked item" }));
		const dialog = await screen.findByRole("dialog", { name: "新增 Linked item" });
		await user.click(within(dialog).getByRole("button", { name: "關聯類型" }));
		await user.click(await screen.findByRole("menuitemcheckbox", { name: "Blocks" }));
		const search = within(dialog).getByLabelText("搜尋 Issue、Task 或 #IID");
		await user.type(search, "retry");
		await waitFor(() => expect(searchRelationshipCandidates).toHaveBeenCalledWith(127, "linked", "retry"));
		await user.click(await within(dialog).findByRole("checkbox", { name: /Add retry metrics/ }));
		await user.click(within(dialog).getByRole("checkbox", { name: /Not on the board/ }));
		expect(within(dialog).getByRole("status")).toHaveTextContent("已選擇 2 個");
		await user.click(within(dialog).getByRole("button", { name: "新增 2 個關聯" }));

		expect(await within(dialog).findByRole("alert")).toHaveTextContent("does not support blocking relationships");
		expect(search).toHaveValue("retry");
		expect(createLinkedItems).toHaveBeenCalledWith(127, [9201, 9199], "blocks");
	});

	it("detaches a child without deleting the work item", async () => {
		const user = userEvent.setup();
		let finishDetach: (() => void) | undefined;
		vi.mocked(detachChildItem).mockImplementationOnce(
			() =>
				new Promise<void>((resolve) => {
					finishDetach = resolve;
				})
		);
		renderRelationships();
		await screen.findByText("Add retry metrics");

		await user.click(screen.getByRole("button", { name: "解除 Child item #201" }));
		const dialog = await screen.findByRole("alertdialog", { name: "解除 Child item？" });
		expect(dialog).toHaveTextContent("不會刪除 Task");
		await user.click(within(dialog).getByRole("button", { name: "確認移除" }));
		expect(screen.queryByText("Add retry metrics")).not.toBeInTheDocument();
		expect(dialog).not.toBeInTheDocument();
		await waitFor(() => expect(detachChildItem).toHaveBeenCalledWith(127, 9201));
		finishDetach?.();
	});

	it("rolls an optimistic removal back when GitLab rejects it", async () => {
		const user = userEvent.setup();
		vi.mocked(deleteLinkedItem).mockRejectedValueOnce(new Error("GitLab unavailable"));
		renderRelationships();
		await screen.findByText("Not on the board");

		await user.click(screen.getByRole("button", { name: "移除 Linked item #199" }));
		await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "確認移除" }));

		expect(await screen.findByText("Not on the board")).toBeVisible();
		expect(await screen.findByRole("alert")).toHaveTextContent("GitLab unavailable");
	});

	it("explains why an issue cannot be a child and carries the search into linked items", async () => {
		const user = userEvent.setup();
		vi.mocked(searchRelationshipCandidates).mockResolvedValue([]);
		renderRelationships();
		await screen.findByText("Add retry metrics");

		await user.click(screen.getByRole("button", { name: "新增 Child item" }));
		await user.click(await screen.findByRole("menuitem", { name: "加入既有 Task" }));
		const childDialog = await screen.findByRole("dialog", { name: "加入既有 Task" });
		await user.type(within(childDialog).getByLabelText("搜尋 Task 標題或 #IID"), "#128");

		expect(await within(childDialog).findByText(/#128 是 Issue，不能作為 Issue 的 Child item/)).toBeVisible();
		await user.click(within(childDialog).getByRole("button", { name: "改用 Linked item" }));

		const linkedDialog = await screen.findByRole("dialog", { name: "新增 Linked item" });
		expect(within(linkedDialog).getByLabelText("搜尋 Issue、Task 或 #IID")).toHaveValue("#128");
		await waitFor(() => expect(searchRelationshipCandidates).toHaveBeenCalledWith(127, "linked", "#128"));
	});
});
