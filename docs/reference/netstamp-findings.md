# Netstamp Design Findings

This document records the source patterns retained by Project Template. It describes engineering structure, not Netstamp product behavior.

## Backend

Netstamp is a Go modular monolith whose meaningful boundary is dependency direction rather than process count. Domain models and policies sit inward of application use cases. Application packages define narrow ports, infrastructure implements them, HTTP handlers translate protocol concerns, and a composition root is the only place that knows concrete implementations.

The strongest patterns retained here are explicit use-case flow, transaction ownership in application code, stable business errors, technical-cause preservation, sqlc mapping inside PostgreSQL adapters, and architecture tests for imports.

## Contract

TypeSpec is the public API source. It emits OpenAPI 3.1 to the documentation site; the same bytes are embedded in the backend and consumed by `openapi-typescript`. Operations use stable IDs, explicit response unions, session-cookie security, examples, and RFC 7807 bodies with machine-readable codes.

Project Template narrows that contract to system, auth, workspaces, members, and tasks. No agent or monitoring endpoints were carried over.

## Frontend

Netstamp organizes product behavior by feature and keeps reusable primitives in a workspace UI package. Server state belongs to TanStack Query; route and filter state belongs to the URL; small cross-route client concerns belong to focused contexts. Project Template preserves those ownership rules while replacing the monitoring domain with a workspace-and-tasks slice.

## Design

The source system favors dense operational layouts, semantic tokens, restrained geometry, complete component states, keyboard focus, and shared Storybook documentation. Project Template keeps those invariants but uses a neutral identity and a balanced neutral/warm/cool palette rather than copying Netstamp branding, fonts, or exact colors.

## Tooling

The source repository uses pnpm workspaces, a root Justfile, area-level guides, generated contract artifacts, split CI, multi-stage Docker builds, Astro docs, and an optional observability stack. Project Template retains those integration points and tightens drift checks so CI generates into temporary paths rather than overwriting committed outputs.
