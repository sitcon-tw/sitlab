# Repository Guide

## Orientation

Project Template is a pnpm workspace around a Go modular monolith, a TypeSpec contract, a React client, a shared UI package, and an Astro documentation site. Root guidance covers cross-project work only. Read the closest area guide before editing:

- Backend, database, migrations, configuration, or observability: `server/AGENTS.md`
- TypeSpec or public API shape: `api/AGENTS.md`
- React routes, feature state, or browser behavior: `web/AGENTS.md`
- Shared primitives and tokens: `packages/ui/AGENTS.md`
- Documentation and API explorer: `docs/AGENTS.md`
- Visual decisions: `design.md`
- Dependency boundaries and use-case rules: `ARCHITECTURE.md`

There is no agent runtime in this template. Do not add probe, assignment, remote-agent, installer, or control-plane concepts unless the product itself explicitly requires them.

## Sources Of Truth

- `api/**/*.tsp` owns the HTTP wire contract.
- `server/db/migrations` owns the database schema.
- `packages/ui/src/styles/tokens.css` owns browser design tokens.
- `Justfile` owns cross-language task names.
- `ARCHITECTURE.md` and architecture tests own dependency direction.

Generated OpenAPI and TypeScript files are committed review artifacts. Never hand-edit them. Run `pnpm generate`, then `pnpm generated:check`.

## Working Rules

- Keep changes inside the feature or layer that owns the behavior.
- Prefer a narrow consumer-owned interface over a shared service abstraction.
- Do not import infrastructure into domain or application code.
- Do not duplicate API DTOs by hand in the web app.
- Keep business errors stable and technical causes available to structured logs and traces.
- Add abstractions only after the reference slice proves they reduce real duplication or coupling.
- Update the relevant guide whenever a command, boundary, config key, or ownership rule changes.

## Commands

- `pnpm install`: install workspace dependencies.
- `just dev`: run the backend; use `just web-dev` in a second terminal.
- `just build`: build API, backend, UI, web, Storybook, and docs.
- `just test`: run Go, web, and UI tests.
- `just lint`: run TypeSpec, Go, web, and frontend policy checks.
- `just typecheck`: check TypeSpec, web, UI, and docs.
- `just ci`: run the deterministic local CI aggregate.

## Change Quality

Tests should match risk. Domain policy needs table-driven unit tests; repositories need real PostgreSQL integration tests; protocol mapping needs response-shape tests; interactive UI needs behavior, keyboard, and focus coverage. Visible changes require screenshots at desktop and mobile widths.

Commit subjects use `area: imperative summary`, for example `server/auth: reject expired sessions` or `api/tasks: add assignee filter`.
