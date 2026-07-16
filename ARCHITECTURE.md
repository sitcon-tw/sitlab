# SITCON Board Architecture

## Product Boundary

SITCON Board is a focused GitLab-backed workflow for the fixed `sitcon-tw/2027` project. The primary path is GitLab OAuth, primary-team confirmation, quick card creation, movement, assignment, due-date adjustment, and closing. There are no workspace, generic task, password-registration, or arbitrary-project concepts.

## Data Flow

```text
Bundled board-directory.yml + GitLab project members + GitLab issues
                                   |
                            background sync
                                   v
                    PostgreSQL snapshots
                              |
                   injected bootstrap JSON
                              v
                       React Board
```

Production traffic is ready only after directory, member, and board snapshots exist. The server injects the complete authenticated bootstrap payload into `index.html`; React renders from that payload before starting background refresh. The development API fallback is `GET /api/v1/bootstrap`.

## Backend Boundaries

Dependencies point inward: HTTP transport calls application use cases, application packages own narrow ports, infrastructure implements those ports, and domain packages own board, directory, identity, and sync models. The file adapter supplies the repository-owned directory bundled into the image; the GitLab adapter supplies members and issues. PostgreSQL adapters do not import application or transport packages. The composition root is the only package that constructs concrete adapters.

Card mutations write the optimistic card cache and a durable operation in one transaction. A worker sends the current canonical card intent, including title, Markdown description, Start/Due dates, labels, and GitLab-native multiple assignees, then reconciles the GitLab response. Open list keys map to the GitLab scoped labels `Status::Waiting`, `Status::Inbox`, `Status::To Do`, `Status::Doing`, and `Status::Review`; the Closed list maps to GitLab issue state. Scoped status labels win over legacy plain labels while reading, and mutations remove those legacy labels. New cards use a temporary negative IID; PostgreSQL updates it to GitLab's positive IID with deferred cascading foreign keys. Failed operations retain the optimistic UI state and can be retried. Normal pending, processing, and successful sync states remain quiet in the browser; only offline or failed states surface technical status.

## Identity And Security

GitLab OAuth uses Authorization Code with PKCE and a single-use server-side state record. Login is restricted to active members of `sitcon-tw/2027`. Browser authentication uses a random opaque token in an HttpOnly cookie; PostgreSQL stores only a keyed digest. Sessions use a 14-day rolling expiry: every valid request renews both the database expiry and browser cookie. Authenticated mutations require the session-bound CSRF token and an allowed Origin.

## Frontend Ownership

`web/src/features/board` owns Board state, the detailed planning dialog, multiple-assignee selection, and optimistic reconciliation. `web/src/features/onboarding` owns primary-team confirmation. TanStack Query owns server snapshots, feature state owns unsaved interaction, and bootstrap initialization pre-fills the first render. Team names, title prefixes, member lists, the six ordered board lists, and cards come from the backend.

## Contract Flow

TypeSpec under `api/` is the HTTP source of truth. Generation emits byte-identical OpenAPI for docs and the embedded backend document, plus TypeScript declarations for the web client. Generated artifacts are never edited manually.
