#!/usr/bin/env node

import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "./lib/commands.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

if (!existsSync(path.join(repoRoot, ".git"))) process.exit(0);

run("git", ["config", "core.hooksPath", ".githooks"], { cwd: repoRoot });
console.log("Configured repository hooks from .githooks/.");
