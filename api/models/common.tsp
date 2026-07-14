import "@typespec/http";

using Http;

namespace SitconBoard;

@format("uuid")
scalar uuid extends string;

model SessionCookieAuth {
	type: AuthType.apiKey;
	in: ApiKeyLocation.cookie;
	name: "__Host-sitcon_board_session";
}

alias ErrorCode =
	| "BAD_REQUEST"
	| "UNAUTHORIZED"
	| "FORBIDDEN"
	| "NOT_FOUND"
	| "CONFLICT"
	| "VALIDATION_FAILED"
	| "INTERNAL_ERROR"
	| "SERVICE_UNAVAILABLE"
	| "METHOD_NOT_ALLOWED"
	| "AUTH_MISSING_SESSION"
	| "AUTH_INVALID_SESSION"
	| "AUTH_INVALID_CSRF"
	| "AUTH_OAUTH_FAILED"
	| "DIRECTORY_NOT_READY"
	| "TEAM_NOT_FOUND"
	| "MEMBER_NOT_ASSIGNABLE"
	| "CARD_NOT_FOUND"
	| "OPERATION_NOT_FOUND"
	| "OPERATION_CONFLICT"
	| "SNAPSHOT_NOT_READY"
	| "GITLAB_UNAVAILABLE";

alias ProblemFieldErrorCode =
	| "INVALID_VALUE"
	| "INVALID_JSON"
	| "INVALID_LENGTH"
	| "INVALID_FORMAT"
	| "INVALID_ENUM"
	| "REQUIRED"
	| "VALUE_TOO_SHORT"
	| "VALUE_TOO_LONG"
	| "UNKNOWN_TEAM"
	| "UNKNOWN_MEMBER";

@example(#{ code: "REQUIRED", message: "title is required", location: "body.title" })
model ProblemError {
	code: ProblemFieldErrorCode;
	message: string;
	location?: string;
}

@error
@example(#{
	type: "https://board.sitcon.org/problems/validation-failed",
	title: "Validation failed",
	status: 422,
	code: "VALIDATION_FAILED",
	detail: "One or more request fields are invalid.",
	requestId: "req_01J00000000000000000000000",
	errors: #[#{ code: "REQUIRED", message: "title is required", location: "body.title" }],
})
model ProblemDetails {
	@format("uri")
	type: string;

	title: string;
	status: int32;
	code: ErrorCode;
	detail?: string;
	requestId?: string;
	errors?: ProblemError[];
}

model ProblemJson {
	@header
	contentType: "application/problem+json";

	...Body<ProblemDetails>;
}

model OkJson<T> {
	...OkResponse;
	...Body<T>;
}

model CreatedJson<T> {
	...CreatedResponse;
	...Body<T>;
}

model AcceptedJson<T> {
	...Response<202>;
	...Body<T>;
}

model NoContent {
	...NoContentResponse;
}

model SessionNoContent {
	...NoContentResponse;

	@header("Set-Cookie")
	setCookie: string;
}

model BadRequestProblem {
	...BadRequestResponse;
	...ProblemJson;
}

model UnauthorizedProblem {
	...UnauthorizedResponse;
	...ProblemJson;
}

model ForbiddenProblem {
	...ForbiddenResponse;
	...ProblemJson;
}

model NotFoundProblem {
	...NotFoundResponse;
	...ProblemJson;
}

model ConflictProblem {
	...ConflictResponse;
	...ProblemJson;
}

model UnprocessableEntityProblem {
	...Response<422>;
	...ProblemJson;
}

model InternalServerProblem {
	...Response<500>;
	...ProblemJson;
}

model ServiceUnavailableProblem {
	...Response<503>;
	...ProblemJson;
}

model CSRFHeader {
	@header("X-CSRF-Token")
	@minLength(1)
	csrfToken: string;
}

model IssueIidPath {
	@path
	issueIid: int64;
}

model OperationIdPath {
	@path
	operationId: uuid;
}
