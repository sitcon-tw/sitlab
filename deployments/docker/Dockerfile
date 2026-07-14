# syntax=docker/dockerfile:1.7

FROM golang:1.23.5-alpine AS server-build

WORKDIR /src/server

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller \
	&& CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM node:22.23.1-alpine AS web-build

WORKDIR /src

RUN corepack enable && corepack prepare pnpm@11.11.0 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY api/package.json api/package.json
COPY docs/package.json docs/package.json
COPY packages/ui/package.json packages/ui/package.json
COPY web/package.json web/package.json
COPY scripts/install-hooks.mjs scripts/install-hooks.mjs
COPY scripts/lib/commands.mjs scripts/lib/commands.mjs

RUN pnpm install --frozen-lockfile --filter @project-template/web... --filter @project-template/ui

COPY packages/ui packages/ui
COPY web web

RUN pnpm --filter @project-template/ui build \
	&& pnpm --filter @project-template/web build

FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S project_template \
	&& adduser -S -G project_template project_template

WORKDIR /app

COPY --from=server-build /out/controller /app/controller
COPY --from=server-build /out/migrate /app/migrate
COPY --from=server-build /src/server/db/migrations /app/db/migrations
COPY --from=web-build /src/web/dist /app/web

ENV PROJECT_TEMPLATE_HTTP_ADDR=:8080 \
	PROJECT_TEMPLATE_WEB_DIR=/app/web

USER project_template:project_template

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/healthz || exit 1

ENTRYPOINT ["/app/controller"]
