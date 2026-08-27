import { demoBootstrap } from "@/test/demoBootstrap";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LabelManagerDialog } from "./LabelManagerDialog";
import { defaultLabelColor, labelPalette } from "./labelPalette";
import { createProjectLabel, deleteProjectLabel, listProjectLabels, updateProjectLabel } from "./labelsApi";
import type { ProjectLabel } from "./model";

vi.mock("./labelsApi", () => ({
	listProjectLabels: vi.fn(),
	createProjectLabel: vi.fn(),
	updateProjectLabel: vi.fn(),
	deleteProjectLabel: vi.fn()
}));

const catalog: ProjectLabel[] = [
	{ id: 1, name: "Team::開發組", color: "#0E8A16", textColor: "#FFFFFF", description: "開發組" },
	{ id: 3, name: "Priority::High", color: "#D73A4A", textColor: "#FFFFFF", description: "優先處理" },
	{ id: 4, name: "Backend", color: "#1D76DB", textColor: "#FFFFFF", description: null }
];

function Harness({ onCreated }: { onCreated?: (label: ProjectLabel) => void } = {}) {
	const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }));
	return (
		<QueryClientProvider client={client}>
			<LabelManagerDialog open onOpenChange={() => {}} bootstrap={demoBootstrap} {...(onCreated ? { onCreated } : {})} />
		</QueryClientProvider>
	);
}

describe("label manager", () => {
	beforeEach(() => {
		vi.mocked(listProjectLabels).mockReset().mockResolvedValue(catalog);
		vi.mocked(createProjectLabel).mockReset();
		vi.mocked(updateProjectLabel).mockReset();
		vi.mocked(deleteProjectLabel).mockReset();
	});

	it("separates manageable labels from the ones the directory owns", async () => {
		render(<Harness />);
		expect(await screen.findByText("Priority::High")).toBeVisible();
		const manageable = screen.getByRole("list", { name: "可管理的 Labels" });
		expect(within(manageable).queryByText("Team::開發組")).not.toBeInTheDocument();

		const reserved = screen.getByRole("list", { name: "保留的 Labels" });
		expect(within(reserved).getByText("Team::開發組")).toBeVisible();
		// Reserved rows are shown so nobody hunts for them, but offer no actions.
		expect(within(reserved).queryAllByRole("button")).toHaveLength(0);
	});

	it("creates a label with a normalized color and hands it back to the caller", async () => {
		const user = userEvent.setup();
		const chosen = labelPalette[5]!;
		const created: ProjectLabel = { id: 9, name: "Docs", color: chosen, textColor: "#FFFFFF", description: null };
		vi.mocked(createProjectLabel).mockResolvedValue(created);
		const onCreated = vi.fn();
		render(<Harness onCreated={onCreated} />);

		await screen.findByText("Backend");
		await user.type(screen.getByLabelText("新 Label 名稱"), "Docs");
		await user.click(screen.getByRole("button", { name: `使用顏色 ${chosen}` }));
		await user.click(screen.getByRole("button", { name: "建立 Label" }));

		await waitFor(() => expect(createProjectLabel).toHaveBeenCalledWith({ name: "Docs", color: chosen, description: null }));
		expect(onCreated).toHaveBeenCalledWith(created);
	});

	it("renames a label through its inline edit row", async () => {
		const user = userEvent.setup();
		vi.mocked(updateProjectLabel).mockResolvedValue({ ...catalog[2]!, name: "Server" });
		render(<Harness />);

		await user.click(await screen.findByRole("button", { name: "編輯 Backend" }));
		const nameField = screen.getByLabelText("編輯名稱");
		await user.clear(nameField);
		await user.type(nameField, "Server");
		await user.click(screen.getByRole("button", { name: "儲存" }));

		await waitFor(() => expect(updateProjectLabel).toHaveBeenCalledWith(4, { name: "Server", color: defaultLabelColor, description: null }));
	});

	it("states how many cards a delete affects before confirming", async () => {
		const user = userEvent.setup();
		vi.mocked(deleteProjectLabel).mockResolvedValue(undefined);
		render(<Harness />);

		await user.click(await screen.findByRole("button", { name: "刪除 Backend" }));
		const confirm = await screen.findByRole("alertdialog");
		const affected = demoBootstrap.board.cards.filter((card) => card.labels.includes("Backend")).length;
		expect(confirm).toHaveTextContent(`刪除後，${affected} 張卡片會失去這個 Label，且無法復原。`);
		// The cancel affordance is localized, not the primitive's English default.
		expect(within(confirm).getByRole("button", { name: "取消" })).toBeVisible();

		await user.click(within(confirm).getByRole("button", { name: "刪除" }));
		await waitFor(() => expect(deleteProjectLabel).toHaveBeenCalledWith(4));
	});

	it("surfaces a failed create without closing the dialog", async () => {
		const user = userEvent.setup();
		vi.mocked(createProjectLabel).mockRejectedValue(new Error("已經有同名的 Label。"));
		render(<Harness />);

		await screen.findByText("Backend");
		await user.type(screen.getByLabelText("新 Label 名稱"), "Backend");
		await user.click(screen.getByRole("button", { name: "建立 Label" }));

		expect(await screen.findByRole("alert")).toHaveTextContent("已經有同名的 Label。");
	});
});
