import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { axe } from "vitest-axe";
import { AuthPage } from "./AuthPage";

const session = {
	user: null,
	loading: false,
	submitting: false,
	login: vi.fn(),
	register: vi.fn(),
	logout: vi.fn()
};

vi.mock("./SessionContext", () => ({ useSession: () => session }));

describe("AuthPage", () => {
	beforeEach(() => vi.clearAllMocks());

	it("renders an accessible login workflow", async () => {
		const { container } = render(
			<MemoryRouter>
				<AuthPage mode="login" />
			</MemoryRouter>
		);
		expect(screen.getByRole("heading", { name: "Welcome back" })).toBeVisible();
		expect(screen.getByLabelText("Email")).toHaveAttribute("type", "email");
		expect(screen.getByLabelText("Password")).toHaveAttribute("autocomplete", "current-password");
		expect((await axe(container)).violations).toEqual([]);
	});

	it("makes account creation requirements explicit", () => {
		render(
			<MemoryRouter>
				<AuthPage mode="register" />
			</MemoryRouter>
		);
		expect(screen.getByLabelText("Name")).toBeRequired();
		expect(screen.getByText("Use at least 12 characters.")).toBeVisible();
	});
});
