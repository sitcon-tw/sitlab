import "@typespec/http";

using Http;

namespace ProjectTemplate;

@format("uuid")
scalar uuid extends string;

@format("email")
scalar email extends string;

model SessionCookieAuth {
	type: AuthType.apiKey;
	in: ApiKeyLocation.cookie;
	name: "__Host-project_template_session";
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
	| "AUTH_INVALID_CREDENTIALS"
	| "AUTH_INVALID_CSRF"
	| "INSUFFICIENT_ROLE"
	| "EMAIL_ALREADY_EXISTS"
	| "WORKSPACE_NOT_FOUND"
	| "WORKSPACE_MEMBER_NOT_FOUND"
	| "WORKSPACE_MEMBER_ALREADY_EXISTS"
	| "WORKSPACE_LAST_OWNER"
	| "TASK_ASSIGNEE_NOT_MEMBER"
	| "TASK_NOT_FOUND";

alias ProblemFieldErrorCode =
	| "INVALID_VALUE"
	| "INVALID_JSON"
	| "INVALID_LENGTH"
	| "INVALID_FORMAT"
	| "INVALID_ENUM"
	| "NOT_A_WORKSPACE_MEMBER"
	| "REQUIRED"
	| "VALUE_TOO_SHORT"
	| "VALUE_TOO_LONG";

@example(#{ code: "REQUIRED", message: "title is required", location: "body.title" })
model ProblemError {
	code: ProblemFieldErrorCode;
	message: string;
	location?: string;
}

@error
@example(#{
	type: "https://project-template.example/problems/validation-failed",
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

model NoContent {
	...NoContentResponse;
}

model SessionOkJson<T> {
	...OkResponse;

	@header("Set-Cookie")
	setCookie: string;

	...Body<T>;
}

model SessionCreatedJson<T> {
	...CreatedResponse;

	@header("Set-Cookie")
	setCookie: string;

	...Body<T>;
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

model WorkspaceIdPath {
	@path
	workspaceId: uuid;
}

model UserIdPath {
	@path
	userId: uuid;
}

model TaskIdPath {
	@path
	taskId: uuid;
}
