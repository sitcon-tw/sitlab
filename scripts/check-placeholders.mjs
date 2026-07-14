#!/usr/bin/env node

import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const metadataPath = path.join(repoRoot, ".template-initialized.json");

if (!existsSync(metadataPath)) {
	console.log("Template has not been initialized; source placeholders are expected.");
	process.exit(0);
}

const sourcePlaceholders = [
	"Yoruplate",
	"Project Template",
	"project-template",
	"example.com/project-template",
	"@project-template",
	"PROJECT_TEMPLATE",
	"project_template"
];
const excludedDirectories = new Set([
	".agents",
	".astro",
	".codex",
	".git",
	".idea",
	".vscode",
	"coverage",
	"dist",
	"node_modules",
	"playwright-report",
	"storybook-static",
	"test-results",
	"tmp",
	"tsp-output"
]);
const excludedFiles = new Set(["LICENSE", "scripts/check-placeholders.mjs", "scripts/init-template.mjs"]);

function findResidue(directory = repoRoot) {
	const matches = [];
	for (const entry of readdirSync(directory, { withFileTypes: true })) {
		const absolute = path.join(directory, entry.name);
		const relative = path.relative(repoRoot, absolute).split(path.sep).join("/");

		if (entry.isDirectory()) {
			if (excludedDirectories.has(entry.name) || relative === "docs/public/storybook") continue;
			matches.push(...findResidue(absolute));
			continue;
		}

		if (!entry.isFile() || excludedFiles.has(relative)) continue;
		const content = readFileSync(absolute);
		if (content.includes(0)) continue;
		const source = content.toString("utf8");
		if (sourcePlaceholders.some((placeholder) => source.includes(placeholder))) matches.push(relative);
	}
	return matches;
}

const matches = findResidue();
if (matches.length > 0) {
	console.error(`Template placeholders remain:\n${matches.map((file) => `- ${file}`).join("\n")}`);
	process.exit(1);
}

const metadata = JSON.parse(readFileSync(metadataPath, "utf8"));
console.log(`No source placeholders remain for ${metadata.name}.`);
