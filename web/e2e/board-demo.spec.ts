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
			await expect(page.getByRole("heading", { name: "To Do" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();

			const layout = await page.evaluate(() => {
				const quick = document.querySelector("form");
				const filters = document.querySelector<HTMLElement>('section[aria-label="篩選看板"]');
				const controls = quick
					? [...quick.querySelectorAll<HTMLElement>(":scope > select, :scope > input, :scope > button, :scope > label, :scope > div")].filter(
							(item) => item.offsetWidth > 2 && item.offsetHeight > 2
						)
					: [];
				const filterControls = filters
					? [...filters.querySelectorAll<HTMLElement>(":scope > label, :scope > button, :scope > div, :scope > span")].filter(
							(item) => item.offsetWidth > 2 && item.offsetHeight > 2
						)
					: [];
				const quickRect = quick?.getBoundingClientRect();
				const filtersRect = filters?.getBoundingClientRect();
				return {
					viewport: window.innerWidth,
					documentWidth: document.documentElement.scrollWidth,
					quickContained: Boolean(
						quickRect &&
						controls.every((item) => {
							const rect = item.getBoundingClientRect();
							return rect.left >= quickRect.left - 1 && rect.right <= quickRect.right + 1;
						})
					),
					filtersContained: Boolean(
						filtersRect &&
						filterControls.every((item) => {
							const rect = item.getBoundingClientRect();
							return rect.left >= filtersRect.left - 1 && rect.right <= filtersRect.right + 1;
						})
					)
				};
			});
			expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport);
			expect(layout.quickContained).toBe(true);
			expect(layout.filtersContained).toBe(true);

			if (viewport.name === "desktop" || viewport.name === "narrow") {
				await page.screenshot({ path: `../docs/assets/sitcon-board-${viewport.name}.png`, fullPage: true });
			}
		});
	}

	test("team and people filters combine on the board", async ({ page }) => {
		await page.setViewportSize({ width: 608, height: 800 });
		await page.goto("/");

		await page.getByLabel("篩選組別").selectOption("design");
		await expect(page.getByRole("heading", { name: "[設計組] 製作工作人員識別證" })).toBeVisible();
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeHidden();

		await page.getByRole("button", { name: "篩選負責人" }).click();
		const picker = page.getByRole("dialog", { name: "篩選負責人" });
		await picker.getByRole("checkbox", { name: /Yorukot/ }).press("Enter");
		await picker.getByRole("checkbox", { name: /林采欣/ }).click();
		await expect(picker.getByText("已選擇 2 人")).toBeVisible();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("status")).toHaveText("0 / 7 張卡片");

		await page.getByRole("button", { name: "清除篩選" }).click();
		await expect(page.getByRole("status")).toHaveText("7 / 7 張卡片");
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
	});

	test("team select all and due date sorting update the board", async ({ page }) => {
		await page.setViewportSize({ width: 928, height: 800 });
		await page.goto("/");

		const doing = page.getByRole("heading", { name: "Doing" }).locator("..").locator("..");
		await page.getByLabel("排序方式").selectOption("due-desc");
		await expect(doing.getByRole("heading", { level: 3 }).first()).toHaveText("[場務組] 盤點會場網路設備");

		await page.getByRole("button", { name: "篩選負責人" }).click();
		const picker = page.getByRole("dialog", { name: "篩選負責人" });
		await picker.getByRole("checkbox", { name: "全選行政組" }).click();
		await expect(picker.getByText("已選擇 2 人")).toBeVisible();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("status")).toHaveText("1 / 7 張卡片");
		await expect(page.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
	});

	test("quick create more options set status and description", async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto("/");

		await page.getByRole("button", { name: "更多建卡選項" }).click();
		const dialog = page.getByRole("dialog", { name: "更多建卡選項" });
		await expect(dialog.getByLabel("新卡片 Status")).toHaveValue("inbox");
		await dialog.getByLabel("新卡片 Status").selectOption("doing");
		await dialog.getByLabel("新卡片 Description").fill("確認交接與值班時段");
		await page.screenshot({ path: "../docs/assets/sitcon-board-quick-create-more-mobile.png", fullPage: true });
		await dialog.getByRole("button", { name: "套用" }).click();
		await page.getByLabel("卡片標題").fill("新增值班表");
		await page.getByRole("button", { name: "建立卡片" }).click();

		const doing = page.getByRole("heading", { name: "Doing" }).locator("..").locator("..");
		await expect(doing.getByRole("heading", { level: 3 }).first()).toHaveText("[開發組] 新增值班表");
		await expect(doing.getByText("確認交接與值班時段")).toBeVisible();
	});

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
		await expect(page.getByRole("checkbox", { name: /Yorukot/ })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-assignee-mobile.png", fullPage: true });
	});

	test("card details expose planning, scheduling, and multiple assignees", async ({ page }) => {
		await page.setViewportSize({ width: 928, height: 800 });
		await page.goto("/");
		await page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }).click();

		const details = page.getByRole("dialog", { name: "#127 卡片詳細資料" });
		await expect(details.getByLabel("組別", { exact: true })).toHaveValue("development");
		await expect(details.getByLabel("狀態")).toHaveValue("todo");
		await expect(details.getByLabel("Start")).toHaveValue("2026-07-17");
		await expect(details.getByLabel("Due")).toHaveValue("2026-07-21");
		await expect(details.getByText("組別::開發")).toBeVisible();
		await expect(details.getByText("Priority::High")).toBeVisible();
		await details.getByRole("textbox", { name: "描述" }).fill("## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)");
		await details.getByRole("button", { name: "預覽" }).click();
		await expect(details.getByRole("heading", { name: "驗收條件" })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-details.png", fullPage: true });
		await details.getByRole("button", { name: "變更 Assignee" }).click();
		const picker = page.getByRole("dialog", { name: "選擇 Assignee" });
		await picker.getByRole("checkbox", { name: /沈明軒/ }).click();
		await expect(picker.getByText("已選擇 2 人")).toBeVisible();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(details.getByText("系統活動")).toBeVisible();
		await details.getByRole("textbox", { name: "Comment" }).fill("測試與監控紀錄已補齊。");
		await details.getByRole("button", { name: "送出 Comment" }).click();
		await expect(details.getByText("測試與監控紀錄已補齊。")).toBeVisible();
		await details.getByRole("heading", { name: "Comment" }).scrollIntoViewIfNeeded();
		await page.screenshot({ path: "../docs/assets/sitcon-board-tags-comments.png", fullPage: true });
		await details.getByRole("button", { name: "儲存細節" }).click();
		await expect(details).toBeVisible();
	});

	test("card details stay operable at 320px", async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 720 });
		await page.goto("/");
		await page.getByRole("heading", { name: "[議程組] 確認議程講者資料" }).click();

		const details = page.getByRole("dialog", { name: "#129 卡片詳細資料" });
		await expect(details.getByLabel("標題")).toBeVisible();
		await expect(details.getByRole("textbox", { name: "描述" })).toBeVisible();
		const startDate = details.getByLabel("Start");
		await startDate.scrollIntoViewIfNeeded();
		await expect(startDate).toBeInViewport();
		await expect(startDate).toHaveValue("");
		await expect(details.getByLabel("Due")).toHaveValue("2026-07-25");
		await details.getByRole("heading", { name: "Tag" }).scrollIntoViewIfNeeded();
		await expect(details.getByLabel("新增 Tag")).toBeVisible();
		await details.getByRole("textbox", { name: "Comment" }).scrollIntoViewIfNeeded();
		await expect(details.getByRole("textbox", { name: "Comment" })).toBeInViewport();
		await page.screenshot({ path: "../docs/assets/sitcon-board-tags-comments-mobile.png", fullPage: true });
		await expect(details.getByRole("button", { name: "儲存細節" })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-details-mobile.png", fullPage: true });
	});
});
