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
}
