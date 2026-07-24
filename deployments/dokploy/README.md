# Deploy SITCON Board on Dokploy

This Compose stack builds the repository Dockerfile, runs PostgreSQL, applies migrations once, and starts the application only after the migration succeeds. Dokploy routes HTTPS traffic to the `app` service on container port `8080`; PostgreSQL is never attached to the public Dokploy network.

## 1. Prepare DNS

Choose the production origin, for example `https://board.example.com`.

- Point the domain's `A` record to the Dokploy server IPv4 address.
- Add an `AAAA` record only when the Dokploy server has working public IPv6.
- Use the origin without a trailing slash for `SITCON_BOARD_PUBLIC_URL`.

## 2. Prepare GitLab

Create a GitLab OAuth application:

- Name: `SITCON Board`
- Redirect URI: `https://board.example.com/api/v1/auth/gitlab/callback`
- Confidential: enabled
- Scope: `read_api`

The redirect URI must exactly match the public URL used in Dokploy.

Create a project access token in `sitcon-tw/2027`:

- Role: `Developer`
- Scope: `api`
- Expiration: set an operationally appropriate expiry and calendar its rotation

Store the token as `SITCON_BOARD_GITLAB_PROJECT_ACCESS_TOKEN`. The application uses it to read project members, labels and issues, and to reconcile issue mutations.

Create a project webhook in `sitcon-tw/2027`:

- Name: `SITLAB Board Realtime`
- Description: `Sync GitLab issue changes to SITLAB Board`
- URL: `https://board.example.com/api/v1/webhooks/gitlab/project`
- Signing token: select **Generate signing token**, then immediately store the complete `whsec_...` value as `SITCON_BOARD_GITLAB_PROJECT_WEBHOOK_SIGNING_TOKEN`
- Secret token: empty
- Trigger: **Work item events** only
- Custom headers and custom webhook template: empty
- SSL verification: enabled

The Work item trigger still sends Issue payloads with `X-Gitlab-Event: Issue Hook`; the application ignores non-Issue work item types.

Create a group webhook in `sitcon-tw`:

- Name: `SITLAB Member Realtime`
- Description: `Refresh SITLAB Board members after GitLab group membership changes`
- URL: `https://board.example.com/api/v1/webhooks/gitlab/group`
- Signing token: generate a second token and store it as `SITCON_BOARD_GITLAB_GROUP_WEBHOOK_SIGNING_TOKEN`
- Secret token: empty
- Trigger: **Member events** only
- SSL verification: enabled

Group Member events require GitLab Premium or Ultimate and group Owner access. Do not reuse signing tokens. GitLab shows each generated token only for configuration; the receiver validates `webhook-signature`, `webhook-id`, and `webhook-timestamp` and never accepts the legacy plain-text secret as a substitute.

The team directory lives at `.sitcon/board-directory.yml` in this repository. The Docker build copies it into the application image, so no GitHub API token is needed. Push and redeploy after changing the file. The application intentionally stays unready when the directory, member, or board snapshot cannot be initialized.

## 3. Generate secrets

Generate three independent URL-safe values. Do not reuse a value; webhook signing tokens come from GitLab and are not generated with these commands:

```bash
openssl rand -hex 32 # SITCON_BOARD_DATABASE_PASSWORD
openssl rand -hex 32 # SITCON_BOARD_SESSION_HASH_KEY
openssl rand -hex 32 # SITCON_BOARD_OAUTH_STATE_CIPHER_KEY
```

Copy the variables from `example.env` into Dokploy's Environment panel and replace every `change-me` value. Do not commit a production `.env` file.

The PostgreSQL password initializes the persistent database volume. Changing only the environment variable after data exists does not change the password inside PostgreSQL; update the database role deliberately before rotating it.

## 4. Create the Dokploy Compose service

1. In Dokploy, create or open a project and add a Compose service.
2. Select the Git provider and repository `sitcon-tw/sitlab`.
3. Use branch `main`.
4. Set the Compose path to `deployments/dokploy/compose.yaml`.
5. Add all variables from `deployments/dokploy/example.env` to the service environment.
6. Set `SITCON_BOARD_PUBLIC_URL` to the final HTTPS origin without a trailing slash.
7. Deploy once so Dokploy builds the image, starts PostgreSQL, and runs migrations.

Dokploy creates the external `dokploy-network` during installation. The Compose file expects that standard network name.

## 5. Attach the domain

In the Compose service domain settings:

- Service: `app`
- Container port: `8080`
- Domain: the hostname used by `SITCON_BOARD_PUBLIC_URL`
- Path: `/`
- HTTPS: enabled with a valid certificate

Do not publish PostgreSQL or add a domain to the `postgres` or `migrate` services.

After the certificate is active, confirm that the OAuth application still has the exact callback URI. Then redeploy if any environment variable changed. In each GitLab webhook's **Recent events** menu, send a test and require a `2xx` response before relying on realtime delivery. Finally, edit a real labeled Issue and verify that an already-open board updates without a reload.

## 6. Verify the deployment

The expected service sequence is:

```text
postgres healthy -> migrate exits 0 -> app initializes directory/GitLab snapshots -> app healthy
```

Check these URLs:

```text
https://board.example.com/api/v1/health/live
https://board.example.com/api/v1/health/ready
https://board.example.com/
```

`health/ready` returns success only after PostgreSQL and all required snapshots are ready. Open the root URL and complete GitLab OAuth to verify the callback, session cookie, and project membership check.

## Updating

Push a commit to `main`, then redeploy the Compose service in Dokploy. Every deployment builds the same Dockerfile and runs idempotent Goose migrations before replacing the app. Set `SITCON_BOARD_VERSION` and `SITCON_BOARD_REVISION` to a release tag or commit SHA when you want image metadata to identify an exact revision.

Back up the `sitcon-board-postgres` volume or configure Dokploy database backups before production use. Test restoration separately; an untested backup is not a recovery plan.

## Troubleshooting

- `production session cookie must be Secure`: confirm `SITCON_BOARD_ENV=production` and use this Dokploy Compose file, which forces `SITCON_BOARD_SESSION_COOKIE_SECURE=true`.
- `initial source sync` with a directory file error: verify the image was rebuilt from a revision containing `.sitcon/board-directory.yml`.
- `initial source sync` with a GitLab error: verify the project access token and its role/scope in `sitcon-tw/2027`.
- OAuth callback error: compare the GitLab Redirect URI and the generated `${SITCON_BOARD_PUBLIC_URL}/api/v1/auth/gitlab/callback` character for character.
- Webhook `401`: confirm the complete generated `whsec_...` value is stored under the matching project or group environment key, and that GitLab and the app clocks are synchronized.
- Webhook `400`: confirm the project is exactly `sitcon-tw/2027`, the group is exactly `sitcon-tw`, and no custom webhook template is configured.
- Webhook succeeds but the board is stale: inspect `gitlab_webhook_deliveries_total`, `gitlab_webhook_processing_duration_seconds`, and GitLab Recent events; the 30-second Board poll remains the issue catch-up fallback.
- CSRF origin error: `SITCON_BOARD_PUBLIC_URL` must be the browser's exact HTTPS origin and must not include a path.
- PostgreSQL authentication error after rotating an environment variable: restore the old value or update the persisted PostgreSQL role password before redeploying.
