set dotenv-load := true

server_dir := "server"
api_filter := "@project-template/api"
docs_filter := "@project-template/docs"
ui_filter := "@project-template/ui"
web_filter := "@project-template/web"

alias fmt := format
alias generate := contract-generate
alias migrate-up := backend-migrate-up

# List canonical repository commands.
default:
    @just --list --unsorted

# Install workspace dependencies.
install:
    pnpm install

# Start the Go API. Run `just web-dev` in another terminal for the client.
dev: backend-dev

# Build every production surface.
build: api-build backend-build ui-build web-build storybook-build docs-build

# Run static analysis for every language surface.
lint: api-lint backend-lint frontend-style-check web-lint ui-lint docs-check

# Run all unit and component tests.
test: backend-test web-test ui-test template-test

# Run all type checkers.
typecheck: api-check web-typecheck ui-typecheck docs-check

# Run the same deterministic gates as CI.
ci: format-check generated-check lint typecheck test build

# Format repository-owned source files.
format:
    pnpm format
    cd {{ server_dir }} && golangci-lint fmt --config ../golangci.yaml

# Check formatting without retaining formatter changes.
format-check:
    pnpm format:check
    @test -z "$$(gofmt -l {{ server_dir }})" || { echo "Go files need formatting; run 'just format'."; gofmt -l {{ server_dir }}; exit 1; }
    @diff="$$(cd {{ server_dir }} && golangci-lint fmt --diff --config ../golangci.yaml)" || exit $$?; test -z "$$diff" || { echo "Go imports need formatting; run 'just format'."; echo "$$diff"; exit 1; }

# Remove build and coverage output; generated contracts remain tracked.
clean:
    rm -rf docs/.astro docs/dist docs/public/storybook packages/ui/dist packages/ui/storybook-static scripts/.contract-* server/bin server/coverage.out server/tmp web/dist web/playwright-report web/test-results

# Build the TypeSpec contract without emitting artifacts.
api-build:
    pnpm --filter {{ api_filter }} build

# Check the TypeSpec contract.
api-check:
    pnpm --filter {{ api_filter }} check

# Lint the TypeSpec contract.
api-lint:
    pnpm --filter {{ api_filter }} lint

# Regenerate docs, backend, and web artifacts from TypeSpec.
contract-generate:
    pnpm generate

# Regenerate contracts in a temporary directory and reject drift.
generated-check:
    pnpm generated:check

# Start the documentation site.
docs-dev:
    pnpm --filter {{ docs_filter }} dev

# Check Astro and documentation types.
docs-check:
    pnpm --filter {{ docs_filter }} check

# Build the documentation site from already-built public assets.
docs-build:
    pnpm --filter {{ docs_filter }} build:site

# Preview the built documentation site.
docs-preview:
    pnpm --filter {{ docs_filter }} preview

# Build the shared UI package.
ui-build:
    pnpm --filter {{ ui_filter }} build

# Typecheck the shared UI package.
ui-typecheck:
    pnpm --filter {{ ui_filter }} typecheck

# Lint the shared UI package.
ui-lint:
    pnpm --filter {{ ui_filter }} lint

# Test the shared UI package.
ui-test:
    pnpm --filter {{ ui_filter }} test

# Start Storybook.
storybook-dev:
    pnpm --filter {{ ui_filter }} storybook

# Publish static Storybook output into the docs site.
storybook-build:
    pnpm --filter {{ ui_filter }} build:storybook -o ../../docs/public/storybook

# Start the React app.
web-dev:
    pnpm --filter {{ web_filter }} dev

# Build the React app.
web-build:
    pnpm --filter {{ web_filter }} build

# Lint the React app.
web-lint:
    pnpm --filter {{ web_filter }} lint

# Typecheck the React app.
web-typecheck:
    pnpm --filter {{ web_filter }} typecheck

# Test the React app.
web-test:
    pnpm --filter {{ web_filter }} test

# Check token and keyboard-focus policies across browser surfaces.
frontend-style-check:
    pnpm check:frontend-style

# Verify initializer rollback, retry, and one-time semantics.
template-test:
    pnpm test:template

# Start the backend API with the Go toolchain.
backend-dev:
    cd {{ server_dir }} && go run ./cmd/controller

# Build backend controller and migration binaries.
backend-build:
    mkdir -p {{ server_dir }}/bin
    cd {{ server_dir }} && go build -buildvcs=false -o bin/controller ./cmd/controller
    cd {{ server_dir }} && go build -buildvcs=false -o bin/migrate ./cmd/migrate

# Run backend tests.
backend-test:
    cd {{ server_dir }} && go test ./...

# Run backend integration tests against SITCON_BOARD_TEST_DATABASE_URL.
backend-test-integration:
    cd {{ server_dir }} && go test -v -tags=integration ./internal/e2e/...

# Run backend static analysis.
backend-lint:
    cd {{ server_dir }} && golangci-lint run --config ../golangci.yaml ./...

# Apply safe backend lint fixes.
backend-lint-fix:
    cd {{ server_dir }} && golangci-lint run --fix --config ../golangci.yaml ./...

# Tidy backend Go module dependencies.
backend-tidy:
    cd {{ server_dir }} && go mod tidy

# Show migration status.
backend-migrate-status:
    cd {{ server_dir }} && go run ./cmd/migrate -command status -dir db/migrations

# Apply pending migrations.
backend-migrate-up:
    cd {{ server_dir }} && go run ./cmd/migrate -command up -dir db/migrations

# Roll back the latest migration.
backend-migrate-down:
    cd {{ server_dir }} && go run ./cmd/migrate -command down -dir db/migrations

# Build the production image.
docker-build:
    docker build -f deployments/docker/Dockerfile -t sitcon-board:local .

# Start the local production-shaped stack.
docker-up:
    docker compose --env-file deployments/docker/.env -f deployments/docker/compose.yaml up --build

# Stop the local stack.
docker-down:
    docker compose --env-file deployments/docker/.env -f deployments/docker/compose.yaml down
