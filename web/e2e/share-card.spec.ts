import { expect, test } from "@playwright/test";

const demoEnabled = process.env.E2E_DEMO === "true";

test.describe("share card image", () => {
	test.skip(!demoEnabled, "requires the explicit VITE_SITCON_DEMO server");

	test("copies a PNG of the card to the clipboard", async ({ page, context }, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "clipboard permission grants are chromium-only");
		await context.grantPermissions(["clipboard-read", "clipboard-write"]);

		await page.goto("/");
		await page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }).click();
		const dialog = page.getByRole("dialog", { name: /127 卡片詳細資料/ });
		await dialog.getByRole("button", { name: "分享圖片" }).click();

		await expect(page.getByText("已複製卡片圖片")).toBeVisible();
		// Real canvas, real clipboard: the exported PNG actually landed there.
		const pngSize = await page.evaluate(async () => {
			for (const item of await navigator.clipboard.read()) {
				if (item.types.includes("image/png")) return (await item.getType("image/png")).size;
			}
			return 0;
		});
		expect(pngSize).toBeGreaterThan(0);
	});
});
