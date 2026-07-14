import { describe, expect, it } from "vitest";
import { canManageMembers, canManageTasks, mapWorkspace } from "./model";

describe("workspace policy", () => {
	it("keeps task writes available to owners and editors only", () => {
		expect(canManageTasks("owner")).toBe(true);
		expect(canManageTasks("editor")).toBe(true);
		expect(canManageTasks("viewer")).toBe(false);
	});

	it("reserves member management for owners", () => {
		expect(canManageMembers("owner")).toBe(true);
		expect(canManageMembers("editor")).toBe(false);
	});

	it("maps API dates at the feature boundary", () => {
		const workspace = mapWorkspace({
			id: "ws-1",
			name: "Operations",
			role: "viewer",
			createdByUserId: "user-1",
			createdAt: "2026-01-02T03:04:05Z",
			updatedAt: "2026-01-02T03:04:05Z"
		});
		expect(workspace.createdAt).toBeInstanceOf(Date);
		expect(workspace.role).toBe("viewer");
	});
});
