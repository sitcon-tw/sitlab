import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace ProjectTemplate;

@tag("Auth")
@route("/auth")
interface AuthService {
	@operationId("registerUser")
	@summary("Register user")
	@post
	@route("/register")
	registerUser(@body body: RegisterRequest):
		| SessionCreatedJson<AuthResponse>
		| BadRequestProblem
		| ConflictProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("loginUser")
	@summary("Log in user")
	@post
	@route("/login")
	loginUser(@body body: LoginRequest):
		| SessionOkJson<AuthResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;

	@operationId("logoutUser")
	@summary("Log out user")
	@useAuth(SessionCookieAuth)
	@post
	@route("/logout")
	logoutUser(...CSRFHeader): SessionNoContent | UnauthorizedProblem | ForbiddenProblem | InternalServerProblem;

	@operationId("getCSRFToken")
	@summary("Get CSRF token")
	@useAuth(SessionCookieAuth)
	@get
	@route("/csrf")
	getCSRFToken(): OkJson<CSRFResponse> | UnauthorizedProblem | InternalServerProblem;

	@operationId("getCurrentUser")
	@summary("Get current user")
	@useAuth(SessionCookieAuth)
	@get
	@route("/me")
	getCurrentUser(): OkJson<AuthResponse> | UnauthorizedProblem | InternalServerProblem;
}
