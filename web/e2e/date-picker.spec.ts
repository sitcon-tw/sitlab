import { expect, test, type Locator } from "@playwright/test";
import { fileURLToPath } from "node:url";

const demoEnabled = process.env.E2E_DEMO === "true";

function docsAsset(filename: string) {
	return fileURLToPath(new URL(`../../docs/assets/${filename}`, import.meta.url));
}

async function firstCardDateField(board: Locator) {
	const card = board.getByRole("article").first();
	await card.scrollIntoViewIfNeeded();
	return {
		card,
		input: card.locator(".md-date-field__compact-input"),
		trigger: card.locator(".md-date-field__trigger")
	};
}

test.describe("product date picker visual regression", () => {
	test.skip(!demoEnabled, "requires the explicit VITE_SITCON_DEMO server");

	test("desktop calendar stays anchored and the card footer stays aligned", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name === "mobile", "desktop geometry is covered in Chromium");
		await page.addInitScript(() => localStorage.setItem("sitcon-board-theme", "dark"));
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");
		const board = page.getByRole("region", { name: "SITCON 2027 工作看板" });
		const { card, input, trigger } = await firstCardDateField(board);

		await expect(input).toHaveValue(/^\d{4}\/\d{2}\/\d{2}$/);
		const footerCenters = await card.locator("footer > *").evaluateAll((items) =>
			items.map((item) => {
				const rect = item.getBoundingClientRect();
				return rect.top + rect.height / 2;
			})
		);
		expect(Math.max(...footerCenters) - Math.min(...footerCenters)).toBeLessThanOrEqual(1);

		await trigger.click();
		const picker = page.locator(".md-date-picker--popover");
		const scroller = picker.locator(".md-date-picker__scroll");
		await expect(picker).toBeVisible();
		await expect(picker.locator(".md-date-picker__day")).toHaveCount(308);
		await expect(picker.getByRole("heading", { name: "2026年8月" })).toBeVisible();
		const geometry = await page.evaluate(() => {
			const triggerRect = document.querySelector<HTMLElement>(".md-date-field__trigger[data-state='open']")!.getBoundingClientRect();
			const pickerRect = document.querySelector<HTMLElement>(".md-date-picker--popover")!.getBoundingClientRect();
			const scrollRect = document.querySelector<HTMLElement>(".md-date-picker__scroll")!.getBoundingClientRect();
			const todayRect = document.querySelector<HTMLElement>(".md-date-picker__day[data-today='true']")!.getBoundingClientRect();
			const days = [...document.querySelectorAll<HTMLElement>(".md-date-picker__day")].map((day) => day.getBoundingClientRect());
			return {
				viewportWidth: window.innerWidth,
				trigger: { top: triggerRect.top, right: triggerRect.right, bottom: triggerRect.bottom, left: triggerRect.left },
				picker: { top: pickerRect.top, right: pickerRect.right, bottom: pickerRect.bottom, left: pickerRect.left, width: pickerRect.width },
				todayCenterOffset: Math.abs(todayRect.top + todayRect.height / 2 - (scrollRect.top + scrollRect.height / 2)),
				minDaySize: Math.min(...days.map((day) => Math.min(day.width, day.height))),
				columns: new Set(days.slice(0, 7).map((day) => Math.round(day.left))).size
			};
		});
		expect(geometry.picker.width).toBeGreaterThanOrEqual(300);
		expect(geometry.picker.width).toBeLessThanOrEqual(324);
		expect(geometry.picker.left).toBeGreaterThanOrEqual(0);
		expect(geometry.picker.right).toBeLessThanOrEqual(geometry.viewportWidth);
		expect(geometry.minDaySize).toBeGreaterThanOrEqual(44);
		expect(geometry.columns).toBe(7);
		expect(geometry.todayCenterOffset).toBeLessThanOrEqual(1);
		const verticalGap = Math.min(Math.abs(geometry.picker.top - geometry.trigger.bottom), Math.abs(geometry.trigger.top - geometry.picker.bottom));
		expect(verticalGap).toBeLessThanOrEqual(8);

		await page.screenshot({ path: docsAsset("sitcon-board-date-picker-dark-desktop.png") });

		const pageScrollBefore = await page.evaluate(() => window.scrollY);
		const calendarScrollBefore = await scroller.evaluate((element) => element.scrollTop);
		await scroller.hover();
		await page.mouse.wheel(0, 100);
		await expect.poll(() => scroller.evaluate((element) => element.scrollTop)).toBeGreaterThan(calendarScrollBefore);
		await expect(picker.getByRole("heading", { name: "2026年9月" })).toBeVisible();
		expect(await page.evaluate(() => window.scrollY)).toBe(pageScrollBefore);
	});

	test("narrow calendar is centered and keeps touch-sized dates", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name !== "mobile", "narrow geometry is covered by the mobile project");
		await page.addInitScript(() => localStorage.setItem("sitcon-board-theme", "dark"));
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto("/");
		const board = page.getByRole("region", { name: "SITCON 2027 工作看板" });
		const { input, trigger } = await firstCardDateField(board);
		await expect(input).toHaveValue(/^\d{4}\/\d{2}\/\d{2}$/);

		await trigger.click();
		const picker = page.getByRole("dialog", { name: /日期選擇器/ });
		await expect(picker).toHaveClass(/md-date-picker--dialog/);
		const geometry = await picker.evaluate((element) => {
			const rect = element.getBoundingClientRect();
			const day = element.querySelector<HTMLElement>(".md-date-picker__day")!.getBoundingClientRect();
			return {
				x: rect.left + rect.width / 2,
				y: rect.top + rect.height / 2,
				viewportX: window.innerWidth / 2,
				viewportY: window.innerHeight / 2,
				width: rect.width,
				daySize: Math.min(day.width, day.height)
			};
		});
		expect(Math.abs(geometry.x - geometry.viewportX)).toBeLessThanOrEqual(1);
		expect(Math.abs(geometry.y - geometry.viewportY)).toBeLessThanOrEqual(1);
		expect(geometry.width).toBeLessThanOrEqual(382);
		expect(geometry.daySize).toBeGreaterThanOrEqual(44);

		await page.screenshot({ path: docsAsset("sitcon-board-date-picker-dark-mobile.png") });
	});
});
