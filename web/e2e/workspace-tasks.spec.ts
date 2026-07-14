import { expect, test, type Page } from "@playwright/test";

async function register(page: Page, input: { name: string; email: string; password: string }) {
	await page.goto("/register");
	await page.getByLabel("Name").fill(input.name);
	await page.getByLabel("Email").fill(input.email);
	await page.getByLabel("Password").fill(input.password);
	await page.getByRole("button", { name: "Create account" }).click();
	await expect(page.getByRole("heading", { name: "Workspaces" })).toBeVisible();
}

async function signOut(page: Page) {
	const openNavigation = page.getByRole("button", { name: "Open navigation" });
	if (await openNavigation.isVisible()) {
		await openNavigation.click();
		await expect(page.getByTitle("Close navigation")).toBeVisible();
	}
	await page.getByRole("button", { name: "Sign out" }).click();
	await expect(page).toHaveURL(/\/login$/);
}

async function postWithCsrf(page: Page, path: string, body: unknown) {
	return page.evaluate(
		async ({ requestPath, requestBody }) => {
			const csrfResponse = await fetch("/api/v1/auth/csrf", { headers: { Accept: "application/json" } });
			if (!csrfResponse.ok) return { status: csrfResponse.status };
			const { token } = (await csrfResponse.json()) as { token: string };
			const response = await fetch(requestPath, {
				method: "POST",
				headers: { Accept: "application/json, application/problem+json", "Content-Type": "application/json", "X-CSRF-Token": token },
				body: JSON.stringify(requestBody)
			});
			return { status: response.status };
		},
		{ requestPath: path, requestBody: body }
	);
}

test("register, manage workspace tasks, and sign back in", async ({ page }) => {
	const suffix = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
	const email = `e2e-${suffix}@example.test`;
	const password = `Test-password-${suffix}`;
	const workspaceName = `Release ${suffix.slice(-6)}`;
	const taskTitle = `Review release ${suffix.slice(-6)}`;

	await register(page, { name: "Template Test", email, password });
	await page.getByRole("main").getByRole("button", { name: "Create workspace" }).click();
	const workspaceDialog = page.getByRole("dialog", { name: "Create workspace" });
	await workspaceDialog.getByLabel("Workspace name").fill(workspaceName);
	await workspaceDialog.getByRole("button", { name: "Create workspace" }).click();

	await expect(page.getByRole("heading", { name: "Tasks", exact: true })).toBeVisible();
	await page.getByRole("button", { name: "Create task" }).first().click();
	const taskDialog = page.getByRole("dialog", { name: "Create task" });
	await taskDialog.getByLabel("Title").fill(taskTitle);
	await taskDialog.getByLabel("Description").fill("Verify the end-to-end workspace flow.");
	await taskDialog.getByLabel("Status").selectOption("in_progress");
	await taskDialog.getByRole("button", { name: "Create task" }).click();

	await expect(page.getByRole("heading", { name: taskTitle })).toBeVisible();
	await expect(page.getByRole("main").getByText("In progress", { exact: true }).filter({ visible: true }).first()).toBeVisible();
	await signOut(page);

	await page.getByLabel("Email").fill(email);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: "Sign in" }).click();
	await expect(page.getByRole("heading", { name: "Tasks", exact: true })).toBeVisible();
	await expect(page.getByRole("link", { name: taskTitle })).toBeVisible();
});

test("viewer can read tasks but cannot use or bypass task write controls", async ({ page }) => {
	const suffix = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
	const viewerEmail = `viewer-${suffix}@example.test`;
	const ownerEmail = `owner-${suffix}@example.test`;
	const password = `Test-password-${suffix}`;
	const taskTitle = `Viewer policy ${suffix.slice(-6)}`;

	await register(page, { name: "View Only", email: viewerEmail, password });
	await signOut(page);
	await register(page, { name: "Workspace Owner", email: ownerEmail, password });

	await page.getByRole("main").getByRole("button", { name: "Create workspace" }).click();
	const workspaceDialog = page.getByRole("dialog", { name: "Create workspace" });
	await workspaceDialog.getByLabel("Workspace name").fill(`Policy ${suffix.slice(-6)}`);
	await workspaceDialog.getByRole("button", { name: "Create workspace" }).click();
	await expect(page.getByRole("heading", { name: "Tasks", exact: true })).toBeVisible();

	await page.getByRole("button", { name: "Create task" }).first().click();
	const taskDialog = page.getByRole("dialog", { name: "Create task" });
	await taskDialog.getByLabel("Title").fill(taskTitle);
	await taskDialog.getByRole("button", { name: "Create task" }).click();
	await expect(page.getByRole("heading", { name: taskTitle })).toBeVisible();

	const routeMatch = new URL(page.url()).pathname.match(/^\/workspaces\/([^/]+)\/tasks\/([^/]+)$/);
	expect(routeMatch, "task detail route must expose workspace and task IDs").not.toBeNull();
	const workspaceId = decodeURIComponent(routeMatch?.[1] ?? "");
	const taskId = decodeURIComponent(routeMatch?.[2] ?? "");
	const addMember = await postWithCsrf(page, `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/members`, { email: viewerEmail, role: "viewer" });
	expect(addMember.status).toBe(201);

	// The out-of-band setup request rotates CSRF state; reload the SPA before using its cached client again.
	await page.reload();
	await expect(page.getByRole("heading", { name: taskTitle })).toBeVisible();
	await signOut(page);
	await page.getByLabel("Email").fill(viewerEmail);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: "Sign in" }).click();
	await expect(page.getByRole("heading", { name: "Tasks", exact: true })).toBeVisible();
	await page.goto(`/workspaces/${encodeURIComponent(workspaceId)}/tasks/${encodeURIComponent(taskId)}`);

	const main = page.getByRole("main");
	await expect(main.getByRole("heading", { name: taskTitle })).toBeVisible();
	await expect(main.getByText("You have viewer access. Task changes are disabled.")).toBeVisible();
	await expect(main.getByRole("button", { name: "Create task" })).toHaveCount(0);
	await expect(main.getByRole("button", { name: "Edit" })).toHaveCount(0);
	await expect(main.getByRole("button", { name: "Delete" })).toHaveCount(0);

	const forbiddenWrite = await postWithCsrf(page, `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/tasks`, {
		title: "Viewer must not create this task"
	});
	expect(forbiddenWrite.status).toBe(403);
});
