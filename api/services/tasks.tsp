import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace ProjectTemplate;

@tag("Tasks")
@useAuth(SessionCookieAuth)
@route("/workspaces/{workspaceId}/tasks")
interface TaskService {
	@operationId("listTasks")
	@summary("List tasks")
	@get
	listTasks(...WorkspaceIdPath, ...TaskListQuery):
		| OkJson<TaskListResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("createTask")
	@summary("Create task")
	@post
	createTask(...WorkspaceIdPath, ...CSRFHeader, @body body: CreateTaskRequest):
		| CreatedJson<TaskResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("getTask")
	@summary("Get task")
	@get
	@route("/{taskId}")
	getTask(...WorkspaceIdPath, ...TaskIdPath):
		| OkJson<TaskResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateTask")
	@summary("Update task")
	@patch(#{ implicitOptionality: false })
	@route("/{taskId}")
	updateTask(...WorkspaceIdPath, ...TaskIdPath, ...CSRFHeader, @body body: UpdateTaskRequest):
		| OkJson<TaskResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("deleteTask")
	@summary("Delete task")
	@delete
	@route("/{taskId}")
	deleteTask(...WorkspaceIdPath, ...TaskIdPath, ...CSRFHeader):
		| NoContent
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;
}
