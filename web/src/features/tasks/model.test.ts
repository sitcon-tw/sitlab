import { describe, expect, it } from "vitest";
import { mapTask, taskToDraft } from "./model";

describe("task adapter", () => {
	it("keeps wire timestamps and nullable ownership outside the view model", () => {
		const task = mapTask({
			id: "task-1",
			workspaceId: "ws-1",
			title: "Review release",
			description: "Check the final notes.",
			status: "in_progress",
			assigneeId: null,
			createdAt: "2026-01-02T03:04:05Z",
			updatedAt: "2026-01-03T03:04:05Z"
		});
		expect(task.createdAt).toBeInstanceOf(Date);
		expect(task.assigneeId).toBeNull();
		expect(taskToDraft(task)).toMatchObject({ title: "Review release", status: "in_progress", assigneeId: null });
	});
});
