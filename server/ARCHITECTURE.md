# Backend architecture

This server is a modular monolith with inward-pointing dependencies:

1. `internal/domain` owns models, invariants, and authorization policy. It imports no project packages.
2. `internal/controller/application` owns use cases and the narrow ports each use case consumes.
3. `internal/controller/infrastructure` implements those ports with PostgreSQL, sqlc, pgx, and security adapters.
4. `internal/controller/transport/http` maps HTTP requests and RFC 7807 responses. It does not query the database.
5. `internal/controller/app` is the only composition root and the only package that knows all concrete adapters.

Mutating use cases validate, authorize, persist, and trace a complete action. Workspace creation and registration
use the injected transaction port so their related records are atomic. Technical errors retain their causes and are
recorded on spans; expected validation, authorization, not-found, and conflict errors use application error kinds.

The API contract is rooted at `/api/v1`. Browser authentication uses opaque server-side sessions in HttpOnly cookies.
Unsafe authenticated requests require a synchronizer token in `X-CSRF-Token` and a matching allowed Origin.
