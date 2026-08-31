import { expect, test, type Locator, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { demoBootstrap } from "../src/test/demoBootstrap";

const demoEnabled = process.env.E2E_DEMO === "true";

function docsAsset(filename: string) {
	return fileURLToPath(new URL(`../../docs/assets/${filename}`, import.meta.url));
}

function closedLaneBootstrap(count: number) {
	const bootstrap = structuredClone(demoBootstrap);
	const closedList = bootstrap.board.lists.find((list) => list.closed)!;
	const template = bootstrap.board.cards.find((card) => card.listKey === closedList.key)!;
	bootstrap.board.cards = [
		...bootstrap.board.cards.filter((card) => card.listKey !== closedList.key),
		...Array.from({ length: count }, (_, index) => ({
			...template,
			issueIid: 2_000 + index,
			title: `近期完成 ${index + 1}`,
			position: index,
			updatedAt: new Date(Date.UTC(2026, 6, 1, 0, index)).toISOString()
		}))
	];
	return bootstrap;
}

function tallLaneBootstrap(count: number) {
	const bootstrap = structuredClone(demoBootstrap);
	const list = bootstrap.board.lists.find((item) => item.name === "To do")!;
	const template = bootstrap.board.cards.find((card) => card.listKey === list.key)!;
	bootstrap.board.cards = [
		...bootstrap.board.cards.filter((card) => card.listKey !== list.key),
		...Array.from({ length: count }, (_, index) => ({
			...template,
			issueIid: 3_000 + index,
			title: `待辦卡片 ${index + 1}`,
			position: index
		}))
	];
	return bootstrap;
}

async function injectBootstrap(page: Page, bootstrap: typeof demoBootstrap) {
	await page.route("**/", async (route) => {
		const response = await route.fetch();
		const body = await response.text();
		const serialized = JSON.stringify(bootstrap).replaceAll("<", "\\u003c");
		await route.fulfill({
			response,
			body: body.replace('<div id="root"></div>', `<div id="root"></div><script id="__SITCON_BOOTSTRAP__" type="application/json">${serialized}</script>`)
		});
	});
}

async function chooseSelectField(page: Page, root: Page | Locator, label: string, option: string) {
	await root.getByRole("button", { name: label, exact: true }).click();
	const menu = page.getByRole("menu", { name: `${label}選項` });
	await menu.getByRole("menuitemcheckbox", { name: option, exact: true }).click();
}

async function openDesktopFilter(page: Page, label: string) {
	const input = page.getByRole("combobox", { name: label });
	await input.click();
	const panelId = await input.getAttribute("aria-controls");
	if (!panelId) throw new Error(`${label} picker did not open`);
	return { input, picker: page.locator(`[id="${panelId}"]`) };
}

test.describe("SITCON Board demo visual audit", () => {
	test.skip(!demoEnabled, "requires the explicit VITE_SITCON_DEMO server");

	for (const theme of ["light", "dark"] as const) {
		for (const viewport of [
			{ name: "desktop", width: 1440, height: 900 },
			{ name: "compact", width: 928, height: 800 },
			{ name: "tablet", width: 608, height: 800 },
			{ name: "narrow", width: 320, height: 720 }
		]) {
			test(`${theme} ${viewport.name} ${viewport.width}px stays contained and shadow-free`, async ({ page }) => {
				await page.addInitScript((selectedTheme) => localStorage.setItem("sitcon-board-theme", selectedTheme), theme);
				await page.setViewportSize(viewport);
				await page.goto("/");
				await expect(page.getByRole("heading", { name: "To do" })).toBeVisible();
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
						shadowCount: [...document.querySelectorAll("*")].filter((element) => getComputedStyle(element).boxShadow !== "none").length,
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
				expect(layout.shadowCount).toBe(0);
				expect(layout.quickContained).toBe(true);
				expect(layout.filtersContained).toBe(true);

				if (viewport.name === "desktop" || viewport.name === "narrow") {
					const themePart = theme === "dark" ? "dark-" : "";
					await page.screenshot({ path: `../docs/assets/sitcon-board-${themePart}${viewport.name}.png`, fullPage: true });
				}
			});
		}
	}

	test("desktop board keeps full-width lanes and scrolls horizontally", async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");
		const board = page.getByRole("region", { name: "SITCON 2027 工作看板" });
		const dimensions = await board.evaluate((element) => ({ clientWidth: element.clientWidth, scrollWidth: element.scrollWidth }));
		expect(dimensions.scrollWidth).toBeGreaterThan(dimensions.clientWidth);
		await board.evaluate((element) => element.scrollTo({ left: element.scrollWidth }));
		await expect.poll(() => board.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
	});

	for (const viewport of [
		{ name: "desktop", width: 1440, height: 900 },
		{ name: "narrow", width: 320, height: 720 }
	]) {
		test(`card scrolling stays inside one column at ${viewport.name} width`, async ({ page }) => {
			await injectBootstrap(page, tallLaneBootstrap(40));
			await page.setViewportSize(viewport);
			await page.goto("/");

			const filters = page.getByRole("region", { name: "篩選看板" });
			const todoLane = page.locator('section[data-list="todo"]');
			const todoHeader = todoLane.getByRole("heading", { name: "To do" });
			const todoCardList = todoLane.getByRole("article").first().locator("..");
			const inboxCardList = page.locator('section[data-list="inbox"]').getByRole("article").first().locator("..");
			const before = {
				filters: await filters.boundingBox(),
				header: await todoHeader.boundingBox()
			};

			const dimensions = await todoCardList.evaluate((element) => ({ clientHeight: element.clientHeight, scrollHeight: element.scrollHeight }));
			expect(dimensions.scrollHeight).toBeGreaterThan(dimensions.clientHeight);
			await todoCardList.evaluate((element) => element.scrollTo({ top: element.scrollHeight }));
			await expect.poll(() => todoCardList.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);

			const after = {
				filters: await filters.boundingBox(),
				header: await todoHeader.boundingBox()
			};
			expect(after.filters?.y).toBeCloseTo(before.filters?.y ?? 0, 0);
			expect(after.header?.y).toBeCloseTo(before.header?.y ?? 0, 0);
			expect(await inboxCardList.evaluate((element) => element.scrollTop)).toBe(0);
			expect(await page.evaluate(() => window.scrollY)).toBe(0);
			expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBeLessThanOrEqual(viewport.height);
		});
	}

	for (const viewport of [
		{ name: "desktop", width: 1440, height: 900 },
		{ name: "narrow", width: 320, height: 720 }
	]) {
		test(`closed lane limits cards and reveals more at ${viewport.name} width`, async ({ page }) => {
			await page.addInitScript(() => localStorage.setItem("sitcon-board-theme", "dark"));
			await injectBootstrap(page, closedLaneBootstrap(55));
			await page.setViewportSize(viewport);
			await page.goto("/");

			const closedLane = page.locator('section[data-list="closed"]');
			await expect(closedLane.getByRole("article")).toHaveCount(50);
			await expect(closedLane.getByRole("heading", { name: /近期完成 55$/ })).toBeVisible();
			await expect(closedLane.getByRole("heading", { name: /近期完成 1$/ })).toHaveCount(0);
			await expect(closedLane.getByText("已顯示最近 50 / 55 個 Issue")).toBeVisible();

			const more = closedLane.getByRole("button", { name: "在 Close 顯示更多 5 個 Issue" });
			await more.scrollIntoViewIfNeeded();
			await expect(more).toBeVisible();
			const layout = await page.evaluate(() => ({ viewport: window.innerWidth, documentWidth: document.documentElement.scrollWidth }));
			expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport);
			await page.screenshot({ path: docsAsset(`sitcon-board-done-limit-dark-${viewport.name}.png`) });

			await more.click();
			await expect(closedLane.getByRole("article")).toHaveCount(55);
			await expect(closedLane.getByRole("heading", { name: /近期完成 1$/ })).toBeVisible();
			await expect(more).toHaveCount(0);
		});
	}

	for (const theme of ["light", "dark"] as const) {
		for (const viewport of [
			{ name: "desktop", width: 1440, height: 900 },
			{ name: "narrow", width: 320, height: 720 }
		]) {
			test(`${theme} Gantt ${viewport.name} stays contained and opens details`, async ({ page }) => {
				await page.addInitScript((selectedTheme) => localStorage.setItem("sitcon-board-theme", selectedTheme), theme);
				await page.setViewportSize(viewport);
				await page.goto("/?view=gantt");

				const gantt = page.getByRole("region", { name: "SITCON 2027 甘特圖" });
				await expect(gantt).toBeVisible();
				await expect(page.getByRole("region", { name: "篩選甘特圖" }).getByRole("status")).toHaveText("6 / 6 個開啟 Issue");
				await expect(gantt.getByText("完成主視覺社群素材")).toHaveCount(0);
				await expect(page.getByRole("button", { name: "排序方式" })).toHaveCount(0);
				const scale = gantt.getByRole("group", { name: "時間尺度" });
				await expect(scale.getByRole("button", { name: "日" })).toHaveAttribute("aria-pressed", "true");
				if (viewport.name === "narrow") {
					await scale.getByRole("button", { name: "週" }).click();
					await expect(page).toHaveURL(/scale=week/);
				}
				const issueBackground = await gantt
					.locator('[class*="issueCell"]')
					.first()
					.evaluate((element) => getComputedStyle(element).backgroundColor);
				expect(issueBackground).not.toBe("rgba(0, 0, 0, 0)");
				expect(issueBackground).not.toBe("transparent");

				const layout = await page.evaluate(() => ({ viewport: window.innerWidth, documentWidth: document.documentElement.scrollWidth }));
				expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport);
				await page.screenshot({ path: docsAsset(`sitcon-board-gantt-${theme}-${viewport.name}.png`), fullPage: true });

				await gantt.getByRole("button", { name: /開啟 Issue #127/ }).focus();
				await page.keyboard.press("Enter");
				await expect(page.getByRole("dialog", { name: /127 卡片詳細資料/ })).toBeVisible();
			});
		}
	}

	test("drag targeting follows the pointer after horizontal scrolling", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name === "mobile", "Playwright mobile emulation does not expose a stable touch-drag gesture");
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");
		const sortControl = page.getByRole("button", { name: "排序方式" });
		await expect(sortControl).toHaveAttribute("data-value", "due-asc");
		const board = page.getByRole("region", { name: "SITCON 2027 工作看板" });
		await board.evaluate((element) => element.scrollTo({ left: element.scrollWidth }));

		const handle = page.getByRole("button", { name: "拖曳 [行銷組] 完成主視覺社群素材" });
		const source = await handle.boundingBox();
		if (!source) throw new Error("drag source is not visible");
		const sourceCenter = { x: source.x + source.width / 2, y: source.y + source.height / 2 };
		await page.mouse.move(sourceCenter.x, sourceCenter.y);
		await page.mouse.down();
		await page.mouse.move(sourceCenter.x, sourceCenter.y - 150, { steps: 12 });
		await expect(page.locator('[class*="dragPreview"]')).toBeVisible();
		await expect(page.locator('section[data-list="closed"]')).not.toHaveAttribute("data-drag-over", "true");

		const reviewHeader = page.getByRole("heading", { name: "Review" });
		const target = await reviewHeader.boundingBox();
		if (!target) throw new Error("drag target is not visible");
		await page.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 12 });
		await expect(page.locator('section[data-list="review"]')).toHaveAttribute("data-drag-over", "true");
		await page.mouse.up();
		await expect(sortControl).toHaveAttribute("data-value", "due-asc");
		await expect(page.locator('section[data-list="review"]').getByRole("heading", { name: "[行銷組] 完成主視覺社群素材" })).toBeVisible();
	});

	test("quick create opens as a dialog and uses one focus indicator", async ({ page }) => {
		await page.setViewportSize({ width: 1360, height: 800 });
		await page.goto("/");

		// Creation is closed by default; the toolbar button opens the dialog.
		await expect(page.getByLabel("卡片標題")).toHaveCount(0);
		await page.getByRole("button", { name: "新增卡片" }).click();
		const dialog = page.getByRole("dialog", { name: "新增卡片" });
		const title = dialog.getByLabel("卡片標題");
		await expect(title).toBeVisible();

		await title.focus();
		await expect
			.poll(() =>
				title.evaluate((input) => {
					const indicator = input.closest(".md-field")?.querySelector(".md-field__outline");
					return indicator ? Number.parseFloat(getComputedStyle(indicator).borderBottomWidth) : 0;
				})
			)
			.toBeGreaterThanOrEqual(2);
		expect(await title.evaluate((input) => getComputedStyle(input).outlineStyle)).toBe("none");

		await page.keyboard.press("Escape");
		await expect(dialog).toHaveCount(0);
		await expect(page.getByRole("button", { name: "新增卡片" })).toBeFocused();
	});

	test("create dialog pins its title and stays inside a short viewport", async ({ page }) => {
		await page.setViewportSize({ width: 1112, height: 320 });
		await page.goto("/");
		await page.getByRole("button", { name: "新增卡片" }).click();
		await expect(page.getByRole("dialog", { name: "新增卡片" })).toBeVisible();

		const geometry = await page.evaluate(() => {
			const surface = document.querySelector<HTMLElement>(".md-dialog")!;
			const body = surface.querySelector<HTMLElement>(".md-dialog__body")!;
			const rect = surface.getBoundingClientRect();
			return {
				viewport: window.innerHeight,
				top: rect.top,
				bottom: rect.bottom,
				bodyScrollable: body.scrollHeight > body.clientHeight
			};
		});
		expect(geometry.top).toBeGreaterThanOrEqual(0);
		expect(geometry.bottom).toBeLessThanOrEqual(geometry.viewport);
		expect(geometry.bodyScrollable).toBe(true);

		// The submit control is reachable by scrolling the body, and the pinned
		// title never scrolls away with it.
		const create = page.getByRole("button", { name: "建立卡片" });
		await create.scrollIntoViewIfNeeded();
		await expect(create).toBeVisible();
		await expect(page.getByRole("dialog", { name: "新增卡片" }).locator(".md-dialog__title")).toBeInViewport();
	});

	test("desktop team filter selects an option by its text", async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");

		const { input, picker } = await openDesktopFilter(page, "搜尋組別");
		await picker.getByText("行政組", { exact: true }).click();

		await expect(input).toHaveValue("行政組");
		await expect(picker).toHaveCount(0);
		await expect(page.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeHidden();
	});

	test("team and people filters combine on the board", async ({ page }) => {
		await page.setViewportSize({ width: 608, height: 800 });
		await page.goto("/");

		await page.getByRole("button", { name: "篩選與排序" }).click();
		const picker = page.getByRole("dialog", { name: "篩選與排序" });
		const team = picker.getByRole("region", { name: "組別" });
		await team.getByRole("searchbox", { name: "搜尋組別" }).fill("設計");
		await team.getByText("設計組", { exact: true }).click();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("heading", { name: "[設計組] 製作工作人員識別證" })).toBeVisible();
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeHidden();

		await page.getByRole("button", { name: /篩選與排序/ }).click();
		const people = picker.getByRole("region", { name: "負責人" });
		await people.getByRole("checkbox", { name: /Yorukot/ }).press("Enter");
		await people.getByRole("checkbox", { name: /林采欣/ }).click();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("0 / 7 張卡片");

		await page.getByRole("button", { name: /篩選與排序/ }).click();
		await picker.getByRole("button", { name: "清除進階篩選" }).click();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("7 / 7 張卡片");
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
	});

	test("people pickers prioritize the current user's primary team", async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");

		const desktopFilter = await openDesktopFilter(page, "搜尋負責人");
		let picker = desktopFilter.picker;
		let primaryTeam = picker.getByRole("region").first();
		await expect(primaryTeam).toHaveAccessibleName("開發組");
		await expect(primaryTeam.getByRole("checkbox").nth(1)).toHaveAccessibleName(/Yorukot/);
		await page.screenshot({ path: `/tmp/sitcon-member-filter-${testInfo.project.name}.png`, fullPage: true });
		await desktopFilter.input.press("Escape");

		if (testInfo.project.name === "mobile") await page.setViewportSize({ width: 412, height: 915 });

		await chooseSelectField(page, page, "新卡片組別", "設計組");
		await page.getByRole("button", { name: "選擇新卡片 Assignee" }).click();
		picker = page.getByRole("dialog", { name: "選擇 Assignee" });
		primaryTeam = picker.getByRole("region").first();
		await expect(primaryTeam).toHaveAccessibleName("開發組");
		await expect(primaryTeam.getByRole("checkbox").nth(1)).toHaveAccessibleName(/Yorukot/);
		await page.screenshot({ path: `/tmp/sitcon-assignee-picker-${testInfo.project.name}.png`, fullPage: true });
	});

	test("team select all and due date sorting update the board", async ({ page }) => {
		await page.setViewportSize({ width: 928, height: 800 });
		await page.goto("/");

		const doing = page.locator('section[data-list="doing"]');
		await page.getByRole("button", { name: "篩選與排序" }).click();
		const picker = page.getByRole("dialog", { name: "篩選與排序" });
		await picker.getByRole("button", { name: "排序方式", exact: true }).click();
		await picker.getByRole("menu", { name: "排序方式選項" }).getByRole("menuitemcheckbox", { name: "Due date：遠到近" }).click();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(doing.getByRole("heading", { level: 3 }).first()).toHaveText("[場務組] 盤點會場網路設備");

		await page.getByRole("button", { name: /篩選與排序/ }).click();
		const people = picker.getByRole("region", { name: "負責人" });
		await people.getByRole("checkbox", { name: "全選行政組" }).click();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("1 / 7 張卡片");
		await expect(page.getByRole("heading", { name: "[行政組] 整理志工行前通知" })).toBeVisible();
	});

	test("shared filters and sorting survive reload", async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/?q=Priority&team=development&member=114&label=Backend&sort=due-desc");

		await expect(page.getByRole("combobox", { name: "搜尋組別" })).toHaveValue("開發組");
		await expect(page.getByRole("button", { name: "排序方式" })).toHaveAttribute("data-value", "due-desc");
		await expect(page.getByRole("combobox", { name: "搜尋 Label" })).toHaveValue("Labels 1");
		await expect(page.getByRole("combobox", { name: "搜尋卡片" })).toHaveValue("Priority");
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
		await page.reload();
		await expect(page.getByRole("combobox", { name: "搜尋組別" })).toHaveValue("開發組");
		await expect(page.getByRole("button", { name: "排序方式" })).toHaveAttribute("data-value", "due-desc");
		await expect(page.getByRole("combobox", { name: "搜尋卡片" })).toHaveValue("Priority");
		await expect(page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" })).toBeVisible();
	});

	test("card search waits for typing to settle and supports the slash shortcut", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name === "mobile", "slash focus is a hardware-keyboard desktop shortcut");
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");
		const search = page.getByRole("combobox", { name: "搜尋卡片" });
		await expect(search).toBeVisible();
		await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
		await page.keyboard.press("/");
		await expect(search).toBeFocused();
		await search.fill("開發 Yorukot Priority");
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("7 / 7 張卡片");
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("1 / 7 張卡片", { timeout: 1_000 });
		await expect.poll(() => new URL(page.url()).searchParams.get("q")).toBe("開發 Yorukot Priority");
		await search.press("Escape");
		await expect(search).toHaveValue("");
		await expect(search).not.toBeFocused();
	});

	test("token search inserts and restores filter tokens", async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/");

		const search = page.getByRole("combobox", { name: "搜尋卡片" });
		await search.click();
		await page.getByRole("option", { name: /^Label/ }).click();
		await search.fill("Backend");
		await page.getByRole("option", { name: "Backend" }).click();

		await expect(page.getByRole("button", { name: "移除 Label 篩選 Backend" })).toBeVisible();
		await expect(page.getByRole("combobox", { name: "搜尋 Label" })).toHaveValue("Labels 1");
		await expect(page.getByRole("region", { name: "篩選看板" }).getByRole("status")).toHaveText("1 / 7 張卡片");
		await page.reload();
		await expect(page.getByRole("button", { name: "移除 Label 篩選 Backend" })).toBeVisible();
	});

	test("token chips stay contained at narrow width", async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 720 });
		await page.goto("/?team=development&member=114&label=Backend");

		await expect(page.getByRole("button", { name: "移除 Label 篩選 Backend" })).toBeVisible();
		const layout = await page.evaluate(() => {
			const filters = document.querySelector<HTMLElement>('section[aria-label="篩選看板"]');
			const rect = filters?.getBoundingClientRect();
			const children = filters
				? [...filters.querySelectorAll<HTMLElement>(":scope > label, :scope > button, :scope > div, :scope > span")].filter(
						(item) => item.offsetWidth > 2 && item.offsetHeight > 2
					)
				: [];
			return {
				viewport: window.innerWidth,
				documentWidth: document.documentElement.scrollWidth,
				contained: Boolean(
					rect &&
					children.every((item) => {
						const box = item.getBoundingClientRect();
						return box.left >= rect.left - 1 && box.right <= rect.right + 1;
					})
				)
			};
		});
		expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport);
		expect(layout.contained).toBe(true);
	});

	test("drag handle reorders within a lane and positions across lanes", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name === "mobile", "Playwright mobile emulation does not expose a stable touch-drag gesture");
		const renderErrors: string[] = [];
		page.on("pageerror", (error) => renderErrors.push(error.message));
		page.on("console", (message) => {
			if (message.type() === "error" && /Maximum update depth exceeded|sitcon_board_render_failed/.test(message.text())) {
				renderErrors.push(message.text());
			}
		});
		await page.goto("/");
		const sortControl = page.getByRole("button", { name: "排序方式" });
		await expect(sortControl).toHaveAttribute("data-value", "due-asc");
		const drag = async (handle: ReturnType<typeof page.getByRole>, target: ReturnType<typeof page.locator>, beforeDrop?: () => Promise<void>) => {
			const sourceBox = await handle.boundingBox();
			if (!sourceBox) throw new Error("drag source is not visible");
			const source = { x: sourceBox.x + sourceBox.width / 2, y: sourceBox.y + sourceBox.height / 2 };
			await page.mouse.move(source.x, source.y);
			await page.mouse.down();
			await page.mouse.move(source.x, source.y + 12, { steps: 3 });
			await expect(page.locator('[class*="dragPreview"]')).toBeVisible();
			const targetBox = await target.boundingBox();
			if (!targetBox) throw new Error("drag target is not visible");
			const destination = { x: targetBox.x + targetBox.width / 2, y: targetBox.y + targetBox.height / 2 };
			await page.mouse.move(destination.x, destination.y, { steps: 18 });
			await page.evaluate(
				() =>
					new Promise<void>((resolve) => {
						requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
					})
			);
			await beforeDrop?.();
			await page.mouse.up();
			await expect(page.locator('[class*="dragPreview"]')).toBeHidden();
			await page.evaluate(
				() =>
					new Promise<void>((resolve) => {
						requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
					})
			);
		};
		const designTitle = "[設計組] 製作工作人員識別證";
		const venueTitle = "[場務組] 盤點會場網路設備";
		const designCard = page.getByRole("heading", { name: designTitle }).locator("xpath=ancestor::article");
		const venueCard = page.getByRole("heading", { name: venueTitle }).locator("xpath=ancestor::article");
		const doing = page.locator('section[data-list="doing"]');
		const beforeNonManualDrop = await doing.getByRole("heading", { level: 3 }).allTextContents();
		await drag(page.getByRole("button", { name: `拖曳 ${designTitle}` }), venueCard, async () => {
			await expect(sortControl).toHaveAttribute("data-value", "due-asc");
		});
		await expect(doing.getByRole("heading", { level: 3 })).toHaveText(beforeNonManualDrop);
		await sortControl.click();
		await page.getByRole("menuitemcheckbox", { name: "手動順序" }).click();
		await drag(page.getByRole("button", { name: `拖曳 ${designTitle}` }), venueCard);
		await expect(doing.getByRole("heading", { level: 3 }).last()).toHaveText(designTitle);

		const todoCard = page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }).locator("xpath=ancestor::article");
		await drag(page.getByRole("button", { name: `拖曳 ${venueTitle}` }), todoCard, async () => {
			const todo = page.locator('section[data-list="todo"]');
			await expect(todo.getByRole("heading", { name: venueTitle })).toBeVisible();
			await expect(page.locator('[class*="dragPreview"]')).toBeVisible();
		});
		const todo = page.locator('section[data-list="todo"]');
		await expect(todo.getByRole("heading", { name: venueTitle })).toBeVisible();
		await expect(designCard).toBeVisible();

		const beforeCancel = await doing.getByRole("heading", { level: 3 }).allTextContents();
		const cancelHandle = page.getByRole("button", { name: `拖曳 ${designTitle}` });
		const cancelBox = await cancelHandle.boundingBox();
		if (!cancelBox) throw new Error("cancel drag source is not visible");
		await page.mouse.move(cancelBox.x + cancelBox.width / 2, cancelBox.y + cancelBox.height / 2);
		await page.mouse.down();
		await page.mouse.move(cancelBox.x + cancelBox.width / 2, cancelBox.y + cancelBox.height + 30, { steps: 5 });
		await page.keyboard.press("Escape");
		await page.mouse.up();
		await expect(doing.getByRole("heading", { level: 3 })).toHaveText(beforeCancel);
		await expect(page.locator("main.sb-startup-error")).toHaveCount(0);
		expect(renderErrors).toEqual([]);
	});

	test("dragging near the board edge scrolls horizontally", async ({ page }, testInfo) => {
		test.skip(testInfo.project.name === "mobile", "desktop pointer regression coverage");
		await page.setViewportSize({ width: 600, height: 800 });
		await page.goto("/?sort=manual");
		const board = page.getByRole("region", { name: "SITCON 2027 工作看板" });
		const handle = page.getByRole("button", { name: "拖曳 [議程組] 確認議程講者資料" });
		const handleBox = await handle.boundingBox();
		const boardBox = await board.boundingBox();
		if (!handleBox || !boardBox) throw new Error("board drag controls are not visible");
		await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2);
		await page.mouse.down();
		await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height + 12, { steps: 3 });
		await page.mouse.move(boardBox.x + boardBox.width - 4, handleBox.y + handleBox.height + 12, { steps: 12 });
		await expect.poll(() => board.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
		await page.keyboard.press("Escape");
		await page.mouse.up();
	});

	test("quick create exposes status, description, and Labels inline", async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto("/");

		await page.getByRole("button", { name: "新增卡片" }).click();
		const dialog = page.getByRole("dialog", { name: "新增卡片" });
		await expect(dialog.getByRole("button", { name: "新卡片 Status" })).toHaveText("Inbox");
		await chooseSelectField(page, dialog, "新卡片 Status", "Doing");
		await dialog.getByLabel("新卡片 Description").fill("確認交接與值班時段");
		await dialog.getByLabel("搜尋新卡片 Label").fill("Backend");
		await dialog.getByRole("checkbox", { name: "Backend" }).check();
		await page.screenshot({ path: "../docs/assets/sitcon-board-quick-create-more-mobile.png", fullPage: true });
		await page.getByLabel("卡片標題").fill("新增值班表");
		await page.getByRole("button", { name: "建立卡片" }).click();
		// Close the create dialog so the board is visible and accessible again.
		await page.keyboard.press("Escape");

		const doing = page.locator('section[data-list="doing"]');
		await expect(doing.getByRole("heading", { name: "[開發組] 新增值班表" })).toBeVisible();
		await expect(doing.getByText("確認交接與值班時段")).toBeVisible();
		await doing.getByRole("heading", { name: "[開發組] 新增值班表" }).click();
		await expect(page.getByRole("dialog", { name: "新卡片詳細資料" }).getByText("Backend")).toBeVisible();
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
		await expect(details.getByRole("button", { name: "組別", exact: true })).toHaveText("開發組");
		await expect(details.getByRole("button", { name: "狀態", exact: true })).toHaveText("To do");
		await expect(details.getByLabel("Start")).toHaveValue("2026/07/17");
		await expect(details.getByLabel("Due")).toHaveValue("2026/07/21");
		await expect(details.getByText("Team::開發組")).toBeVisible();
		await expect(details.getByText("Priority::High")).toBeVisible();
		await expect(details.getByLabel("描述預覽")).toBeVisible();
		await details.getByRole("button", { name: "編輯" }).click();
		await details.getByRole("textbox", { name: "描述" }).fill("## 驗收條件\n\n- [ ] 補齊測試\n\n[規格](https://example.com/spec)");
		await details.getByRole("button", { name: "預覽" }).click();
		await expect(details.getByRole("heading", { name: "驗收條件" })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-details.png", fullPage: true });
		await details.getByRole("button", { name: "變更 Assignee" }).click();
		const picker = page.getByRole("dialog", { name: "選擇 Assignee" });
		await picker.getByRole("checkbox", { name: /沈明軒/ }).click();
		await expect(picker.getByText("已選擇 2 人")).toBeVisible();
		await picker.getByRole("button", { name: "完成" }).click();
		await expect(details.getByText("系統活動").first()).toBeVisible();
		// The demo title-change note carries GitLab's inline-diff HTML and must
		// render as text, not markup.
		await expect(details.getByText("[開發組]", { exact: true })).toBeVisible();
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
		await expect(details.getByLabel("Due")).toHaveValue("2026/07/25");
		await details.getByRole("heading", { name: "Labels" }).scrollIntoViewIfNeeded();
		await expect(details.getByLabel("新增 Label")).toBeVisible();
		await details.getByRole("textbox", { name: "Comment" }).scrollIntoViewIfNeeded();
		await expect(details.getByRole("textbox", { name: "Comment" })).toBeInViewport();
		await page.screenshot({ path: "../docs/assets/sitcon-board-tags-comments-mobile.png", fullPage: true });
		await expect(details.getByRole("button", { name: "儲存細節" })).toBeVisible();
		await page.screenshot({ path: "../docs/assets/sitcon-board-details-mobile.png", fullPage: true });
	});

	test("card relationships create, redirect invalid children, and link work items", async ({ page }, testInfo) => {
		const mobile = testInfo.project.name === "mobile";
		await page.addInitScript(() => localStorage.setItem("sitcon-board-theme", "dark"));
		await page.setViewportSize(mobile ? { width: 320, height: 720 } : { width: 1440, height: 900 });
		await page.goto("/");
		await page.getByRole("heading", { name: "[開發組] 修正報名系統寄信流程" }).click();

		const details = page.getByRole("dialog", { name: "#127 卡片詳細資料" });
		await expect(details.getByText("補上退信重試整合測試")).toBeVisible();
		await expect(details.getByText("整理志工行前通知")).toBeVisible();

		await details.getByRole("button", { name: "新增 Child item" }).click();
		await page.getByRole("menuitem", { name: "加入既有 Task" }).click();
		let relationshipDialog = page.getByRole("dialog", { name: "加入既有 Task" });
		await relationshipDialog.getByLabel("搜尋 Task 標題或 #IID").fill("#128");
		await expect(relationshipDialog.getByText(/#128 是 Issue，不能作為 Issue 的 Child item/)).toBeVisible();
		await relationshipDialog.getByRole("button", { name: "改用 Linked item" }).click();
		relationshipDialog = page.getByRole("dialog", { name: "新增 Linked item" });
		await expect(relationshipDialog.getByLabel("搜尋 Issue、Task 或 #IID")).toHaveValue("#128");
		await relationshipDialog.getByRole("button", { name: "取消" }).click();

		await details.getByRole("button", { name: "新增 Child item" }).click();
		await page.getByRole("menuitem", { name: "建立新 Task" }).click();
		relationshipDialog = page.getByRole("dialog", { name: "建立 Child Task" });
		await relationshipDialog.getByLabel("Task 標題").fill("補上關聯回歸測試");
		await relationshipDialog.getByRole("button", { name: "建立 Task" }).click();
		await expect(details.getByText("補上關聯回歸測試")).toBeVisible();

		await details.getByRole("button", { name: "新增 Linked item" }).click();
		relationshipDialog = page.getByRole("dialog", { name: "新增 Linked item" });
		await relationshipDialog.getByLabel("搜尋 Issue、Task 或 #IID").fill("#129");
		await relationshipDialog.getByRole("checkbox", { name: /確認議程講者資料/ }).click();
		await relationshipDialog.getByLabel("搜尋 Issue、Task 或 #IID").fill("#130");
		await relationshipDialog.getByRole("checkbox", { name: /製作工作人員識別證/ }).click();
		await page.screenshot({
			path: docsAsset(`sitcon-board-linked-multiselect${mobile ? "-mobile" : ""}.png`),
			fullPage: false
		});
		await relationshipDialog.getByRole("button", { name: "新增 2 個關聯" }).click();
		await expect(details.getByText("確認議程講者資料")).toBeVisible();
		await expect(details.getByText("製作工作人員識別證")).toBeVisible();
		await details.getByRole("button", { name: "移除 Linked item #130" }).click();
		await page.getByRole("alertdialog", { name: "移除 Linked item？" }).getByRole("button", { name: "確認移除" }).click();
		await expect(details.getByText("製作工作人員識別證")).toBeHidden();

		await details.getByRole("heading", { name: "Linked items" }).scrollIntoViewIfNeeded();
		await page.screenshot({
			path: docsAsset(`sitcon-board-relations${mobile ? "-mobile" : ""}.png`),
			fullPage: false
		});
	});
});
