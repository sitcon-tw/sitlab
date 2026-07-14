import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace SitconBoard;

@example(#{ name: "SITCON Board API", version: "0.1.0" })
model APIStatusResponse {
	name: "SITCON Board API";
	version: string;
}

@example(#{ status: "ok" })
model HealthResponse {
	status: "ok";
}

@tag("System")
interface SystemService {
	@operationId("getAPIStatus")
	@summary("Get API status")
	@get
	@route("/")
	getAPIStatus(): OkJson<APIStatusResponse>;

	@operationId("getLiveness")
	@summary("Get process liveness")
	@get
	@route("/health/live")
	getLiveness(): OkJson<HealthResponse>;

	@operationId("getReadiness")
	@summary("Check database and snapshot readiness")
	@get
	@route("/health/ready")
	getReadiness(): OkJson<HealthResponse> | ServiceUnavailableProblem;
}
