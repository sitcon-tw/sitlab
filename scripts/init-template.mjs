#!/usr/bin/env node

import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "./lib/commands.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceMarker = path.join(repoRoot, ".template-source.json");
const initializedMarker = path.join(repoRoot, ".template-initialized.json");

const allowedRoots = new Set([".github", ".githooks", "api", "deployments", "docs", "packages", "scripts", "server", "web"]);
const allowedRootFiles = new Set([
	".editorconfig",
	".dockerignore",
	".gitignore",
	".nvmrc",
	".prettierignore",
	".prettierrc.json",
	"AGENTS.md",
	"ARCHITECTURE.md",
	"Justfile",
	"LICENSE",
	"README.md",
	"design.md",
	"golangci.yaml",
	"package.json",
	"pnpm-lock.yaml",
	"pnpm-workspace.yaml"
]);
const allowedExtensions = new Set([
	".astro",
	".css",
	".go",
	".html",
	".js",
	".json",
	".md",
	".mdx",
	".mjs",
	".mod",
	".sql",
	".sum",
	".tsp",
	".ts",
	".tsx",
	".txt",
	".yaml",
	".yml"
]);
const excludedParts = new Set([
	".astro",
	".git",
	"coverage",
	"dist",
	"node_modules",
	"playwright-report",
	"storybook-static",
	"test-results",
	"tmp",
	"tsp-output"
]);
const excludedRelativeDirectories = new Set(["docs/public/storybook"]);
const replacementExcludedFiles = new Set([
	"LICENSE",
	"docs/public/openapi.json",
	"scripts/check-placeholders.mjs",
	"scripts/init-template.mjs",
	"server/internal/controller/transport/http/openapi/openapi.json",
	"web/src/shared/api/openapi.d.ts"
]);
const sourceOnlyPattern =
	/^[\t ]*(?:<!-- template-source-only:start -->|\{\/\* template-source-only:start \*\/\})\r?\n[\s\S]*?^[\t ]*(?:<!-- template-source-only:end -->|\{\/\* template-source-only:end \*\/\})\r?\n?/gm;

function collectManagedFiles(directory = repoRoot) {
	const relativeDirectory = path.relative(repoRoot, directory).split(path.sep).join("/");
	const entries = readdirSync(directory, { withFileTypes: true });
	const files = [];

	for (const entry of entries) {
		const absolute = path.join(directory, entry.name);
		const relative = path.relative(repoRoot, absolute).split(path.sep).join("/");
		const parts = relative.split("/");
		if (parts.some((part) => excludedParts.has(part))) continue;

		if (entry.isDirectory()) {
			if (excludedRelativeDirectories.has(relative) || entry.name.startsWith(".contract-")) continue;
			const root = parts[0];
			if (relativeDirectory || allowedRoots.has(root)) files.push(...collectManagedFiles(absolute));
			continue;
		}

		if (!entry.isFile()) continue;
		if (parts.length === 1) {
			if (allowedRootFiles.has(relative)) files.push(relative);
			continue;
		}

		if (
			allowedRoots.has(parts[0]) &&
			(allowedExtensions.has(path.extname(entry.name)) ||
				entry.name === "Dockerfile" ||
				entry.name === "example.env" ||
				entry.name.endsWith(".env.example") ||
				parts[0] === ".githooks")
		) {
			files.push(relative);
		}
	}

	return files;
}

function collectMutableFiles() {
	return collectManagedFiles().filter((relative) => !replacementExcludedFiles.has(relative));
}

function createSnapshot() {
	const snapshotRoot = mkdtempSync(path.join(tmpdir(), "project-template-init-"));
	const backupRoot = path.join(snapshotRoot, "files");
	const relativeFiles = new Set(collectManagedFiles());
	relativeFiles.add(path.relative(repoRoot, sourceMarker).split(path.sep).join("/"));
	const modes = new Map();

	try {
		for (const relative of relativeFiles) {
			const sourcePath = path.join(repoRoot, relative);
			if (!existsSync(sourcePath)) continue;
			const backupPath = path.join(backupRoot, relative);
			mkdirSync(path.dirname(backupPath), { recursive: true });
			copyFileSync(sourcePath, backupPath);
			modes.set(relative, statSync(sourcePath).mode & 0o777);
		}
	} catch (error) {
		rmSync(snapshotRoot, { recursive: true, force: true });
		throw error;
	}

	return { backupRoot, modes, relativeFiles, snapshotRoot };
}

function restoreSnapshot(snapshot, temporaryMetadata) {
	for (const relative of collectManagedFiles()) {
		if (!snapshot.relativeFiles.has(relative)) rmSync(path.join(repoRoot, relative), { force: true });
	}

	for (const relative of snapshot.relativeFiles) {
		const backupPath = path.join(snapshot.backupRoot, relative);
		if (!existsSync(backupPath)) continue;
		const destination = path.join(repoRoot, relative);
		mkdirSync(path.dirname(destination), { recursive: true });
		copyFileSync(backupPath, destination);
		const mode = snapshot.modes.get(relative);
		if (mode !== undefined) chmodSync(destination, mode);
	}

	rmSync(initializedMarker, { force: true });
	rmSync(temporaryMetadata, { force: true });
}

function assertNoSourcePlaceholders(source) {
	const ignored = new Set(["LICENSE", "scripts/check-placeholders.mjs", "scripts/init-template.mjs"]);
	const placeholders = Object.values(source).filter((value) => typeof value === "string" && value !== "");
	const residue = [];

	for (const relative of collectManagedFiles()) {
		if (ignored.has(relative)) continue;
		const content = readFileSync(path.join(repoRoot, relative), "utf8");
		if (placeholders.some((placeholder) => content.includes(placeholder))) residue.push(relative);
	}

	if (residue.length > 0) throw new Error(`template placeholders remain:\n${residue.map((relative) => `- ${relative}`).join("\n")}`);
}

function usage(message) {
	if (message) console.error(message);
	console.error(
		"Usage: pnpm run template:init -- --name <name> --slug <slug> --go-module <module> --npm-scope <@scope> --env-prefix <PREFIX> --cookie-prefix <prefix>"
	);
	process.exit(2);
}

function parseArgs(argv) {
	const values = {};
	for (let index = 0; index < argv.length; index += 2) {
		const key = argv[index];
		const value = argv[index + 1];
		if (!key?.startsWith("--") || !value || value.startsWith("--")) usage(`Missing value for ${key ?? "argument"}.`);
		values[key.slice(2)] = value;
	}
	return values;
}

function requireMatch(label, value, pattern, example) {
	if (!value || !pattern.test(value)) usage(`${label} must match ${example}.`);
}

if (existsSync(initializedMarker) || !existsSync(sourceMarker)) {
	console.error("This repository has already been initialized. The initializer is intentionally one-time only.");
	process.exit(1);
}

const values = parseArgs(process.argv.slice(2).filter((argument) => argument !== "--"));
const next = {
	name: values.name,
	slug: values.slug,
	goModule: values["go-module"],
	npmScope: values["npm-scope"],
	envPrefix: values["env-prefix"],
	cookiePrefix: values["cookie-prefix"]
};

requireMatch("name", next.name, /^[A-Za-z][A-Za-z0-9 ._-]{1,79}$/, "a readable 2-80 character product name");
requireMatch("slug", next.slug, /^[a-z0-9]+(?:-[a-z0-9]+)*$/, "lowercase-kebab-case");
requireMatch("Go module", next.goModule, /^[a-z0-9][a-z0-9.-]*(?:\/[A-Za-z0-9._~-]+)+$/, "example.com/your-project");
requireMatch("npm scope", next.npmScope, /^@[a-z0-9][a-z0-9._-]*$/, "@lowercase-scope");
requireMatch("environment prefix", next.envPrefix, /^[A-Z][A-Z0-9_]*$/, "UPPER_SNAKE_CASE");
requireMatch("cookie prefix", next.cookiePrefix, /^[a-z][a-z0-9_]*$/, "lower_snake_case");

const source = JSON.parse(readFileSync(sourceMarker, "utf8"));
const replacements = [
	[source.goModule, next.goModule],
	[source.npmScope, next.npmScope],
	[source.envPrefix, next.envPrefix],
	[source.cookiePrefix, next.cookiePrefix],
	[source.brandName, next.name],
	[source.name, next.name],
	[source.slug, next.slug]
];
const snapshot = createSnapshot();
const temporaryMetadata = `${initializedMarker}.${process.pid}.tmp`;

try {
	for (const relativePath of collectMutableFiles()) {
		const absolutePath = path.join(repoRoot, relativePath);
		if (!existsSync(absolutePath)) continue;
		let content = readFileSync(absolutePath, "utf8");
		for (const [from, to] of replacements) content = content.replaceAll(from, to);
		content = content.replace(sourceOnlyPattern, "");
		writeFileSync(absolutePath, content);
	}

	run("pnpm", ["install"], { cwd: repoRoot });
	run("pnpm", ["format"], { cwd: repoRoot });
	run("pnpm", ["generate"], { cwd: repoRoot });
	run("pnpm", ["generated:check"], { cwd: repoRoot });
	assertNoSourcePlaceholders(source);

	const metadata = { ...next, initializedAt: new Date().toISOString() };
	writeFileSync(temporaryMetadata, `${JSON.stringify(metadata, null, 2)}\n`);
	renameSync(temporaryMetadata, initializedMarker);
	rmSync(sourceMarker);
} catch (error) {
	try {
		restoreSnapshot(snapshot, temporaryMetadata);
	} catch (rollbackError) {
		throw new AggregateError([error, rollbackError], "Template initialization failed and rollback could not fully restore the source tree.");
	}
	console.error("Initialization failed. Source files and template markers were restored; the initializer can be retried.");
	throw error;
} finally {
	rmSync(snapshot.snapshotRoot, { recursive: true, force: true });
	rmSync(temporaryMetadata, { force: true });
}

console.log(`Initialized ${next.name}. Review the generated diff before the first commit.`);
