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

Project Issue and group Member webhooks are authenticated with GitLab HMAC signing tokens and committed to a durable inbox before acknowledgement. Workers fetch canonical GitLab state instead of trusting webhook payloads. Every visible content or sync-state change advances a PostgreSQL bootstrap revision; unchanged recovery polls only refresh snapshot health. `LISTEN/NOTIFY` fans revisions out across instances and a continuously connected authenticated SSE stream tells browsers to refetch. A five-second Board poll and the slower directory poll remain the missed-event fallback.

## Backend Boundaries

Dependencies point inward: HTTP transport calls application use cases, application packages own narrow ports, infrastructure implements those ports, and domain packages own board, directory, identity, and sync models. The file adapter supplies the repository-owned directory bundled into the image; the GitLab adapter supplies members and issues. PostgreSQL adapters do not import application or transport packages. The composition root is the only package that constructs concrete adapters.

Card mutations write the optimistic card cache and a durable operation in one transaction. A worker resolves the encrypted GitLab OAuth credential for the operation's actor, sends the current canonical card intent through the GitLab Work Item GraphQL API as that user, then reconciles the response. The intent includes title, Markdown description, Start/Due dates, labels, GitLab-native multiple assignees, and native lifecycle status. Tag edits accept existing project labels, require exactly one active Team label, and never change lifecycle status; moves and the explicit status control own status changes. PostgreSQL owns manual card positions: GitLab refreshes preserve same-list order, while newly observed cards and external status moves enter at the top of their list. Snapshot merges reject GitLab data older than a completed local mutation. The fixed list keys map by name to `Waiting`, `Inbox`, `To do`, `Doing`, `Review`, and `Done`; synchronization fails readiness instead of guessing when a managed issue uses an unmapped status. Legacy workflow labels are filtered from mutations and the label catalog. New cards use a temporary negative IID and position zero; PostgreSQL updates the IID to GitLab's positive value with deferred cascading foreign keys. Failed operations retain the optimistic UI state and can be retried. Background sync stays quiet, but a field the user just edited shows an in-place saving indicator that resolves to a brief saved marker; only offline or failed states surface technical status.

Project labels are read from GitLab with the project token and do not enter bootstrap. Creating, renaming, and deleting a project label runs as the real actor through the GitLab REST label API, so GitLab's own project role decides who may change a project-wide resource; GraphQL is not used because it has no label update or delete mutation. Team-prefixed and legacy workflow names are reserved and rejected before any GitLab call, and a reserved label cannot be renamed even to a legal new name. A card mutation whose labels no longer all exist drops the vanished ones rather than failing permanently, except for a missing Team label, which the board needs to place the card. Card comments and system notes are fetched on demand with the requesting user's OAuth token, and new comments are written synchronously as that user. Comments are not cached in PostgreSQL and are not added to the durable operation queue; a failed submission leaves the browser draft available for an explicit retry.

## Identity And Security

GitLab OAuth uses Authorization Code with PKCE, `api` scope, and a single-use server-side state record. Login is restricted to active members of `sitcon-tw/2027`. Access and rotating refresh tokens are encrypted at rest and resolved only by application use cases that execute durable card mutations or on-demand comment reads and writes as the real actor. Browser authentication uses a random opaque token in an HttpOnly cookie; PostgreSQL stores only a keyed digest. Sessions use a 14-day rolling expiry: every valid request renews both the database expiry and browser cookie. Authenticated mutations require an allowed Origin and a stable, purpose-separated HMAC CSRF token bound to the session ID; bootstrap refreshes never rotate or invalidate it.

## Frontend Ownership

`web/src/features/board` owns Board state, the navigable right-side detail drawer, Tag selection, Comment queries and drafts, typed Quick Actions, leader bulk-create, multiple-assignee selection, and optimistic reconciliation. `web/src/features/onboarding` owns primary-team confirmation. TanStack Query owns server snapshots and per-card Comment state, feature state owns unsaved interaction, and bootstrap initialization pre-fills the first render. Team names, leader IDs, title prefixes, member lists, the six ordered board lists, and cards come from the backend.

## Contract Flow

TypeSpec under `api/` is the HTTP source of truth. Generation emits byte-identical OpenAPI for docs and the embedded backend document, plus TypeScript declarations for the web client. Generated artifacts are never edited manually.
