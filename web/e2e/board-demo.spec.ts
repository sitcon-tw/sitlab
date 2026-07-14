import { expect, test } from "@playwright/test";

const demoEnabled = process.env.E2E_DEMO === "true";

test.describe("SITCON Board demo visual audit", () => {
	test.skip(!demoEnabled, "requires the explicit VITE_SITCON_DEMO server");

	for (const viewport of [
		{ name: "desktop", width: 1440, height: 900 },
		{ name: "compact", width: 928, height: 800 },
		{ name: "tablet", width: 608, height: 800 },
		{ name: "narrow", width: 320, height: 720 }
	]) {
		test(`${viewport.name} ${viewport.width}px stays contained`, async ({ page }) => {
			await page.setViewportSize(viewport);
			await page.goto("/");
			await expect(page.getByRole("heading", { name: "Todo" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();

			const layout = await page.evaluate(() => {
				const quick = document.querySelector("form");
				const controls = quick
					? [...quick.querySelectorAll<HTMLElement>(":scope > select, :scope > input, :scope > button, :scope > label, :scope > div")].filter(
							(item) => item.offsetWidth > 2 && item.offsetHeight > 2
						)
					: [];
				const quickRect = quick?.getBoundingClientRect();
				return {
					viewport: window.innerWidth,
					documentWidth: document.documentElement.scrollWidth,
					quickContained: Boolean(
						quickRect &&
						controls.every((item) => {
							const rect = item.getBoundingClientRect();
							return rect.left >= quickRect.left - 1 && rect.right <= quickRect.right + 1;
						})
					)
				};
			});
			expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport);
			expect(layout.quickContained).toBe(true);

			if (viewport.name === "desktop" || viewport.name === "narrow") {
				await page.screenshot({ path: `../docs/assets/sitcon-board-${viewport.name}.png`, fullPage: true });
			}
		});
	}

	test("member drawer and assignee dialog are complete", async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto("/");

		await page.getByRole("button", { name: "成員" }).click();
		await expect(page.getByRole("dialog", { name: "籌備團隊" })).toBeVisible();
		await expect(page.getByRole("heading", { name: /開發組/ })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-members-mobile.png", fullPage: true });
		await page.getByRole("button", { name: "Close drawer" }).click();

		await page.getByRole("button", { name: "選擇新卡片 Assignee" }).click();
		await expect(page.getByRole("dialog", { name: "選擇 Assignee" })).toBeVisible();
		await expect(page.getByRole("button", { name: /Yorukot/ })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-assignee-mobile.png", fullPage: true });
	});
});
