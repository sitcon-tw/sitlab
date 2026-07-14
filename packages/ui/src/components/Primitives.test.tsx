import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { DataTable } from "./DataTable";
import { Dialog } from "./Dialog";
import { EmptyState } from "./EmptyState";
import { Tabs } from "./Tabs";

function DialogHarness() {
	const [open, setOpen] = useState(false);
	return (
		<Dialog open={open} onOpenChange={setOpen} title="Edit profile" description="Update your public details." trigger={<Button>Open profile</Button>}>
			<label htmlFor="profile-name">Name</label>
			<input id="profile-name" />
		</Dialog>
	);
}

describe("overlay and collection primitives", () => {
	it("labels the dialog, contains focus, closes on Escape, and restores focus", async () => {
		const user = userEvent.setup();
		render(<DialogHarness />);
		const trigger = screen.getByRole("button", { name: "Open profile" });
		await user.click(trigger);
		const dialog = screen.getByRole("dialog", { name: "Edit profile" });
		expect(dialog).toHaveAccessibleDescription("Update your public details.");
		expect(dialog).toContainElement(document.activeElement as HTMLElement);
		await user.keyboard("{Escape}");
		await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
		expect(trigger).toHaveFocus();
	});

	it("supports arrow-key tab activation", async () => {
		const user = userEvent.setup();
		render(
			<Tabs
				label="Task views"
				items={[
					{ value: "open", label: "Open", content: <p>Open tasks</p> },
					{ value: "done", label: "Done", content: <p>Completed tasks</p> }
				]}
			/>
		);
		const tablist = screen.getByRole("tablist", { name: "Task views" });
		const openTab = within(tablist).getByRole("tab", { name: "Open" });
		openTab.focus();
		await user.keyboard("{ArrowRight}");
		expect(within(tablist).getByRole("tab", { name: "Done" })).toHaveAttribute("aria-selected", "true");
		expect(screen.getByText("Completed tasks")).toBeVisible();
	});

	it("renders an accessible table and delegates its empty state", () => {
		const columns = [{ key: "name", header: "Task", cell: (row: { id: string; name: string }) => row.name }];
		const { rerender } = render(
			<DataTable label="Workspace tasks" rows={[{ id: "1", name: "Review access" }]} columns={columns} getRowKey={(row) => row.id} />
		);
		const table = screen.getByRole("table", { name: "Workspace tasks" });
		expect(within(table).getByRole("columnheader", { name: "Task" })).toBeVisible();
		expect(within(table).getByRole("cell", { name: "Review access" })).toBeVisible();
		rerender(
			<DataTable
				label="Workspace tasks"
				rows={[]}
				columns={columns}
				getRowKey={(row) => row.id}
				empty={<EmptyState title="No tasks" description="Create the first task." />}
			/>
		);
		expect(screen.queryByRole("table")).not.toBeInTheDocument();
		expect(screen.getByRole("heading", { name: "No tasks" })).toBeVisible();
	});
});
