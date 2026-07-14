# API Contract Guide

`api/` is the only source of truth for the public HTTP contract. TypeSpec emits OpenAPI 3.1, which is then copied verbatim to the docs and backend and used to generate the web TypeScript declarations.

## Rules

- Keep JSON fields camelCase and operation IDs stable.
- Add status-specific RFC 7807 response variants instead of a generic error response.
- Public models need realistic examples. Never include secrets that look usable.
- Use `SessionCookieAuth` for authenticated endpoints and `CSRFHeader` for authenticated mutations.
- Do not hand-edit any generated OpenAPI or TypeScript artifact.
- Do not add backend implementation details, database types, or frontend view models here.

## Validation

- `pnpm --filter @project-template/api check`
- `pnpm generate`
- `pnpm generated:check`

Any contract change must update all three generated artifacts in the same commit.
