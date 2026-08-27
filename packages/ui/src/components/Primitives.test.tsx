import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { Button } from "./Button";
import { DataTable } from "./DataTable";
import { Dialog } from "./Dialog";
import { EmptyState } from "./EmptyState";
import { Menu, MenuDivider, MenuItem } from "./Menu";
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

	// The board previously used <details> popovers closed by writing
	// ref.current.open = false: no focus trap, no roving tabindex, no Escape.
	it("traps focus in a menu, roves with arrow keys, and restores focus on Escape", async () => {
		const user = userEvent.setup();
		const onSelect = vi.fn();
		render(
			<Menu label="Account" trigger={<Button>Open menu</Button>}>
				<MenuItem onSelect={onSelect}>Archive</MenuItem>
				<MenuItem>Duplicate</MenuItem>
				<MenuDivider />
				<MenuItem disabled>Delete</MenuItem>
			</Menu>
		);
		const trigger = screen.getByRole("button", { name: "Open menu" });
		await user.click(trigger);

		const menu = await screen.findByRole("menu");
		expect(menu).toContainElement(document.activeElement as HTMLElement);
		await user.keyboard("{ArrowDown}");
		expect(within(menu).getByRole("menuitem", { name: "Archive" })).toHaveFocus();
		await user.keyboard("{ArrowDown}");
		expect(within(menu).getByRole("menuitem", { name: "Duplicate" })).toHaveFocus();

		await user.keyboard("{Escape}");
		await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
		expect(trigger).toHaveFocus();
	});

	it("renders asChild buttons as the caller's element", () => {
		render(
			<Button asChild variant="text">
				<a href="#somewhere">Open GitLab</a>
			</Button>
		);
		const link = screen.getByRole("link", { name: "Open GitLab" });
		expect(link.tagName).toBe("A");
		expect(link).toHaveClass("md-button", "md-button--text");
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
