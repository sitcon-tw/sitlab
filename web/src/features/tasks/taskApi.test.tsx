import { queryKeys } from "@/shared/api/queryKeys";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Task, TaskDraft } from "./model";
import { useUpdateTaskMutation } from "./taskApi";

const mocks = vi.hoisted(() => ({ patch: vi.fn(), getCsrfToken: vi.fn(async () => "csrf-token") }));

vi.mock("@/shared/api/client", () => ({
	api: { PATCH: mocks.patch },
	getCsrfToken: mocks.getCsrfToken,
	expectData: (result: { data: unknown }) => result.data
}));

const currentTask: Task = {
	id: "task-1",
	workspaceId: "workspace-1",
	title: "Original title",
	description: "Original description",
	status: "todo",
	assigneeId: null,
	createdAt: new Date("2026-07-10T08:00:00Z"),
	updatedAt: new Date("2026-07-10T08:00:00Z")
};

const changedDraft: TaskDraft = {
	title: "Updated title",
	description: "Updated description",
	status: "in_progress",
	assigneeId: null
};

function setup() {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
	const key = queryKeys.workspaces.task("workspace-1", "task-1");
	client.setQueryData(key, currentTask);
	const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
	const hook = renderHook(() => useUpdateTaskMutation("workspace-1", "task-1"), { wrapper });
	return { client, key, hook };
}

describe("task update cache behavior", () => {
	beforeEach(() => vi.clearAllMocks());

	it("publishes an optimistic task then reconciles with the server", async () => {
		let resolveRequest!: (result: unknown) => void;
		mocks.patch.mockReturnValue(
			new Promise((resolve) => {
				resolveRequest = resolve;
			})
		);
		const { client, key, hook } = setup();
		let request!: Promise<Task>;
		act(() => {
			request = hook.result.current.mutateAsync(changedDraft);
		});
		await waitFor(() => expect(client.getQueryData<Task>(key)?.title).toBe("Updated title"));
		resolveRequest({
			data: {
				task: {
					id: "task-1",
					workspaceId: "workspace-1",
					title: "Canonical title",
					description: "Updated description",
					status: "in_progress",
					assigneeId: null,
					createdAt: "2026-07-10T08:00:00Z",
					updatedAt: "2026-07-10T09:00:00Z"
				}
			}
		});
		await act(async () => {
			await request;
		});
		expect(client.getQueryData<Task>(key)?.title).toBe("Canonical title");
	});

	it("rolls back the optimistic task when the request fails", async () => {
		mocks.patch.mockRejectedValue(new Error("network unavailable"));
		const { client, key, hook } = setup();
		await act(async () => {
			await expect(hook.result.current.mutateAsync(changedDraft)).rejects.toThrow("network unavailable");
		});
		expect(client.getQueryData<Task>(key)).toEqual(currentTask);
	});
});
