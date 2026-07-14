# Backend Architecture

The server is a modular monolith with inward-pointing dependencies:

1. `internal/domain` owns identity, directory, board, durable-operation, and sync models.
2. `internal/controller/application` owns OAuth, directory preferences, bootstrap, board mutation, and sync use cases plus their narrow ports.
3. `internal/controller/infrastructure` implements GitHub directory, GitLab, PostgreSQL, cryptography, and observability adapters.
4. `internal/controller/transport/http` maps the TypeSpec contract, RFC 7807 errors, rolling session cookies, and injected HTML bootstrap.
5. `internal/controller/app` is the only composition root.

The directory source is fixed to `sitcon-tw/sitlab/.sitcon/board-directory.yml` on GitHub, while members and issues come from the fixed GitLab project `sitcon-tw/2027`; clients never provide either source. Production startup performs an initial directory/member/board sync. Existing snapshots may serve during a source outage, but an instance with no snapshots fails readiness.

Unsafe authenticated requests require `X-CSRF-Token` plus an allowed Origin. Session cookies are opaque, HttpOnly, Secure in production, and renewed for 14 days on every valid use. OAuth PKCE verifiers are encrypted at rest, while session and OAuth state tokens are stored only as keyed hashes.

Optimistic mutations and durable operations commit atomically. Operation IDs are idempotency keys. The operation worker applies full current card intent to GitLab and records technical failure detail for logs and retry while preserving stable client error codes.
