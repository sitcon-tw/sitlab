import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LoginPage } from "./LoginPage";

describe("login page", () => {
	it("uses the Material button primitive without changing GitLab navigation", () => {
		render(<LoginPage />);

		const login = screen.getByRole("link", { name: "使用 GitLab 登入" });
		expect(login).toHaveAttribute("href", "/api/v1/auth/gitlab");
		expect(login).toHaveClass("md-button", "md-button--filled");
		expect(screen.getByRole("heading", { name: "籌備工作看板" })).toHaveClass("md-typescale-headline-medium");
	});
});
