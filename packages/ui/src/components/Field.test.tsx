import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { axe } from "vitest-axe";
import { SelectField, TextAreaField, TextField } from "./Field";

describe("field primitives", () => {
	it("keeps the label associated with each control", () => {
		render(
			<>
				<TextField label="Title" />
				<TextAreaField label="Description" />
				<SelectField label="Status" options={[{ value: "todo", label: "To do" }]} />
			</>
		);
		expect(screen.getByRole("textbox", { name: "Title" })).toBeVisible();
		expect(screen.getByRole("textbox", { name: "Description" })).toBeVisible();
		expect(screen.getByRole("combobox", { name: "Status" })).toBeVisible();
	});

	it("wires description and error to the control and marks it invalid", () => {
		render(<TextField label="Title" description="Shown on the card." error="A title is required." />);
		const input = screen.getByRole("textbox", { name: /title/i });
		expect(input).toHaveAttribute("aria-invalid", "true");
		expect(input).toHaveAccessibleDescription("Shown on the card. A title is required.");
	});

	it("keeps a placeholder present so the floating label has an emptiness signal", async () => {
		const user = userEvent.setup();
		render(<TextField label="Title" />);
		const input = screen.getByRole("textbox", { name: "Title" });
		expect(input).toHaveAttribute("placeholder", " ");
		await user.type(input, "Ship it");
		expect(input).toHaveValue("Ship it");
	});

	it("passes an explicit placeholder through", () => {
		render(<TextField label="Title" placeholder="Prepare quarterly review" />);
		expect(screen.getByRole("textbox", { name: "Title" })).toHaveAttribute("placeholder", "Prepare quarterly review");
	});

	// The notched outline is a <fieldset>/<legend>. Without aria-hidden it exposes
	// an empty group whose legend duplicates the label.
	it("has no accessibility violations across all three field types and the error state", async () => {
		const { container } = render(
			<>
				<TextField label="Title" />
				<TextAreaField label="Description" description="Markdown is supported." />
				<SelectField label="Status" options={[{ value: "todo", label: "To do" }]} />
				<TextField label="Due date" error="Pick a date in the future." />
			</>
		);
		expect(container.querySelector(".md-field__outline")).toHaveAttribute("aria-hidden", "true");
		expect((await axe(container)).violations).toEqual([]);
	});
});
