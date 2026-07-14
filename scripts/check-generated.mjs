#!/usr/bin/env node

import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "./lib/commands.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const tempRoot = mkdtempSync(path.join(repoRoot, "scripts", ".contract-"));
const generatedOpenAPI = path.join(tempRoot, "openapi.json");
const generatedTypes = path.join(tempRoot, "openapi.d.ts");

const artifacts = [
	["docs OpenAPI", path.join(repoRoot, "docs/public/openapi.json"), generatedOpenAPI],
	["backend embedded OpenAPI", path.join(repoRoot, "server/internal/controller/transport/http/openapi/openapi.json"), generatedOpenAPI],
	["web API types", path.join(repoRoot, "web/src/shared/api/openapi.d.ts"), generatedTypes]
];

try {
	run(
		"pnpm",
		["--filter", "@project-template/api", "exec", "tsp", "compile", ".", "--warn-as-error", "--option", `@typespec/openapi3.emitter-output-dir=${tempRoot}`],
		{ cwd: repoRoot }
	);
	run("pnpm", ["exec", "openapi-typescript", generatedOpenAPI, "-o", generatedTypes], { cwd: repoRoot });
	run(
		"pnpm",
		[
			"exec",
			"prettier",
			"--config",
			path.join(repoRoot, ".prettierrc.json"),
			"--ignore-path",
			path.join(repoRoot, ".prettierignore"),
			"--write",
			generatedOpenAPI,
			generatedTypes
		],
		{ cwd: repoRoot }
	);

	const drift = [];
	for (const [label, committed, generated] of artifacts) {
		let committedSource;
		try {
			committedSource = readFileSync(committed, "utf8");
		} catch {
			drift.push(`${label}: missing ${path.relative(repoRoot, committed)}`);
			continue;
		}

		if (committedSource !== readFileSync(generated, "utf8")) {
			drift.push(`${label}: ${path.relative(repoRoot, committed)} is stale`);
		}
	}

	if (drift.length > 0) {
		console.error("Generated contract check failed:");
		for (const message of drift) console.error(`- ${message}`);
		console.error("Run `pnpm generate` and commit all generated artifacts.");
		process.exitCode = 1;
	} else {
		console.log("Generated contract artifacts are current.");
	}
} finally {
	rmSync(tempRoot, { recursive: true, force: true });
}
