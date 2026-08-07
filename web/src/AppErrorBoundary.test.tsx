import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AppErrorBoundary } from "./AppErrorBoundary";

function BrokenView(): never {
	throw new Error("render failed");
}

afterEach(() => {
	vi.restoreAllMocks();
});

it("shows a recoverable fallback instead of an empty root", () => {
	vi.spyOn(console, "error").mockImplementation(() => undefined);

	render(
		<AppErrorBoundary>
			<BrokenView />
		</AppErrorBoundary>
	);

	expect(screen.getByRole("alert")).toHaveTextContent("看板畫面發生錯誤");
	expect(screen.getByRole("button", { name: "重新載入" })).toBeVisible();
});
