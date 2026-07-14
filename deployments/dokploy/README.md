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

The team directory lives at `.sitcon/board-directory.yml` in this repository. The Docker build copies it into the application image, so no GitHub API token is needed. Push and redeploy after changing the file. The application intentionally stays unready when the directory, member, or board snapshot cannot be initialized.

## 3. Generate secrets

Generate three independent URL-safe values. Do not reuse a value:

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

After the certificate is active, confirm that the OAuth application still has the exact callback URI. Then redeploy if any environment variable changed.

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
- CSRF origin error: `SITCON_BOARD_PUBLIC_URL` must be the browser's exact HTTPS origin and must not include a path.
- PostgreSQL authentication error after rotating an environment variable: restore the old value or update the persisted PostgreSQL role password before redeploying.
