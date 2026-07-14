import "@typespec/http";
import "@typespec/openapi";

import "./models/common.tsp";
import "./models/auth.tsp";
import "./models/workspace.tsp";
import "./models/task.tsp";

import "./services/system.tsp";
import "./services/auth.tsp";
import "./services/workspaces.tsp";
import "./services/tasks.tsp";

using Http;
using OpenAPI;

@service(#{ title: "Project Template API" })
@server("/api/v1", "Version 1 API")
@doc("Contract for the Project Template modular monolith.")
@info(#{ version: "0.1.0" })
@tagMetadata("System", #{ description: "Process liveness and dependency readiness." })
@tagMetadata("Auth", #{ description: "Registration, login, logout, CSRF, and current-session identity." })
@tagMetadata("Workspaces", #{ description: "Workspace lifecycle operations." })
@tagMetadata("Workspace Members", #{ description: "Workspace membership and role management." })
@tagMetadata("Tasks", #{ description: "Workspace-scoped task workflow." })
namespace ProjectTemplate;
