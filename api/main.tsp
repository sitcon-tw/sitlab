import "@typespec/http";
import "@typespec/openapi";

import "./models/common.tsp";
import "./models/auth.tsp";
import "./models/directory.tsp";
import "./models/board.tsp";

import "./services/system.tsp";
import "./services/auth.tsp";
import "./services/directory.tsp";
import "./services/board.tsp";
import "./services/realtime.tsp";

using Http;
using OpenAPI;

@service(#{ title: "SITCON Board API" })
@server("/api/v1", "Version 1 API")
@doc("GitLab-backed board for the SITCON 2027 preparation team.")
@info(#{ version: "0.1.0" })
@tagMetadata("System", #{ description: "Process liveness and dependency readiness." })
@tagMetadata("Auth", #{ description: "GitLab OAuth and current browser session." })
@tagMetadata("Bootstrap", #{ description: "Complete initial state for a loading-free first render." })
@tagMetadata("Directory", #{ description: "Normalized SITCON teams and GitLab project members." })
@tagMetadata("Cards", #{ description: "Optimistic GitLab issue board mutations." })
@tagMetadata("Sync", #{ description: "Durable operations and background snapshot refresh." })
@tagMetadata("Realtime", #{ description: "Signed GitLab webhooks and authenticated snapshot events." })
namespace SitconBoard;
