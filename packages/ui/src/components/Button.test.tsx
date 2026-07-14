import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { axe } from "vitest-axe";
import { Button } from "./Button";
import { IconButton } from "./IconButton";

describe("button primitives", () => {
	it("exposes loading state without changing its label", () => {
		render(<Button loading>Save changes</Button>);
		expect(screen.getByRole("button", { name: /save changes/i })).toBeDisabled();
		expect(screen.getByRole("status")).toHaveTextContent("Working");
	});

	it("requires and renders an accessible icon label", async () => {
		const { container } = render(<IconButton label="Delete task" icon={<span aria-hidden="true">x</span>} />);
		expect(screen.getByRole("button", { name: "Delete task" })).toBeVisible();
		expect((await axe(container)).violations).toEqual([]);
	});
});
