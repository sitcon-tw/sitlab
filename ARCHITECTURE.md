# SITCON Board Architecture

## Product Boundary

SITCON Board is a focused GitLab-backed workflow for the fixed `sitcon-tw/2027` project. The primary path is GitLab OAuth, primary-team confirmation, quick card creation, movement, assignment, due-date adjustment, and closing. There are no workspace, generic task, password-registration, or arbitrary-project concepts.

## Data Flow

```text
GitLab signed project/group webhooks -> durable webhook inbox
                                             |
Bundled board-directory.yml + GitLab API -> PostgreSQL snapshots + revision
                                             |
                              injected bootstrap JSON + REST + SSE
                                             |
                                         React Board
```

Production traffic is ready only after directory, member, and board snapshots exist. The server injects the complete authenticated bootstrap payload into `index.html`; React renders from that payload before starting background refresh. The development API fallback is `GET /api/v1/bootstrap`.

Project Issue and group Member webhooks are authenticated with GitLab HMAC signing tokens and committed to a durable inbox before acknowledgement. Workers fetch canonical GitLab state instead of trusting webhook payloads. Every visible transaction advances a PostgreSQL bootstrap revision; `LISTEN/NOTIFY` fans the revision out across instances and a continuously connected authenticated SSE stream tells browsers to refetch. A five-second Board poll and the slower directory poll remain the missed-event fallback.

## Backend Boundaries

Dependencies point inward: HTTP transport calls application use cases, application packages own narrow ports, infrastructure implements those ports, and domain packages own board, directory, identity, and sync models. The file adapter supplies the repository-owned directory bundled into the image; the GitLab adapter supplies members and issues. PostgreSQL adapters do not import application or transport packages. The composition root is the only package that constructs concrete adapters.

Card mutations write the optimistic card cache and a durable operation in one transaction. A worker resolves the encrypted GitLab OAuth credential for the operation's actor, sends the current canonical card intent as that user, then reconciles the GitLab response. The intent includes title, Markdown description, Start/Due dates, labels, and GitLab-native multiple assignees. Open list keys map to the GitLab scoped labels `Status::Waiting`, `Status::Inbox`, `Status::To Do`, `Status::Doing`, and `Status::Review`; the Closed list maps to GitLab issue state. Scoped status labels win over legacy plain labels while reading, and mutations remove those legacy labels. New cards use a temporary negative IID and position zero; PostgreSQL updates the IID to GitLab's positive value with deferred cascading foreign keys. Failed operations retain the optimistic UI state and can be retried. Normal pending, processing, and successful sync states remain quiet in the browser; only offline or failed states surface technical status.

## Identity And Security

GitLab OAuth uses Authorization Code with PKCE, `api` scope, and a single-use server-side state record. Login is restricted to active members of `sitcon-tw/2027`. Access and rotating refresh tokens are encrypted at rest and used only by the durable worker to preserve GitLab actor attribution. Browser authentication uses a random opaque token in an HttpOnly cookie; PostgreSQL stores only a keyed digest. Sessions use a 14-day rolling expiry: every valid request renews both the database expiry and browser cookie. Authenticated mutations require the session-bound CSRF token and an allowed Origin.

## Frontend Ownership

`web/src/features/board` owns Board state, the navigable right-side detail drawer, typed Quick Actions, leader bulk-create, multiple-assignee selection, and optimistic reconciliation. `web/src/features/onboarding` owns primary-team confirmation. TanStack Query owns server snapshots, feature state owns unsaved interaction, and bootstrap initialization pre-fills the first render. Team names, leader IDs, title prefixes, member lists, the six ordered board lists, and cards come from the backend.

## Contract Flow

TypeSpec under `api/` is the HTTP source of truth. Generation emits byte-identical OpenAPI for docs and the embedded backend document, plus TypeScript declarations for the web client. Generated artifacts are never edited manually.
