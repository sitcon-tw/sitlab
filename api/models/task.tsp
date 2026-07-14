import "@typespec/http";

using Http;

namespace ProjectTemplate;

alias TaskStatus = "todo" | "in_progress" | "done";

@example(#{
	id: "33333333-3333-3333-3333-333333333333",
	workspaceId: "22222222-2222-2222-2222-222222222222",
	title: "Publish architecture decision record",
	description: "Document transaction ownership for workspace creation.",
	status: "in_progress",
	assigneeId: "11111111-1111-1111-1111-111111111111",
	createdAt: utcDateTime.fromISO("2026-07-10T09:00:00Z"),
	updatedAt: utcDateTime.fromISO("2026-07-10T09:30:00Z"),
})
model Task {
	id: uuid;
	workspaceId: uuid;

	@minLength(1)
	@maxLength(160)
	title: string;

	@maxLength(4000)
	description: string;

	status: TaskStatus;
	assigneeId: uuid | null;
	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

@example(#{
	title: "Publish architecture decision record",
	description: "Document transaction ownership for workspace creation.",
	assigneeId: "11111111-1111-1111-1111-111111111111",
})
model CreateTaskRequest {
	@minLength(1)
	@maxLength(160)
	title: string;

	@maxLength(4000)
	description?: string;

	status?: TaskStatus;
	assigneeId?: uuid | null;
}

@example(#{ title: "Publish the architecture decision record", status: "done", assigneeId: "11111111-1111-1111-1111-111111111111" })
model UpdateTaskRequest {
	@minLength(1)
	@maxLength(160)
	title?: string;

	@maxLength(4000)
	description?: string;

	status?: TaskStatus;
	assigneeId?: uuid | null;
}

model TaskResponse {
	task: Task;
}

model TaskListResponse {
	tasks: Task[];
}

model TaskListQuery {
	@query
	status?: TaskStatus;
}
