#!/usr/bin/env node

import { copyFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "./lib/commands.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const openapiSource = path.join(repoRoot, "docs/public/openapi.json");
const backendOpenAPI = path.join(repoRoot, "server/internal/controller/transport/http/openapi/openapi.json");
const webTypes = path.join(repoRoot, "web/src/shared/api/openapi.d.ts");

run("pnpm", ["--filter", "@project-template/api", "generate:openapi"], { cwd: repoRoot });

mkdirSync(path.dirname(backendOpenAPI), { recursive: true });
mkdirSync(path.dirname(webTypes), { recursive: true });

run("pnpm", ["exec", "openapi-typescript", openapiSource, "-o", webTypes], { cwd: repoRoot });
run("pnpm", ["exec", "prettier", "--config", path.join(repoRoot, ".prettierrc.json"), "--write", openapiSource, webTypes], {
	cwd: repoRoot
});
copyFileSync(openapiSource, backendOpenAPI);

console.log("Generated docs, backend, and web contract artifacts.");
