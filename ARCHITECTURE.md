# Project Template Architecture

## Purpose

This repository is a production baseline for SaaS and internal tools. Its reference slice supports session-authenticated users, workspaces with owner/editor/viewer roles, and workspace-scoped tasks. The slice exists to prove the boundaries; it is not sample code to bypass when adding the next feature.

## Backend Dependency Direction

```text
transport/http -----> application -----> domain
                           ^                ^
                           |                |
infrastructure -----------+----------------+

composition root --------> concrete implementations
```

`domain` contains entities, value objects, policies, and business errors. It imports only the standard library.

`application` contains use cases and the narrow ports each use case consumes. It may depend on domain. It does not know Chi, pgx, sqlc, HTTP DTOs, environment loading, or global loggers.

`infrastructure` implements application ports. PostgreSQL adapters translate sqlc records and constraint failures into domain values and application errors. External integration adapters terminate vendor-specific concerns here.

`transport/http` decodes protocol input, reads authenticated context, invokes one application use case, and maps the result to the TypeSpec contract. It contains no authorization policy or SQL.

`app` is the only composition root. It constructs concrete adapters, decorates them with observability, and wires handlers. Transport must not import infrastructure directly.

Architecture tests reject imports that violate these rules. Empty layer directories and interfaces with one speculative implementation do not count as architecture.

## Use Cases, Transactions, And Ports

Commands represent behavior that changes state. A command owns validation, authorization, transaction scope, stable errors, and telemetry. Queries can remain direct when they only coordinate reads; do not create empty command/query files for symmetry.

Ports are consumer-owned and deliberately narrow. A workspace creation use case can request `CreateWorkspace` and `CreateOwnerMembership` inside a transaction without gaining access to every repository method.

The transaction boundary belongs to application code. Creating a workspace and its owner membership commits or rolls back atomically. Production constructors require a transaction implementation; tests use an explicitly named fake or no-op only when atomicity is irrelevant to that test.

## Errors

Business failures use stable sentinel or typed errors such as not found, forbidden, conflict, and last owner. HTTP maps those errors to the status and `ProblemDetails.code` defined by TypeSpec.

Technical failures wrap their cause. They are recorded once at an ownership boundary with request ID, trace context, operation, and safe identifiers. Clients receive `INTERNAL_ERROR`, never SQL, stack, credential, or raw dependency details.

## Contract Flow

```text
TypeSpec
  -> OpenAPI 3.1 in docs/public/openapi.json
  -> identical backend embedded OpenAPI
  -> openapi-typescript declarations for the web client
```

TypeSpec is the only public wire source. The backend may define private transport structs for decoding but they must conform to the generated contract. The frontend consumes generated declarations through a typed client and feature adapters. `pnpm generated:check` emits into a temporary directory and compares all tracked outputs without modifying the worktree.

## Frontend Ownership

Routes are lazy composition points. `web/src/features/<feature>` owns feature workflows, server adapters, and view-specific models. `web/src/shared` contains domain-neutral browser infrastructure. `packages/ui` contains cross-surface interaction primitives, tokens, and Storybook stories; it must not mention workspaces or tasks.

State ownership is explicit:

- TanStack Query owns server state and cache reconciliation.
- The URL owns shareable navigation, search, sorting, and filters.
- Context owns session, theme, and current workspace selection.
- A component or feature reducer owns unsaved interaction state.

Do not mirror query data into context or local state. Query keys are factories rooted by feature and workspace so invalidation remains predictable.

## Security

Authentication uses random opaque session tokens in HttpOnly cookies. The database stores only a keyed hash. Production cookies are Secure and SameSite, with an explicit lifetime. Authenticated mutations require a synchronizer token through `X-CSRF-Token`; origin validation is defense in depth.

Authorization is application policy, not a hidden repository filter. Owners manage workspaces and members, editors create and mutate tasks, and viewers read. Removing or demoting the last owner is a conflict.

Secrets are read from environment or a deployment secret store and are never logged. Database credentials use a least-privilege application role in production.

## Observability

Requests carry a request ID and trace context. Logs are structured and include service name, version, operation, duration, outcome, and safe identifiers. Prometheus scrapes application metrics directly; traces use an OpenTelemetry OTLP adapter. Trace export is optional: the process must start and remain correct when no collector exists.

## Evolution Checklist

Before merging a new feature:

1. Define or change the TypeSpec contract and regenerate artifacts.
2. Add domain behavior and policy tests.
3. Add an application use case with narrow ports and an explicit transaction decision.
4. Implement adapters and PostgreSQL integration tests.
5. Map HTTP behavior and problem responses.
6. Add the web feature with complete loading, empty, error, forbidden, and success states.
7. Promote only proven generic primitives into `packages/ui` and add stories.
8. Update docs, architecture decisions, configuration, and deployment in the same change.
