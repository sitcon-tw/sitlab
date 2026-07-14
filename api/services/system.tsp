import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace ProjectTemplate;

@example(#{ name: "Project Template API", version: "0.1.0" })
model APIStatusResponse {
	name: "Project Template API";
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

	@operationId("getHealth")
	@summary("Get health status")
	@get
	@route("/healthz")
	getHealth(): OkJson<HealthResponse> | ServiceUnavailableProblem;
}
