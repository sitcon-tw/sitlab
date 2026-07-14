#!/usr/bin/env node

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

const repoRoot = process.cwd();
const scanRoots = ["web/src", "docs/src", "packages/ui/src", "packages/ui/.storybook"];
const sourcePattern = /\.(astro|css|js|jsx|mjs|ts|tsx)$/;
const rawColorPattern = /#[0-9a-fA-F]{3,8}\b|(?:rgb|hsl)a?\(/g;
const unsafeFocusPattern = /outline:\s*(?:none|0)\b|:focus(?!-visible|-within)/g;
const ignoredParts = new Set(["node_modules", "dist", ".astro", "storybook-static"]);

const rawColorAllowlist = new Map([
	["docs/src/layouts/BaseLayout.astro", "browser theme-color metadata requires a concrete color"],
	["docs/src/styles/global.css", "documentation shell owns its fallback token values"],
	["packages/ui/src/styles/tokens.css", "the token source of truth must contain concrete values"]
]);

const focusAllowlist = new Map();

function relative(file) {
	return path.relative(repoRoot, file).split(path.sep).join("/");
}

function walk(root) {
	const absolute = path.join(repoRoot, root);
	if (!existsSync(absolute)) return [];

	const files = [];
	for (const entry of readdirSync(absolute)) {
		const candidate = path.join(absolute, entry);
		const relativeCandidate = relative(candidate);
		if (relativeCandidate.split("/").some((part) => ignoredParts.has(part))) continue;
		if (statSync(candidate).isDirectory()) files.push(...walk(relativeCandidate));
		else if (sourcePattern.test(entry) && !entry.endsWith(".d.ts")) files.push(candidate);
	}
	return files;
}

function issuesFor(file, pattern, allowlist, kind) {
	const fileName = relative(file);
	if (allowlist.has(fileName)) return [];
	const source = readFileSync(file, "utf8");
	return [...source.matchAll(pattern)].map((match) => ({
		fileName,
		kind,
		line: source.slice(0, match.index).split("\n").length,
		excerpt: source.split(/\r?\n/)[source.slice(0, match.index).split("\n").length - 1]?.trim() ?? ""
	}));
}

const files = scanRoots.flatMap(walk);
const issues = files.flatMap((file) => [
	...issuesFor(file, rawColorPattern, rawColorAllowlist, "raw color"),
	...issuesFor(file, unsafeFocusPattern, focusAllowlist, "unsafe focus reset")
]);

if (issues.length > 0) {
	console.error("Frontend style check failed.");
	for (const issue of issues) console.error(`${issue.fileName}:${issue.line}: ${issue.kind}: ${issue.excerpt}`);
	console.error("Use semantic UI tokens and :focus-visible. Add a path-specific exception with a reason only when the platform requires it.");
	process.exit(1);
}

console.log(`Frontend style check passed (${files.length} files scanned).`);
