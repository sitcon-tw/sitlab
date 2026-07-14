import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace ProjectTemplate;

@tag("Workspaces")
@useAuth(SessionCookieAuth)
@route("/workspaces")
interface WorkspaceService {
	@operationId("listWorkspaces")
	@summary("List workspaces")
	@get
	listWorkspaces(): OkJson<WorkspaceListResponse> | UnauthorizedProblem | InternalServerProblem;

	@operationId("createWorkspace")
	@summary("Create workspace")
	@post
	createWorkspace(...CSRFHeader, @body body: CreateWorkspaceRequest):
		| CreatedJson<WorkspaceResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("getWorkspace")
	@summary("Get workspace")
	@get
	@route("/{workspaceId}")
	getWorkspace(...WorkspaceIdPath):
		| OkJson<WorkspaceResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateWorkspace")
	@summary("Update workspace")
	@patch(#{ implicitOptionality: false })
	@route("/{workspaceId}")
	updateWorkspace(...WorkspaceIdPath, ...CSRFHeader, @body body: UpdateWorkspaceRequest):
		| OkJson<WorkspaceResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("deleteWorkspace")
	@summary("Delete workspace")
	@delete
	@route("/{workspaceId}")
	deleteWorkspace(...WorkspaceIdPath, ...CSRFHeader):
		| NoContent
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;
}

@tag("Workspace Members")
@useAuth(SessionCookieAuth)
@route("/workspaces/{workspaceId}/members")
interface WorkspaceMemberService {
	@operationId("listWorkspaceMembers")
	@summary("List workspace members")
	@get
	listWorkspaceMembers(...WorkspaceIdPath):
		| OkJson<WorkspaceMemberListResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("addWorkspaceMember")
	@summary("Add workspace member")
	@post
	addWorkspaceMember(...WorkspaceIdPath, ...CSRFHeader, @body body: AddWorkspaceMemberRequest):
		| CreatedJson<WorkspaceMemberResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateWorkspaceMember")
	@summary("Update workspace member")
	@patch(#{ implicitOptionality: false })
	@route("/{userId}")
	updateWorkspaceMember(...WorkspaceIdPath, ...UserIdPath, ...CSRFHeader, @body body: UpdateWorkspaceMemberRequest):
		| OkJson<WorkspaceMemberResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("removeWorkspaceMember")
	@summary("Remove workspace member")
	@delete
	@route("/{userId}")
	removeWorkspaceMember(...WorkspaceIdPath, ...UserIdPath, ...CSRFHeader):
		| NoContent
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;
}
