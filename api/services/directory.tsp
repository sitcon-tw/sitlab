import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace SitconBoard;

@tag("Directory")
@useAuth(SessionCookieAuth)
interface DirectoryService {
	@operationId("getDirectory")
	@summary("Get the normalized SITCON team directory")
	@get
	@route("/directory")
	getDirectory(): OkJson<DirectoryResponse> | UnauthorizedProblem | ServiceUnavailableProblem | InternalServerProblem;

	@operationId("updateMyPreferences")
	@summary("Confirm the current user's primary team")
	@put
	@route("/me/preferences")
	updateMyPreferences(...CSRFHeader, @body body: UpdatePreferencesRequest):
		| OkJson<PreferencesResponse>
		| UnauthorizedProblem
		| NotFoundProblem
		| UnprocessableEntityProblem
		| InternalServerProblem;
}
