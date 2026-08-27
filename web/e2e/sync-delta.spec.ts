import { expect, test } from "@playwright/test";
import { demoBootstrap } from "../src/test/demoBootstrap";

const initialBootstrap = { ...demoBootstrap, revision: "10" };
const incomingCard = {
	...demoBootstrap.board.cards.find((card) => card.issueIid === 127)!,
	title: "SSE delta updated this card",
	updatedAt: "2026-08-28T12:00:00Z"
};

test("a sync frame updates the board without refetching bootstrap", async ({ page }) => {
	let bootstrapRequests = 0;
	page.on("request", (request) => {
		if (new URL(request.url()).pathname === "/api/v1/bootstrap") bootstrapRequests++;
	});
	await page.route("**/api/v1/bootstrap", (route) => route.fulfill({ json: initialBootstrap }));
	await page.route("**/api/v1/labels", (route) => route.fulfill({ json: { labels: [] } }));
	await page.route("**/api/v1/events/sync?*", (route) =>
		route.fulfill({
			status: 200,
			contentType: "text/event-stream",
			headers: { "Cache-Control": "no-cache" },
			body: `id: 11\nevent: sync\ndata: ${JSON.stringify({
				checkpoint: "11",
				actions: [
					{
						entity: "card",
						syncId: "11",
						entityId: "127",
						operation: "upsert",
						actorGitLabUserId: null,
						occurredAt: "2026-08-28T12:00:00Z",
						card: incomingCard
					}
				],
				hasMore: false
			})}\n\n`
		})
	);

	// loadInitialBootstrap reads this virtual injected payload before considering the
	// network, matching production's server-rendered bootstrap without a real backend.
	await page.addInitScript((bootstrap) => {
		const original = Document.prototype.getElementById;
		Document.prototype.getElementById = function getElementById(id: string) {
			if (id === "__SITCON_BOOTSTRAP__") return { textContent: JSON.stringify(bootstrap) } as HTMLElement;
			return original.call(this, id);
		};
	}, initialBootstrap);

	await page.goto("/");
	await expect(page.getByRole("heading", { name: "[開發組] SSE delta updated this card" })).toBeVisible();
	await expect.poll(() => bootstrapRequests).toBe(0);
});
