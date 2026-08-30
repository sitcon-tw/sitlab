import { demoBootstrap } from "@/test/demoBootstrap";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	createCard,
	createComment,
	listComments,
	listQuickActions,
	listQuickActionSuggestions,
	moveCard,
	retryOperation,
	updateAssignee,
	updateDetails,
	updateDueDate,
	updateLabels,
	updateStartDate,
	updateTeam
} from "./boardApi";
import { BoardPage } from "./BoardPage";
import { listProjectLabels } from "./labelsApi";
import type { BoardCard, Bootstrap, CardMutation } from "./model";
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
import { renderShareCardImage } from "./shareCardImage";

vi.mock("./labelsApi", () => ({
	listProjectLabels: vi.fn(),
	createProjectLabel: vi.fn(),
	updateProjectLabel: vi.fn(),
	deleteProjectLabel: vi.fn()
}));

vi.mock("./boardApi", () => ({
	createCard: vi.fn(),
	createComment: vi.fn(),
	listComments: vi.fn(),
	listQuickActions: vi.fn(),
	listQuickActionSuggestions: vi.fn(),
	logout: vi.fn(),
	moveCard: vi.fn(),
	retryOperation: vi.fn(),
	savePreferences: vi.fn(),
	updateAssignee: vi.fn(),
	updateDetails: vi.fn(),
	updateDueDate: vi.fn(),
	updateLabels: vi.fn(),
	updateStartDate: vi.fn(),
	updateTeam: vi.fn()
}));

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

// The canvas renderer needs a real 2D context, which jsdom stubs to null; its
// layout logic is covered by shareCard.test.ts against pure helpers.
vi.mock("./shareCardImage", () => ({
	renderShareCardImage: vi.fn()
}));

function Harness({ initial = demoBootstrap }: { initial?: Bootstrap }) {
	const [bootstrap, setBootstrap] = useState<Bootstrap>(() => structuredClone(initial));
	const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }));
	return (
		<QueryClientProvider client={client}>
			<BoardPage bootstrap={bootstrap} updateBootstrap={(update) => setBootstrap((current) => update(current))} backgroundOffline={false} />
		</QueryClientProvider>
	);
}

async function openSearchableFilter(user: ReturnType<typeof userEvent.setup>, label: string) {
	const input = screen.getByRole("combobox", { name: label });
	await user.click(input);
	const panelId = input.getAttribute("aria-controls");
	const panel = panelId ? document.getElementById(panelId) : null;
	if (!panel) throw new Error(`${label} picker did not open`);
	return { input, picker: within(panel) };
}

async function chooseTeamFilter(user: ReturnType<typeof userEvent.setup>, name: string) {
	const { input, picker } = await openSearchableFilter(user, "搜尋組別");
	await user.type(input, name);
	await user.click(picker.getByText(name, { exact: true }));
}

async function chooseSort(user: ReturnType<typeof userEvent.setup>, name: string) {
	await user.click(screen.getByRole("button", { name: "排序方式" }));
	await user.click(await screen.findByRole("menuitemcheckbox", { name }));
}

async function openQuickCreate(user: ReturnType<typeof userEvent.setup>) {
	await user.click(screen.getByRole("button", { name: "新增卡片" }));
	await screen.findByRole("dialog", { name: "新增卡片" });
}

async function chooseSelectField(user: ReturnType<typeof userEvent.setup>, label: string, option: string, root: HTMLElement = document.body) {
	await user.click(within(root).getByRole("button", { name: label }));
	const menu = await screen.findByRole("menu", { name: `${label}選項` });
	await user.click(within(menu).getByRole("menuitemcheckbox", { name: option }));
}

function mutationResult(card: BoardCard, operationId: string, patch: Partial<BoardCard>): CardMutation {
	return {
		card: { ...card, ...patch, syncState: "synced", syncError: null, pendingOperationId: null },
		operation: {
			id: operationId,
			kind: "update_details",
			state: "synced",
			attempts: 1,
			lastError: null,
			createdAt: "2026-08-22T08:00:00Z",
			updatedAt: "2026-08-22T08:00:01Z"
		}
	};
}

describe("SITCON Board interactions", () => {
	beforeEach(() => {
		window.history.replaceState(null, "", "/");
		vi.mocked(createCard).mockReset();
		vi.mocked(createComment).mockReset();
		vi.mocked(listComments).mockReset();
		vi.mocked(listQuickActions).mockReset();
		vi.mocked(listQuickActionSuggestions).mockReset();
		vi.mocked(listProjectLabels).mockReset();
		vi.mocked(moveCard).mockReset();
		vi.mocked(retryOperation).mockReset();
		vi.mocked(updateAssignee).mockReset();
		vi.mocked(updateDetails).mockReset();
		vi.mocked(updateDueDate).mockReset();
		vi.mocked(updateLabels).mockReset();
		vi.mocked(updateStartDate).mockReset();
		vi.mocked(updateTeam).mockReset();
		vi.mocked(attachChildItem).mockReset();
		vi.mocked(createChildItem).mockReset();
		vi.mocked(createLinkedItems).mockReset();
		vi.mocked(deleteLinkedItem).mockReset();
		vi.mocked(detachChildItem).mockReset();
		vi.mocked(listChildItems).mockReset();
		vi.mocked(listLinkedItems).mockReset();
		vi.mocked(searchRelationshipCandidates).mockReset();
		vi.mocked(listProjectLabels).mockResolvedValue([
			{ id: 1, name: "Team::開發組", color: "#0E8A16", textColor: "#FFFFFF", description: "開發組" },
			{ id: 2, name: "Team::設計組", color: "#B60205", textColor: "#FFFFFF", description: "設計組" },
			{ id: 3, name: "Priority::High", color: "#D73A4A", textColor: "#FFFFFF", description: null },
			{ id: 4, name: "Backend", color: "#1D76DB", textColor: "#FFFFFF", description: null }
		]);
		vi.mocked(listComments).mockResolvedValue([]);
		vi.mocked(listQuickActions).mockResolvedValue([
			{ name: "assign", aliases: [], params: ["@user"], description: "Assign users", warning: null, icon: null },
			{ name: "close", aliases: [], params: [], description: "Close this work item", warning: null, icon: null },
			{ name: "label", aliases: [], params: ["~label"], description: "Add labels", warning: null, icon: null }
		]);
		vi.mocked(listQuickActionSuggestions).mockResolvedValue([]);
		vi.mocked(listChildItems).mockResolvedValue({ items: [], totalCount: 0, nextCursor: null });
		vi.mocked(listLinkedItems).mockResolvedValue({ items: [], totalCount: 0, nextCursor: null });
		vi.mocked(searchRelationshipCandidates).mockResolvedValue([]);
		vi.mocked(attachChildItem).mockResolvedValue();
		vi.mocked(createLinkedItems).mockResolvedValue();
		vi.mocked(deleteLinkedItem).mockResolvedValue();
		vi.mocked(detachChildItem).mockResolvedValue();
	});

	it("defaults quick create to the primary team and shows every field inline", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		// Creation is closed by default and lives behind the toolbar button.
		expect(screen.queryByLabelText("卡片標題")).not.toBeInTheDocument();
		await openQuickCreate(user);

		expect(screen.getByRole("button", { name: "新卡片組別" })).toHaveTextContent("開發組");
		expect(screen.getByRole("button", { name: "選擇新卡片 Assignee" })).toHaveTextContent("Yorukot");
		expect((screen.getByLabelText("新卡片期限") as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		const dialog = screen.getByRole("dialog", { name: "新增卡片" });
		expect(within(dialog).getByRole("button", { name: "新卡片 Status" })).toHaveTextContent("Inbox");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
		expect(within(dialog).queryByRole("button", { name: "更多建卡選項" })).not.toBeInTheDocument();
	});

	it("clears the default assignee when quick create switches to another team", async () => {
		const user = userEvent.setup();
		render(<Harness />);
		await openQuickCreate(user);

		await chooseSelectField(user, "新卡片組別", "設計組");

		expect(screen.getByRole("button", { name: "選擇新卡片 Assignee" })).toHaveTextContent("未指派");
		expect(screen.getByText("已清除不屬於此組別的 Assignee")).toBeVisible();
	});

	it("renders a new card optimistically before the API responds", async () => {
		const user = userEvent.setup();
		vi.mocked(createCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		await openQuickCreate(user);

		const dialog = screen.getByRole("dialog", { name: "新增卡片" });
		await chooseSelectField(user, "新卡片 Status", "Doing", dialog);
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "確認交接與值班時段");
		await user.type(within(dialog).getByLabelText("搜尋新卡片 Label"), "Backend");
		await user.click(await within(dialog).findByRole("checkbox", { name: "Backend" }));
		await user.type(screen.getByLabelText("卡片標題"), "新增值班表");
		await user.click(screen.getByRole("button", { name: "建立卡片" }));

		// The create dialog stays open for the next card: the chosen Status
		// sticks while description and labels reset.
		expect(within(dialog).getByRole("button", { name: "新卡片 Status" })).toHaveTextContent("Doing");
		expect(within(dialog).getByLabelText("新卡片 Description")).toHaveValue("");
		expect(within(dialog).getByRole("checkbox", { name: "Backend" })).not.toBeChecked();
		// The modal create dialog hides the board from the accessibility tree.
		await user.keyboard("{Escape}");

		expect(screen.getByRole("heading", { name: "[開發組] 新增值班表" })).toBeVisible();
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(within(doingLane as HTMLElement).getByRole("heading", { name: "[開發組] 新增值班表" })).toBeVisible();
		expect(within(doingLane as HTMLElement).getByText("確認交接與值班時段")).toBeVisible();
		expect(screen.queryByText("同步中")).not.toBeInTheDocument();
		expect(createCard).toHaveBeenCalledWith(expect.objectContaining({ listKey: "doing", description: "確認交接與值班時段", labels: ["Backend"] }));
		await user.click(screen.getByRole("heading", { name: "[開發組] 新增值班表" }));
		expect(screen.getByRole("dialog", { name: "新卡片詳細資料" })).toHaveTextContent("Backend");
		await user.click(screen.getByRole("button", { name: "Close drawer" }));
	});

	it("renders the configured board columns in order", () => {
		render(<Harness />);

		const board = screen.getByRole("region", { name: "SITCON 2027 工作看板" });
		expect(
			within(board)
				.getAllByRole("heading", { level: 2 })
				.map((heading) => heading.textContent)
		).toEqual(["Waiting", "Inbox", "To do", "Doing", "Review", "Done"]);
	});

	it("filters the board to one team and clears the filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await chooseTeamFilter(user, "設計組");

		const filters = screen.getByRole("region", { name: "篩選看板" });
		expect(within(filters).getByRole("status")).toHaveTextContent("1 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[設計組] 製作工作人員識別證" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).not.toBeInTheDocument();
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(within(doingLane as HTMLElement).getByText("1")).toBeVisible();

		await user.click(within(filters).getByRole("button", { name: "清除篩選" }));

		expect(screen.getByRole("combobox", { name: "搜尋組別" })).toHaveValue("所有組別");
		expect(within(filters).getByRole("status")).toHaveTextContent("7 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
	});

	it("searches complete card information after typing settles and restores the query from the URL", async () => {
		window.history.replaceState(null, "", "/?q=Backend");
		const user = userEvent.setup();
		render(<Harness />);

		const search = screen.getByRole("combobox", { name: "搜尋卡片" });
		expect(search).toHaveValue("Backend");
		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("1 / 7 張卡片");

		await user.clear(search);
		await user.type(search, "開發 Yorukot Priority");
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("7 / 7 張卡片");
		await waitFor(() => expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("1 / 7 張卡片"));
		expect(new URLSearchParams(window.location.search).get("q")).toBe("開發 Yorukot Priority");

		await user.click(screen.getByRole("button", { name: "清除卡片搜尋" }));
		await waitFor(() => expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("7 / 7 張卡片"));
	});

	it("focuses card search with slash and clears it with Escape", async () => {
		const user = userEvent.setup();
		render(<Harness />);
		const search = screen.getByRole("combobox", { name: "搜尋卡片" });

		fireEvent.keyDown(window, { key: "/" });
		expect(search).toHaveFocus();
		await user.type(search, "Backend");
		await user.keyboard("{Escape}");
		expect(search).toHaveValue("");
		expect(search).not.toHaveFocus();
	});

	it("restores shared filter and sort query state and keeps later selections in the URL", async () => {
		window.history.replaceState(null, "", "/?team=development&member=114&label=Backend&sort=due-desc&demo=1");
		const user = userEvent.setup();
		render(<Harness />);

		expect(screen.getByRole("combobox", { name: "搜尋組別" })).toHaveValue("開發組");
		expect(screen.getByRole("button", { name: "排序方式" })).toHaveAttribute("data-value", "due-desc");
		expect(screen.getByRole("combobox", { name: "搜尋 Label" })).toHaveValue("Labels 1");
		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[設計組] 製作工作人員識別證" })).not.toBeInTheDocument();

		await chooseSort(user, "Updated time：舊到新");
		expect(window.location.search).toContain("team=development");
		expect(window.location.search).toContain("member=114");
		expect(window.location.search).toContain("label=Backend");
		expect(window.location.search).toContain("sort=updated-asc");
		expect(window.location.search).toContain("demo=1");
	});

	it("inserts filter tokens from the token search suggestions", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const search = screen.getByRole("combobox", { name: "搜尋卡片" });
		await user.click(search);
		await user.click(screen.getByRole("option", { name: /^組別/ }));
		await user.type(search, "設計");
		await user.click(screen.getByRole("option", { name: "設計組" }));

		expect(screen.getByRole("button", { name: "移除組別篩選 設計組" })).toBeVisible();
		expect(screen.getByRole("combobox", { name: "搜尋組別" })).toHaveValue("設計組");
		expect(window.location.search).toContain("team=design");
		expect(window.location.search).not.toContain("q=");
		expect(screen.getByRole("heading", { name: "[設計組] 製作工作人員識別證" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).not.toBeInTheDocument();
	});

	it("removes filter tokens and syncs the pickers", async () => {
		window.history.replaceState(null, "", "/?team=development&member=114&label=Backend");
		const user = userEvent.setup();
		render(<Harness />);

		expect(screen.getByRole("button", { name: "移除組別篩選 開發組" })).toBeVisible();
		expect(screen.getByRole("button", { name: "移除負責人篩選 Yorukot" })).toBeVisible();
		expect(screen.getByRole("button", { name: "移除 Label 篩選 Backend" })).toBeVisible();

		await user.click(screen.getByRole("button", { name: "移除 Label 篩選 Backend" }));
		expect(screen.getByRole("combobox", { name: "搜尋 Label" })).toHaveValue("所有 Labels");
		expect(window.location.search).not.toContain("label=Backend");
		expect(window.location.search).toContain("team=development");
	});

	it("removes the last token with backspace and cancels a pending category instead of a chip", async () => {
		window.history.replaceState(null, "", "/?member=114&label=Backend");
		const user = userEvent.setup();
		render(<Harness />);

		const search = screen.getByRole("combobox", { name: "搜尋卡片" });
		await user.click(search);
		await user.keyboard("{Backspace}");
		expect(screen.queryByRole("button", { name: "移除 Label 篩選 Backend" })).not.toBeInTheDocument();
		expect(screen.getByRole("button", { name: "移除負責人篩選 Yorukot" })).toBeVisible();
		expect(window.location.search).not.toContain("label=");

		await user.click(screen.getByRole("option", { name: /^負責人/ }));
		expect(screen.getByText("負責人：")).toBeVisible();
		await user.keyboard("{Backspace}");
		expect(screen.queryByText("負責人：")).not.toBeInTheDocument();
		expect(screen.getByRole("button", { name: "移除負責人篩選 Yorukot" })).toBeVisible();
	});

	it("keeps already selected values out of the token suggestions", async () => {
		window.history.replaceState(null, "", "/?member=114");
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("combobox", { name: "搜尋卡片" }));
		await user.click(screen.getByRole("option", { name: /^負責人/ }));
		expect(screen.getAllByRole("option").length).toBeGreaterThan(0);
		expect(screen.queryByRole("option", { name: /Yorukot/ })).not.toBeInTheDocument();
	});

	it("walks the token search escape ladder and consumes a chosen category prefix", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const search = screen.getByRole("combobox", { name: "搜尋卡片" });
		await user.click(search);
		expect(screen.getAllByRole("option")).toHaveLength(3);
		await user.keyboard("{Escape}");
		expect(search).toHaveFocus();
		expect(screen.queryAllByRole("option")).toHaveLength(0);

		await user.keyboard("{ArrowDown}");
		expect(screen.getAllByRole("option")).toHaveLength(3);

		await user.type(search, "組");
		expect(screen.getAllByRole("option")).toHaveLength(1);
		await user.keyboard("{Enter}");
		expect(screen.getByText("組別：")).toBeVisible();
		expect(search).toHaveValue("");
		expect(window.location.search).not.toContain("q=");

		await user.keyboard("{Escape}");
		expect(screen.queryByText("組別：")).not.toBeInTheDocument();
		expect(search).toHaveFocus();
	});

	it("searches and combines selected Labels with AND semantics", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const { input: search, picker } = await openSearchableFilter(user, "搜尋 Label");
		expect(search.closest(".md-field")).not.toBeNull();
		await user.type(search, "backend");
		const backend = picker.getByRole("checkbox", { name: "Backend" });
		expect(backend).toHaveClass("md-checkbox");
		await user.click(backend);
		await user.clear(search);
		await user.type(search, "priority");
		await user.click(picker.getByRole("checkbox", { name: "Priority::High" }));

		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("1 / 7 張卡片");
		expect(new URLSearchParams(window.location.search).getAll("label")).toEqual(["Backend", "Priority::High"]);
	});

	it("filters by any selected person and combines people with the team filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const { picker } = await openSearchableFilter(user, "搜尋負責人");
		const currentUser = picker.getByRole("checkbox", { name: /Yorukot/ });
		currentUser.focus();
		await user.keyboard("{Enter}");
		await user.click(picker.getByRole("checkbox", { name: /林采欣/ }));
		expect(currentUser).toBeChecked();

		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(screen.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[設計組] 製作工作人員識別證" })).not.toBeInTheDocument();
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("2 / 7 張卡片");

		await chooseTeamFilter(user, "開發組");

		expect(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		expect(screen.queryByRole("heading", { name: "[行政組] 整理志工行前通知" })).not.toBeInTheDocument();
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("1 / 7 張卡片");
	});

	it("puts the current user's primary team first and the current user first within it", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const { picker } = await openSearchableFilter(user, "搜尋負責人");
		let groups = picker.getAllByRole("region");
		expect(groups[0]).toHaveAccessibleName("開發組");
		expect(within(groups[0] as HTMLElement).getAllByRole("checkbox")[1]).toHaveAccessibleName(/Yorukot/);
		await user.keyboard("{Escape}");

		await openQuickCreate(user);
		await chooseSelectField(user, "新卡片組別", "設計組");
		const assigneeTrigger = screen.getByRole("button", { name: "選擇新卡片 Assignee" });
		expect(assigneeTrigger.closest(".md-chip")).toHaveClass("md-chip--input");
		await user.click(assigneeTrigger);
		const dialog = screen.getByRole("dialog", { name: "選擇 Assignee" });
		groups = within(dialog).getAllByRole("region");
		expect(groups[0]).toHaveAccessibleName("開發組");
		expect(within(groups[0] as HTMLElement).getAllByRole("checkbox")[1]).toHaveAccessibleName(/Yorukot/);
	});

	it("selects an entire team in the member filter", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		const { picker } = await openSearchableFilter(user, "搜尋負責人");
		const teamSelectAll = picker.getByRole("checkbox", { name: "全選行政組" });
		expect(teamSelectAll).not.toBeChecked();
		await user.click(teamSelectAll);

		expect(teamSelectAll).toBeChecked();
		expect(within(screen.getByRole("region", { name: "篩選看板" })).getByRole("status")).toHaveTextContent("1 / 7 張卡片");
		expect(screen.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
	});

	it("keeps a member selection synchronized across every team they belong to", async () => {
		const initial = structuredClone(demoBootstrap);
		const currentUser = initial.members.find((member) => member.gitLabUserId === initial.me.gitLabUserId);
		if (!currentUser) throw new Error("demo current user is missing");
		currentUser.teamKeys.push("design");
		const user = userEvent.setup();
		render(<Harness initial={initial} />);

		const { picker } = await openSearchableFilter(user, "搜尋負責人");
		const development = picker.getByRole("region", { name: "開發組" });
		const design = picker.getByRole("region", { name: "設計組" });
		await user.click(within(development).getByRole("checkbox", { name: /Yorukot/ }));

		expect(within(design).getByRole("checkbox", { name: /Yorukot/ })).toBeChecked();
		expect(within(design).getByRole("checkbox", { name: "全選設計組" })).toBePartiallyChecked();
	});

	it("sorts cards inside each lane and preserves sorting when filters are cleared", async () => {
		const user = userEvent.setup();
		render(<Harness />);
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		if (!doingLane) throw new Error("Doing lane is missing");
		// The heading text carries an aria-hidden #iid prefix; strip it so the
		// assertions keep comparing titles alone.
		const titles = () =>
			within(doingLane)
				.getAllByRole("heading", { level: 3 })
				.map((heading) => heading.textContent?.replace(/^#\w+/, ""));

		expect(titles()).toEqual(["[設計組] 製作工作人員識別證", "[場務組] 盤點會場網路設備"]);
		await chooseSort(user, "Due date：遠到近");
		expect(titles()).toEqual(["[場務組] 盤點會場網路設備", "[設計組] 製作工作人員識別證"]);

		await chooseTeamFilter(user, "設計組");
		await user.click(screen.getByRole("button", { name: "清除篩選" }));
		expect(screen.getByRole("button", { name: "排序方式" })).toHaveAttribute("data-value", "due-desc");
		expect(titles()).toEqual(["[場務組] 盤點會場網路設備", "[設計組] 製作工作人員識別證"]);
	});

	it("defaults to due date near-to-far without disabling card dragging", () => {
		render(<Harness />);
		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		if (!doingLane) throw new Error("Doing lane is missing");

		expect(screen.getByRole("button", { name: "排序方式" })).toHaveAttribute("data-value", "due-asc");
		expect(within(doingLane).queryByRole("button", { name: /^(上移|下移) / })).not.toBeInTheDocument();
		const handle = within(doingLane).getByRole("button", { name: "拖曳 [設計組] 製作工作人員識別證" });
		expect(handle).toBeEnabled();
		expect(handle).toHaveAttribute("title", "拖曳變更卡片狀態");
	});

	it("keeps team and status controls in card details and moves optimistically", async () => {
		const user = userEvent.setup();
		vi.mocked(moveCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";

		expect(screen.queryByLabelText(`${title}的狀態`)).not.toBeInTheDocument();
		await user.click(screen.getByRole("heading", { name: title }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(within(dialog).getByRole("button", { name: "組別" })).toHaveTextContent("開發組");
		await chooseSelectField(user, "狀態", "Doing", dialog);
		await user.click(within(dialog).getByRole("button", { name: "Close drawer" }));

		const doingLane = screen.getByRole("heading", { name: "Doing" }).closest("section");
		expect(doingLane).not.toBeNull();
		expect(within(doingLane as HTMLElement).getByRole("heading", { name: title })).toBeVisible();
		expect(moveCard).toHaveBeenCalledOnce();
		expect(vi.mocked(moveCard).mock.calls[0]).toHaveLength(3);
	});

	it("shows an in-place saving indicator that settles to saved without moving focus", async () => {
		const user = userEvent.setup();
		let resolveStart: ((result: CardMutation) => void) | undefined;
		vi.mocked(updateStartDate).mockImplementation(
			(card, operationId) =>
				new Promise<CardMutation>((resolve) => {
					resolveStart = () => resolve(mutationResult(card, operationId, { startDate: "2026-07-18" }));
				})
		);
		render(<Harness />);
		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		const startInput = within(dialog).getByLabelText("Start");
		fireEvent.change(startInput, { target: { value: "2026-07-18" } });

		expect(within(dialog).getByTitle("Start儲存中")).toBeInTheDocument();
		expect(within(dialog).getByRole("status")).toHaveTextContent("Start儲存中");

		resolveStart!({} as CardMutation);
		await waitFor(() => expect(within(dialog).getByTitle("Start已儲存")).toBeInTheDocument());
		// The control is never disabled or re-keyed while saving, so focus survives.
		expect(within(dialog).getByLabelText("Start")).toBe(startInput);
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
	});

	// Regression: the card carries one syncState and only the newest
	// pendingOperationId, so any indicator derived from it would strand the first
	// field on "saving" forever. Save state is per field, keyed by operationId.
	it("settles two fields edited in quick succession independently", async () => {
		const user = userEvent.setup();
		const pending = new Map<string, (result: CardMutation) => void>();
		vi.mocked(updateStartDate).mockImplementation(
			(card, operationId) =>
				new Promise<CardMutation>((resolve) => pending.set("start", () => resolve(mutationResult(card, operationId, { startDate: "2026-07-18" }))))
		);
		vi.mocked(updateDueDate).mockImplementation(
			(card, operationId) =>
				new Promise<CardMutation>((resolve) => pending.set("due", () => resolve(mutationResult(card, operationId, { dueDate: "2026-07-25" }))))
		);
		render(<Harness />);
		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		fireEvent.change(within(dialog).getByLabelText("Start"), { target: { value: "2026-07-18" } });
		fireEvent.change(within(dialog).getByLabelText("Due"), { target: { value: "2026-07-25" } });
		expect(within(dialog).getByTitle("Start儲存中")).toBeInTheDocument();
		expect(within(dialog).getByTitle("Due儲存中")).toBeInTheDocument();

		pending.get("start")!({} as CardMutation);

		await waitFor(() => expect(within(dialog).getByTitle("Start已儲存")).toBeInTheDocument());
		expect(within(dialog).getByTitle("Due儲存中")).toBeInTheDocument();
		expect(within(dialog).queryByTitle("Due已儲存")).not.toBeInTheDocument();
	});

	it("marks the field as unsynced when its save fails and leaves the card alert authoritative", async () => {
		const user = userEvent.setup();
		vi.mocked(updateDueDate).mockRejectedValue(new Error("GitLab 暫時無法更新。"));
		render(<Harness />);
		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		fireEvent.change(within(dialog).getByLabelText("Due"), { target: { value: "2026-07-25" } });
		await waitFor(() => expect(within(dialog).getByTitle("Due未同步")).toBeInTheDocument());
		expect(within(dialog).getByRole("status")).toHaveTextContent("Due儲存失敗");
	});

	it("shows general labels on the card as a single scrollable row and hides the team label", async () => {
		render(<Harness />);
		const card = (await screen.findByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).closest("article") as HTMLElement;
		const strip = await within(card).findByRole("group", { name: "[開發組] 修正報名系統寄信流程 的 Labels" });
		expect(within(strip).getByText("Priority::High")).toBeVisible();
		expect(within(strip).getByText("Backend")).toBeVisible();
		// The title already carries the team prefix, so a Team:: chip would repeat it.
		expect(within(strip).queryByText("Team::開發組")).not.toBeInTheDocument();
		// One tab stop for the row, not one per chip.
		expect(strip).toHaveAttribute("tabindex", "0");
		expect(within(strip).queryAllByRole("button")).toHaveLength(0);
	});

	it("renders no label row on a card that only carries its team label", async () => {
		render(<Harness />);
		const card = (await screen.findByRole("heading", { name: "[行政組] 整理志工行前通知" })).closest("article") as HTMLElement;
		expect(within(card).queryByRole("group", { name: /Labels$/ })).not.toBeInTheDocument();
	});

	it("ignores an older move response after a newer move has started", async () => {
		const user = userEvent.setup();
		const requests: Array<{
			card: BoardCard;
			operationId: string;
			listKey: string;
			position: number | undefined;
			resolve: (result: CardMutation) => void;
		}> = [];
		vi.mocked(moveCard).mockImplementation(
			(card, operationId, listKey, position) => new Promise<CardMutation>((resolve) => requests.push({ card, operationId, listKey, position, resolve }))
		);
		render(<Harness />);
		const title = "[開發組] 修正報名系統寄信流程";
		await user.click(screen.getByRole("heading", { name: title }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await chooseSelectField(user, "狀態", "Doing", dialog);
		await chooseSelectField(user, "狀態", "Review", dialog);
		expect(requests).toHaveLength(2);

		const result = (request: (typeof requests)[number]): CardMutation => ({
			card: {
				...request.card,
				listKey: request.listKey,
				position: request.position ?? 0,
				syncState: "synced",
				syncError: null,
				pendingOperationId: null
			},
			operation: {
				id: request.operationId,
				kind: "move_card",
				state: "synced",
				attempts: 1,
				lastError: null,
				createdAt: "2026-08-22T08:00:00Z",
				updatedAt: "2026-08-22T08:00:01Z"
			}
		});
		requests[1]!.resolve(result(requests[1]!));
		await waitFor(() => expect(within(dialog).getByRole("button", { name: "狀態" })).toHaveTextContent("Review"));
		requests[0]!.resolve(result(requests[0]!));
		await waitFor(() => expect(within(dialog).getByRole("button", { name: "狀態" })).toHaveTextContent("Review"));
		await user.click(within(dialog).getByRole("button", { name: "Close drawer" }));
		const reviewLane = screen.getByRole("heading", { name: "Review" }).closest("section");
		expect(within(reviewLane as HTMLElement).getByRole("heading", { name: title })).toBeVisible();
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
		expect(within(dialog).queryByLabelText("描述")).not.toBeInTheDocument();
		expect(within(dialog).getByText("釐清失敗重送條件，補上整合測試與觀測紀錄。")).toBeVisible();
		await user.click(within(dialog).getByLabelText("描述預覽"));
		expect(within(dialog).getByLabelText("描述")).toHaveFocus();
		fireEvent.change(within(dialog).getByLabelText("描述"), { target: { value: "## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)" } });
		await user.click(within(dialog).getByRole("button", { name: "預覽" }));
		expect(within(dialog).getByRole("heading", { name: "驗收條件" })).toBeVisible();
		expect(within(dialog).getByRole("link", { name: "規格" })).toHaveAttribute("href", "https://example.com/spec");
		await user.click(within(dialog).getByRole("button", { name: "儲存" }));

		expect(updateDetails).toHaveBeenCalledWith(
			expect.objectContaining({ issueIid: 127 }),
			expect.any(String),
			"完成寄信失敗重送",
			"## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)"
		);
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
		expect(within(dialog).getByLabelText("標題")).toHaveValue("完成寄信失敗重送");
		// The optimistic patch already matches the fields, so the unsaved bar
		// closes even while the request is still in flight.
		expect(within(dialog).queryByText("小心，你還沒儲存")).not.toBeInTheDocument();
	});

	it("shows the unsaved bar for detail edits and reverts them", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(within(dialog).queryByText("小心，你還沒儲存")).not.toBeInTheDocument();
		expect(within(dialog).queryByRole("button", { name: "還原" })).not.toBeInTheDocument();

		await user.clear(within(dialog).getByLabelText("標題"));
		expect(within(dialog).getByText("小心，你還沒儲存")).toBeVisible();
		expect(within(dialog).getByRole("button", { name: "儲存" })).toBeDisabled();

		await user.type(within(dialog).getByLabelText("標題"), "尚未儲存的標題");
		expect(within(dialog).getByRole("button", { name: "儲存" })).toBeEnabled();

		await user.click(within(dialog).getByRole("button", { name: "還原" }));
		expect(within(dialog).getByLabelText("標題")).toHaveValue("修正報名系統寄信流程");
		expect(within(dialog).getByLabelText("標題")).toHaveFocus();
		expect(within(dialog).queryByText("小心，你還沒儲存")).not.toBeInTheDocument();
		expect(updateDetails).not.toHaveBeenCalled();
	});

	it("opens description in edit mode when empty and in preview when it has content", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[議程組] 確認議程講者資料" }));
		const emptyDialog = screen.getByRole("dialog", { name: /129 卡片詳細資料/ });
		expect(within(emptyDialog).getByLabelText("描述")).toBeVisible();
		await user.click(within(emptyDialog).getByRole("button", { name: "Close drawer" }));

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(within(dialog).queryByLabelText("描述")).not.toBeInTheDocument();
		expect(within(dialog).getByText("釐清失敗重送條件，補上整合測試與觀測紀錄。")).toBeVisible();
	});

	it("keeps status out of Labels and normalizes Team Label changes", async () => {
		const user = userEvent.setup();
		vi.mocked(updateLabels).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		const tags = within(dialog).getByRole("heading", { name: "Labels" }).closest("section");
		expect(tags).not.toBeNull();
		expect(within(tags as HTMLElement).getByText("Team::開發組")).toBeVisible();
		expect(within(tags as HTMLElement).queryByText("Status::To Do")).not.toBeInTheDocument();
		expect(within(tags as HTMLElement).getByText("Priority::High")).toBeVisible();
		expect(within(tags as HTMLElement).getByRole("button", { name: "移除 Label Team::開發組" })).toBeDisabled();

		await user.click(within(tags as HTMLElement).getByRole("button", { name: "移除 Label Backend" }));
		expect(updateLabels).toHaveBeenCalledWith(
			expect.objectContaining({ issueIid: 127 }),
			expect.any(String),
			expect.arrayContaining(["Team::開發組", "Priority::High"])
		);

		await user.click(within(tags as HTMLElement).getByRole("button", { name: "新增 Label" }));
		const labelPicker = await screen.findByRole("dialog", { name: "新增 Label" });
		await user.click(within(labelPicker).getByRole("button", { name: /Team::設計組/ }));
		expect(updateLabels).toHaveBeenLastCalledWith(expect.objectContaining({ issueIid: 127 }), expect.any(String), expect.not.arrayContaining(["Team::開發組"]));
		expect(vi.mocked(updateLabels).mock.calls.at(-1)?.[2]).toContain("Team::設計組");
	});

	it("renders system activity and keeps a failed Comment draft for retry", async () => {
		const user = userEvent.setup();
		vi.mocked(listComments).mockResolvedValue([
			{
				id: 1,
				body: "changed status to **To Do**",
				author: { gitLabUserId: 114, username: "yorukot", displayName: "Yorukot", avatarUrl: null, profileUrl: "https://gitlab.com/yorukot" },
				system: true,
				createdAt: "2026-07-28T08:00:00Z",
				updatedAt: "2026-07-28T08:00:00Z"
			},
			{
				id: 2,
				body: '<p>changed title from <code class="idiff">調查相見歡日期</code> to <code class="idiff"><span class="idiff left right addition">[行政組] </span>調查相見歡日期</code></p>',
				author: { gitLabUserId: 114, username: "yorukot", displayName: "Yorukot", avatarUrl: null, profileUrl: "https://gitlab.com/yorukot" },
				system: true,
				createdAt: "2026-07-28T09:00:00Z",
				updatedAt: "2026-07-28T09:00:00Z"
			}
		]);
		vi.mocked(createComment)
			.mockRejectedValueOnce(new Error("GitLab unavailable"))
			.mockResolvedValueOnce({
				comment: {
					id: 3,
					body: "請協助 review",
					author: { gitLabUserId: 114, username: "yorukot", displayName: "Yorukot", avatarUrl: null, profileUrl: "https://gitlab.com/yorukot" },
					system: false,
					createdAt: "2026-07-29T08:00:00Z",
					updatedAt: "2026-07-29T08:00:00Z"
				},
				quickActionsApplied: false,
				summary: []
			});
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		expect(await within(dialog).findAllByText("系統活動")).toHaveLength(2);
		const comments = within(dialog).getByRole("heading", { name: "Comment" }).closest("section");
		expect(comments).not.toBeNull();
		expect(within(comments as HTMLElement).getByText("To Do")).toBeVisible();
		// GitLab title-change notes ship inline-diff HTML; it must render, not
		// print as escaped markup.
		expect(within(comments as HTMLElement).getByText("[行政組]")).toBeVisible();
		expect(within(comments as HTMLElement).queryByText(/<code/)).toBeNull();

		const composer = within(dialog).getByRole("combobox", { name: "Comment" });
		await user.type(composer, "請協助 review");
		await user.click(within(dialog).getByRole("button", { name: "送出 Comment" }));
		expect(await within(dialog).findByRole("alert")).toHaveTextContent("GitLab unavailable");
		expect(composer).toHaveValue("請協助 review");

		await user.click(within(dialog).getByRole("button", { name: "送出 Comment" }));
		await waitFor(() => expect(composer).toHaveValue(""));
		expect(within(dialog).getByText("請協助 review")).toBeVisible();
	});

	it("strips unsafe HTML from comment bodies", async () => {
		const user = userEvent.setup();
		vi.mocked(listComments).mockResolvedValue([
			{
				id: 1,
				body: '<p>hi <script>window.alert(1)</script><a href="javascript:alert(1)">bad link</a></p>',
				author: { gitLabUserId: 115, username: "ming", displayName: "沈明軒", avatarUrl: null, profileUrl: "https://gitlab.com/ming" },
				system: false,
				createdAt: "2026-07-29T06:30:00Z",
				updatedAt: "2026-07-29T06:30:00Z"
			}
		]);
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		const comments = within(dialog).getByRole("heading", { name: "Comment" }).closest("section") as HTMLElement;
		expect(await within(comments).findByText(/hi/)).toBeVisible();
		expect(comments.querySelector("script")).toBeNull();
		expect(within(comments).queryByRole("link", { name: "bad link" })?.getAttribute("href") ?? "").not.toContain("javascript:");
	});

	it("switches cards inside the right detail drawer", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const first = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await user.clear(within(first).getByLabelText("標題"));
		await user.type(within(first).getByLabelText("標題"), "尚未儲存的標題");
		await user.click(within(first).getByRole("button", { name: "下一張卡片" }));

		// Unsaved edits hold the drawer on this card until saved or reverted.
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
		expect(within(first).getByLabelText("標題")).toHaveValue("尚未儲存的標題");
		// Bar message and the drawer's live region both carry the explanation.
		expect(within(first).getAllByText("請先儲存或還原變更，才能離開這張卡片").length).toBeGreaterThan(0);
		await user.click(within(first).getByRole("button", { name: "還原" }));
		await user.click(within(first).getByRole("button", { name: "下一張卡片" }));

		expect(screen.getByRole("dialog", { name: /130 卡片詳細資料/ })).toBeVisible();
		expect(screen.getByLabelText("標題")).toHaveValue("製作工作人員識別證");
		expect(screen.queryByText("小心，你還沒儲存")).not.toBeInTheDocument();
	});

	it("blocks closing the drawer while edits are unsaved", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
		const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await user.type(within(dialog).getByLabelText("標題"), "！");

		await user.click(within(dialog).getByRole("button", { name: "Close drawer" }));
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
		await user.keyboard("{Escape}");
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
		expect(within(dialog).getAllByText("請先儲存或還原變更，才能離開這張卡片").length).toBeGreaterThan(0);

		await user.click(within(dialog).getByRole("button", { name: "還原" }));
		await user.click(within(dialog).getByRole("button", { name: "Close drawer" }));
		expect(screen.queryByRole("dialog", { name: /127 卡片詳細資料/ })).not.toBeInTheDocument();
	});

	it("creates the same top-position card for every configured team leader", async () => {
		const user = userEvent.setup();
		vi.mocked(createCard).mockReturnValue(new Promise(() => undefined));
		render(<Harness />);
		await openQuickCreate(user);

		await user.click(screen.getByRole("button", { name: "所有組長" }));
		const dialog = screen.getByRole("dialog", { name: "新增卡片" });
		await chooseSelectField(user, "新卡片 Status", "Review", dialog);
		await user.type(within(dialog).getByLabelText("新卡片 Description"), "每週例行回報");
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

	it("navigates GitLab quick actions inside a description", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("heading", { name: "[議程組] 確認議程講者資料" }));
		const dialog = screen.getByRole("dialog", { name: /129 卡片詳細資料/ });
		const input = within(dialog).getByLabelText("描述");
		await user.type(input, "/");
		const suggestions = screen.getByRole("listbox", { name: "GitLab autocomplete" });
		expect(within(suggestions).getByRole("option", { name: /\/assign/ })).toHaveAttribute("aria-selected", "true");

		await user.keyboard("{ArrowDown}");
		expect(within(suggestions).getByRole("option", { name: /\/close/ })).toHaveAttribute("aria-selected", "true");

		await user.keyboard("{Enter}");
		expect(input).toHaveValue("/close");
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

	it("shows the 50 most recently updated closed cards before revealing more", async () => {
		const user = userEvent.setup();
		const initial = structuredClone(demoBootstrap);
		const closedList = initial.board.lists.find((list) => list.closed)!;
		const template = initial.board.cards.find((card) => card.listKey === closedList.key)!;
		const closedCards = Array.from({ length: 55 }, (_, index) => ({
			...template,
			issueIid: 2_000 + index,
			title: `近期完成 ${index + 1}`,
			position: index,
			updatedAt: new Date(Date.UTC(2026, 6, 1, 0, index)).toISOString()
		}));
		initial.board.cards = [...initial.board.cards.filter((card) => card.listKey !== closedList.key), ...closedCards];
		render(<Harness initial={initial} />);

		const doneLane = screen.getByRole("heading", { name: closedList.name }).closest("section")!;
		expect(within(doneLane).getAllByRole("article")).toHaveLength(50);
		expect(within(doneLane).getByRole("heading", { name: /近期完成 55$/ })).toBeVisible();
		expect(within(doneLane).queryByRole("heading", { name: /近期完成 1$/ })).not.toBeInTheDocument();
		expect(within(doneLane).getByText("已顯示最近 50 / 55 個 Issue")).toBeVisible();

		await user.click(within(doneLane).getByRole("button", { name: "在 Done 顯示更多 5 個 Issue" }));
		expect(within(doneLane).getAllByRole("article")).toHaveLength(55);
		expect(within(doneLane).getByRole("heading", { name: /近期完成 1$/ })).toBeVisible();
		expect(within(doneLane).queryByRole("button", { name: /顯示更多/ })).not.toBeInTheDocument();

		await user.type(screen.getByRole("combobox", { name: "搜尋卡片" }), "近期完成");
		await waitFor(() => expect(within(doneLane).getAllByRole("article")).toHaveLength(50));
		expect(within(doneLane).getByText("已顯示最近 50 / 55 個 Issue")).toBeVisible();
	});

	it("switches to a URL-backed Gantt view with only open issues", async () => {
		const user = userEvent.setup();
		render(<Harness />);

		await user.click(screen.getByRole("button", { name: "Gantt" }));
		const gantt = await screen.findByRole("region", { name: "SITCON 2027 甘特圖" });

		expect(window.location.search).toContain("view=gantt");
		expect(within(screen.getByRole("region", { name: "篩選甘特圖" })).getByRole("status")).toHaveTextContent("6 / 6 個開啟 Issue");
		expect(screen.queryByRole("button", { name: "排序方式" })).not.toBeInTheDocument();
		expect(within(gantt).queryByText("完成主視覺社群素材")).not.toBeInTheDocument();
		const scale = within(gantt).getByRole("group", { name: "時間尺度" });
		expect(within(scale).getByRole("button", { name: "日" })).toHaveAttribute("aria-pressed", "true");
		await user.click(within(scale).getByRole("button", { name: "週" }));
		expect(window.location.search).toContain("scale=week");

		await user.click(within(gantt).getByRole("button", { name: /開啟 Issue #127/ }));
		expect(screen.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
	});

	it("keeps undated open issues in the Gantt unscheduled section", async () => {
		const user = userEvent.setup();
		const initial = structuredClone(demoBootstrap);
		const undated = initial.board.cards.find((card) => card.issueIid === 129)!;
		undated.startDate = null;
		undated.dueDate = null;
		render(<Harness initial={initial} />);

		await user.click(screen.getByRole("button", { name: "Gantt" }));
		const unscheduled = await screen.findByRole("region", { name: "未排程" });
		expect(within(unscheduled).getByRole("button", { name: /確認議程講者資料/ })).toBeVisible();
	});

	describe("share image", () => {
		class FakeClipboardItem {
			constructor(readonly items: Record<string, Blob | Promise<Blob>>) {}
		}

		// Awaits the wrapped blob like real browsers, so a failed render rejects
		// the write instead of silently "copying" nothing.
		function stubImageClipboard(write: (items: FakeClipboardItem[]) => Promise<void>) {
			vi.stubGlobal("ClipboardItem", FakeClipboardItem);
			Object.defineProperty(navigator, "clipboard", { configurable: true, value: { write } });
		}

		const consumingWrite = () => vi.fn(async (items: FakeClipboardItem[]) => void (await items[0]!.items["image/png"]));
		const pngBlob = new Blob(["png"], { type: "image/png" });

		async function openShareableCard(user: ReturnType<typeof userEvent.setup>) {
			render(<Harness />);
			await user.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
			return screen.getByRole("dialog", { name: /127 卡片詳細資料/ });
		}

		beforeEach(() => {
			vi.mocked(renderShareCardImage).mockReset();
		});

		afterEach(() => {
			vi.unstubAllGlobals();
			Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
		});

		it("copies the card image and hands the renderer resolved card data", async () => {
			const user = userEvent.setup();
			const write = consumingWrite();
			stubImageClipboard(write);
			vi.mocked(renderShareCardImage).mockResolvedValue(pngBlob);
			const dialog = await openShareableCard(user);

			await user.click(within(dialog).getByRole("button", { name: "分享圖片" }));

			expect(await screen.findByText("已複製卡片圖片")).toBeVisible();
			expect(write).toHaveBeenCalledOnce();
			const data = vi.mocked(renderShareCardImage).mock.calls[0]![0];
			expect(data.title).toBe("[開發組] 修正報名系統寄信流程");
			expect(data.listName).toBe("To do");
			expect(data.teamName).toBe("開發組");
			expect(data.assignees.map((assignee) => assignee.name)).toEqual(["Yorukot"]);
			expect(data.labels.map((label) => label.name)).toEqual(["Priority::High", "Backend"]);
		});

		it("auto-dismisses the share toast", async () => {
			stubImageClipboard(consumingWrite());
			vi.mocked(renderShareCardImage).mockResolvedValue(pngBlob);
			render(<Harness />);
			fireEvent.click(screen.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }));
			const dialog = screen.getByRole("dialog", { name: /127 卡片詳細資料/ });

			vi.useFakeTimers();
			try {
				fireEvent.click(within(dialog).getByRole("button", { name: "分享圖片" }));
				await act(async () => {
					await vi.advanceTimersByTimeAsync(0);
				});
				expect(screen.getByText("已複製卡片圖片")).toBeVisible();
				await act(async () => {
					await vi.advanceTimersByTimeAsync(4000);
				});
				expect(screen.queryByText("已複製卡片圖片")).not.toBeInTheDocument();
			} finally {
				vi.useRealTimers();
			}
		});

		it("falls back to a download when the clipboard write is refused", async () => {
			const user = userEvent.setup();
			stubImageClipboard(
				vi.fn(async () => {
					throw new DOMException("denied", "NotAllowedError");
				})
			);
			vi.mocked(renderShareCardImage).mockResolvedValue(pngBlob);
			URL.createObjectURL = vi.fn(() => "blob:fake");
			URL.revokeObjectURL = vi.fn();
			const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
			try {
				const dialog = await openShareableCard(user);

				await user.click(within(dialog).getByRole("button", { name: "分享圖片" }));

				expect(await screen.findByText("無法複製圖片，已改為下載")).toBeVisible();
				expect(click).toHaveBeenCalledOnce();
			} finally {
				click.mockRestore();
			}
		});

		it("reports a failed image render as an alert", async () => {
			const user = userEvent.setup();
			stubImageClipboard(consumingWrite());
			vi.mocked(renderShareCardImage).mockRejectedValue(new Error("no canvas"));
			const dialog = await openShareableCard(user);

			await user.click(within(dialog).getByRole("button", { name: "分享圖片" }));

			expect(await screen.findByText("無法產生卡片圖片，請再試一次")).toBeVisible();
			expect(screen.getByRole("alert")).toHaveTextContent("無法產生卡片圖片，請再試一次");
		});
	});
});
