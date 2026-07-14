import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace SitconBoard;

@tag("Auth")
@route("/auth")
interface AuthService {
	@operationId("startGitLabOAuth")
	@summary("Start GitLab OAuth")
	@get
	@route("/gitlab")
	startGitLabOAuth(): OAuthRedirect | InternalServerProblem;

	@operationId("completeGitLabOAuth")
	@summary("Complete GitLab OAuth")
	@get
	@route("/gitlab/callback")
	completeGitLabOAuth(...GitLabOAuthCallbackQuery): OAuthRedirect | BadRequestProblem | UnauthorizedProblem | ServiceUnavailableProblem | InternalServerProblem;

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
