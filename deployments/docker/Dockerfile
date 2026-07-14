# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23.5
ARG NODE_VERSION=22.23.1

FROM golang:${GO_VERSION}-alpine AS server-build

WORKDIR /src/server

COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY server/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller \
	&& CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM node:${NODE_VERSION}-alpine AS web-build

WORKDIR /src

ENV PNPM_HOME=/pnpm \
	PATH=/pnpm:$PATH

RUN corepack enable && corepack prepare pnpm@11.11.0 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY api/package.json api/package.json
COPY docs/package.json docs/package.json
COPY packages/ui/package.json packages/ui/package.json
COPY web/package.json web/package.json
COPY scripts/install-hooks.mjs scripts/install-hooks.mjs
COPY scripts/lib/commands.mjs scripts/lib/commands.mjs

RUN --mount=type=cache,id=sitcon-board-pnpm,target=/pnpm/store \
	pnpm config set store-dir /pnpm/store \
	&& pnpm install --frozen-lockfile --filter @project-template/web... --filter @project-template/ui

COPY packages/ui packages/ui
COPY web web

RUN pnpm --filter @project-template/web build

FROM alpine:3.23 AS runtime

ARG APP_VERSION=dev
ARG VCS_REVISION=unknown

LABEL org.opencontainers.image.title="SITCON Board" \
	org.opencontainers.image.description="GitLab-backed workflow board for SITCON 2027" \
	org.opencontainers.image.source="https://github.com/sitcon-tw/sitlab" \
	org.opencontainers.image.version="${APP_VERSION}" \
	org.opencontainers.image.revision="${VCS_REVISION}" \
	org.opencontainers.image.licenses="Apache-2.0"

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S -g 10001 sitcon_board \
	&& adduser -S -D -H -u 10001 -G sitcon_board sitcon_board

WORKDIR /app

COPY --chmod=0555 --from=server-build /out/controller /app/controller
COPY --chmod=0555 --from=server-build /out/migrate /app/migrate
COPY --from=server-build /src/server/db/migrations /app/db/migrations
COPY --from=web-build /src/web/dist /app/web
COPY --chmod=0444 .sitcon/board-directory.yml /app/board-directory.yml
COPY LICENSE /app/LICENSE

ENV SITCON_BOARD_HTTP_ADDR=:8080 \
	SITCON_BOARD_DIRECTORY_FILE=/app/board-directory.yml \
	SITCON_BOARD_WEB_DIR=/app/web

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/health/ready || exit 1

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/controller"]
