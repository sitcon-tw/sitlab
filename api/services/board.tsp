import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace SitconBoard;

@tag("Bootstrap")
@useAuth(SessionCookieAuth)
interface BootstrapService {
	@operationId("getBootstrap")
	@summary("Get all state required for the first Board render")
	@get
	@route("/bootstrap")
	getBootstrap(): OkJson<BootstrapResponse> | UnauthorizedProblem | ServiceUnavailableProblem | InternalServerProblem;
}

@tag("Labels")
@useAuth(SessionCookieAuth)
@route("/labels")
interface LabelService {
	@operationId("listProjectLabels")
	@summary("List labels available in the GitLab project")
	@get
	listProjectLabels(): OkJson<ProjectLabelsResponse> | UnauthorizedProblem | ServiceUnavailableProblem | InternalServerProblem;

	@operationId("createProjectLabel")
	@summary("Create a GitLab project label as the current user")
	@post
	createProjectLabel(...CSRFHeader, @body body: CreateProjectLabelRequest):
		| CreatedJson<ProjectLabel>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("updateProjectLabel")
	@summary("Rename or restyle a GitLab project label as the current user")
	@put
	@route("/{labelId}")
	updateProjectLabel(...ProjectLabelPath, ...CSRFHeader, @body body: UpdateProjectLabelRequest):
		| OkJson<ProjectLabel>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("deleteProjectLabel")
	@summary("Delete a GitLab project label as the current user")
	@delete
	@route("/{labelId}")
	deleteProjectLabel(...ProjectLabelPath, ...CSRFHeader):
		| NoContent
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;
}

@tag("Cards")
@useAuth(SessionCookieAuth)
@route("/cards")
interface CardService {
	@operationId("createCard")
	@summary("Optimistically create a card")
	@post
	createCard(...CSRFHeader, @body body: CreateCardRequest):
		| AcceptedJson<CardMutationResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ForbiddenProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("updateCardDetails")
	@summary("Optimistically update card details")
	@put
	@route("/{issueIid}/details")
	updateCardDetails(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardDetailsRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateCardTeam")
	@summary("Optimistically change a card team")
	@put
	@route("/{issueIid}/team")
	updateCardTeam(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardTeamRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateCardAssignee")
	@summary("Optimistically change a card assignee")
	@put
	@route("/{issueIid}/assignee")
	updateCardAssignee(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardAssigneeRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateCardStartDate")
	@summary("Optimistically change a card start date")
	@put
	@route("/{issueIid}/start-date")
	updateCardStartDate(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardStartDateRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateCardDueDate")
	@summary("Optimistically change a card due date")
	@put
	@route("/{issueIid}/due-date")
	updateCardDueDate(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardDueDateRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("updateCardLabels")
	@summary("Optimistically update card labels")
	@put
	@route("/{issueIid}/labels")
	updateCardLabels(...IssueIidPath, ...CSRFHeader, @body body: UpdateCardLabelsRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("listCardComments")
	@summary("List GitLab comments and system notes for a card")
	@get
	@route("/{issueIid}/comments")
	listCardComments(...IssueIidPath):
		| OkJson<CardCommentsResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("createCardComment")
	@summary("Add a GitLab comment as the current user")
	@post
	@route("/{issueIid}/comments")
	createCardComment(...IssueIidPath, ...CSRFHeader, @body body: CreateCardCommentRequest):
		| CreatedJson<CardComment>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("moveCard")
	@summary("Optimistically move or close a card")
	@put
	@route("/{issueIid}/position")
	moveCard(...IssueIidPath, ...CSRFHeader, @body body: MoveCardRequest):
		| AcceptedJson<CardMutationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;
}

@tag("Sync")
@useAuth(SessionCookieAuth)
interface SyncService {
	@operationId("retryOperation")
	@summary("Retry a failed durable operation")
	@post
	@route("/operations/{operationId}/retry")
	retryOperation(...OperationIdPath, ...CSRFHeader):
		| AcceptedJson<RetryOperationResponse>
		| UnauthorizedProblem
		| ForbiddenProblem
		| NotFoundProblem
		| ConflictProblem
		| InternalServerProblem;

	@operationId("refreshSnapshots")
	@summary("Request a background GitLab refresh")
	@post
	@route("/sync/refresh")
	refreshSnapshots(...CSRFHeader): AcceptedJson<RefreshSyncResponse> | UnauthorizedProblem | ForbiddenProblem | ConflictProblem | InternalServerProblem;

	@operationId("getSyncDelta")
	@summary("Replay board changes since a checkpoint")
	@get
	@route("/sync")
	getSyncDelta(@query since: string, @query limit?: int32):
		| OkJson<SyncDeltaResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ConflictProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;
}
