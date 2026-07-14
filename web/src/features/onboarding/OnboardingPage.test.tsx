import { savePreferences } from "@/features/board/boardApi";
import type { Bootstrap } from "@/features/board/model";
import { demoBootstrap } from "@/test/demoBootstrap";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OnboardingPage } from "./OnboardingPage";

vi.mock("@/features/board/boardApi", () => ({ savePreferences: vi.fn() }));

function onboardingBootstrap(): Bootstrap {
	return {
		...structuredClone(demoBootstrap),
		preferences: { defaultTeamKey: null, confirmedAt: null, directoryTeamKeys: ["development"] }
	};
}

describe("primary team onboarding", () => {
	beforeEach(() => {
		vi.mocked(savePreferences).mockReset();
		vi.mocked(savePreferences).mockResolvedValue({
			preferences: { defaultTeamKey: "development", confirmedAt: "2026-07-14T09:00:00Z", directoryTeamKeys: ["development"] }
		});
	});

	it("preselects the directory team but requires explicit confirmation", async () => {
		const user = userEvent.setup();
		const updateBootstrap = vi.fn();
		render(<OnboardingPage bootstrap={onboardingBootstrap()} updateBootstrap={updateBootstrap} />);

		expect(screen.getByRole("radio", { name: /開發組/ })).toHaveAttribute("aria-checked", "true");
		expect(screen.getByText("Yorukot")).toBeVisible();
		expect(savePreferences).not.toHaveBeenCalled();

		await user.click(screen.getByRole("button", { name: "確認主要組別" }));

		expect(savePreferences).toHaveBeenCalledWith("development");
		expect(updateBootstrap).toHaveBeenCalledOnce();
	});

	it("expands a team so the user can verify member names", async () => {
		const user = userEvent.setup();
		render(<OnboardingPage bootstrap={onboardingBootstrap()} updateBootstrap={vi.fn()} />);

		await user.click(screen.getByRole("button", { name: "展開設計組成員" }));

		expect(screen.getByText("周美亞")).toBeVisible();
		expect(screen.getByText("@mia")).toBeVisible();
	});
});
