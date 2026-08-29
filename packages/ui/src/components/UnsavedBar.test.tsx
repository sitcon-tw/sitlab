import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UnsavedBar } from "./UnsavedBar";

describe("unsaved bar", () => {
	it("shows the message and fires onRevert", async () => {
		const user = userEvent.setup();
		const onRevert = vi.fn();
		render(<UnsavedBar message="Careful — you haven't saved" revertLabel="Revert" saveLabel="Save" onRevert={onRevert} />);
		expect(screen.getByText("Careful — you haven't saved")).toBeVisible();
		await user.click(screen.getByRole("button", { name: "Revert" }));
		expect(onRevert).toHaveBeenCalledTimes(1);
	});

	it("disables both actions while saving", () => {
		render(<UnsavedBar message="Careful — you haven't saved" revertLabel="Revert" saveLabel="Save" savingLabel="Saving" saving onRevert={() => {}} />);
		expect(screen.getByRole("button", { name: "Revert" })).toBeDisabled();
		const save = screen.getByRole("button", { name: /save/i });
		expect(save).toBeDisabled();
		expect(save).toHaveAttribute("aria-busy", "true");
	});

	it("submits the surrounding form", async () => {
		const user = userEvent.setup();
		const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
		render(
			<form onSubmit={onSubmit}>
				<UnsavedBar message="Careful — you haven't saved" revertLabel="Revert" saveLabel="Save" onRevert={() => {}} />
			</form>
		);
		await user.click(screen.getByRole("button", { name: "Save" }));
		expect(onSubmit).toHaveBeenCalledTimes(1);
	});

	// CardDetail renders the bar outside its form (the form is a grid, so a
	// sticky child could not move) and relies on the HTML form attribute.
	it("submits a form it sits outside of via the form attribute", async () => {
		const user = userEvent.setup();
		const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
		render(
			<>
				<form id="detail-form" onSubmit={onSubmit}>
					<input aria-label="Title" />
				</form>
				<UnsavedBar message="Careful — you haven't saved" revertLabel="Revert" saveLabel="Save" form="detail-form" onRevert={() => {}} />
			</>
		);
		await user.click(screen.getByRole("button", { name: "Save" }));
		expect(onSubmit).toHaveBeenCalledTimes(1);
	});
});
