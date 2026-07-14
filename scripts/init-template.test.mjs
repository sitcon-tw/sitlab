import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));

const sourceSlug = ["project", "template"].join("-");
const source = {
	brandName: ["Yoru", "plate"].join(""),
	name: ["Project", "Template"].join(" "),
	slug: sourceSlug,
	goModule: ["example.com", sourceSlug].join("/"),
	npmScope: `@${sourceSlug}`,
	envPrefix: ["PROJECT", "TEMPLATE"].join("_"),
	cookiePrefix: ["project", "template"].join("_")
};

function writeFixture(root) {
	mkdirSync(path.join(root, "scripts/lib"), { recursive: true });
	mkdirSync(path.join(root, "server"), { recursive: true });
	mkdirSync(path.join(root, "api"), { recursive: true });
	mkdirSync(path.join(root, "docs"), { recursive: true });
	mkdirSync(path.join(root, "bin"), { recursive: true });
	copyFileSync(path.join(scriptsDir, "init-template.mjs"), path.join(root, "scripts/init-template.mjs"));
	copyFileSync(path.join(scriptsDir, "check-placeholders.mjs"), path.join(root, "scripts/check-placeholders.mjs"));
	copyFileSync(path.join(scriptsDir, "lib/commands.mjs"), path.join(root, "scripts/lib/commands.mjs"));

	writeFileSync(path.join(root, ".template-source.json"), `${JSON.stringify(source, null, 2)}\n`);
	writeFileSync(
		path.join(root, "package.json"),
		`${JSON.stringify({ name: source.slug, private: true, scripts: { "template:init": "node scripts/init-template.mjs" } }, null, 2)}\n`
	);
	writeFileSync(
		path.join(root, "README.md"),
		`<!-- template-source-only:start -->\nClone ${source.brandName}.\n<!-- template-source-only:end -->\n# ${source.brandName}\n\n${source.name} ${source.goModule} ${source.npmScope} ${source.envPrefix} ${source.cookiePrefix}\n`
	);
	chmodSync(path.join(root, "README.md"), 0o640);
	writeFileSync(path.join(root, "LICENSE"), `Copyright (c) 2026 ${source.brandName} contributors\n`);
	writeFileSync(path.join(root, "server/go.mod"), `module ${source.goModule}\n\ngo 1.23.0\n`);
	writeFileSync(path.join(root, "api/package.json"), `${JSON.stringify({ name: `${source.npmScope}/api`, private: true }, null, 2)}\n`);
	writeFileSync(
		path.join(root, "docs/getting-started.mdx"),
		`Before.\n\n{/* template-source-only:start */}\nInitialize ${source.brandName}.\n{/* template-source-only:end */}\n\nAfter.\n`
	);

	const fakePnpm = path.join(root, "bin/pnpm");
	writeFileSync(
		fakePnpm,
		`#!/usr/bin/env bash\nset -eu\nprintf '%s\\n' "$*" >> "$FAKE_PNPM_LOG"\nif [[ "\${FAKE_PNPM_MUTATE_ON_INSTALL:-}" == "true" && "\${1:-}" == "install" ]]; then\n\tmkdir -p docs\n\tprintf '{"generated":true}\\n' > docs/created-during-install.json\n\tchmod 600 README.md\nfi\nif [[ "\${FAKE_PNPM_FAIL_ON:-}" == "\${1:-}" ]]; then exit 42; fi\n`
	);
	chmodSync(fakePnpm, 0o755);
}

function invoke(root, extraEnv = {}) {
	return spawnSync(
		process.execPath,
		[
			path.join(root, "scripts/init-template.mjs"),
			"--name",
			"Acme Console",
			"--slug",
			"acme-console",
			"--go-module",
			"github.com/acme/console",
			"--npm-scope",
			"@acme-console",
			"--env-prefix",
			"ACME_CONSOLE",
			"--cookie-prefix",
			"acme_console"
		],
		{
			cwd: root,
			encoding: "utf8",
			env: {
				...process.env,
				PATH: `${path.join(root, "bin")}:${process.env.PATH ?? ""}`,
				FAKE_PNPM_LOG: path.join(root, ".fake-pnpm.log"),
				...extraEnv
			}
		}
	);
}

test("initializer rolls back a failed command and remains one-time after a successful retry", () => {
	const root = mkdtempSync(path.join(tmpdir(), "template-init-test-"));
	const sourceFiles = [".template-source.json", "package.json", "README.md", "LICENSE", "server/go.mod", "api/package.json", "docs/getting-started.mdx"];

	try {
		writeFixture(root);
		const before = new Map(sourceFiles.map((relative) => [relative, readFileSync(path.join(root, relative), "utf8")]));
		const readmeMode = statSync(path.join(root, "README.md")).mode & 0o777;

		const failed = invoke(root, { FAKE_PNPM_FAIL_ON: "format", FAKE_PNPM_MUTATE_ON_INSTALL: "true" });
		assert.notEqual(failed.status, 0);
		assert.equal(existsSync(path.join(root, ".template-initialized.json")), false);
		assert.equal(existsSync(path.join(root, "docs/created-during-install.json")), false);
		for (const [relative, content] of before) assert.equal(readFileSync(path.join(root, relative), "utf8"), content, `${relative} was not restored`);
		assert.equal(statSync(path.join(root, "README.md")).mode & 0o777, readmeMode);
		assert.deepEqual(readFileSync(path.join(root, ".fake-pnpm.log"), "utf8").trim().split("\n"), ["install", "format"]);

		const succeeded = invoke(root);
		assert.equal(succeeded.status, 0, succeeded.stderr);
		assert.equal(existsSync(path.join(root, ".template-source.json")), false);
		const metadata = JSON.parse(readFileSync(path.join(root, ".template-initialized.json"), "utf8"));
		assert.equal(metadata.name, "Acme Console");
		assert.equal(metadata.goModule, "github.com/acme/console");
		assert.match(readFileSync(path.join(root, "package.json"), "utf8"), /acme-console/);
		const initializedReadme = readFileSync(path.join(root, "README.md"), "utf8");
		assert.match(initializedReadme, /^# Acme Console/m);
		assert.doesNotMatch(initializedReadme, /template-source-only|Clone/);
		assert.doesNotMatch(initializedReadme, new RegExp(source.slug));
		assert.doesNotMatch(initializedReadme, new RegExp(source.brandName));
		assert.equal(readFileSync(path.join(root, "LICENSE"), "utf8"), `Copyright (c) 2026 ${source.brandName} contributors\n`);
		assert.equal(readFileSync(path.join(root, "docs/getting-started.mdx"), "utf8"), "Before.\n\n\nAfter.\n");
		const placeholderCheck = spawnSync(process.execPath, [path.join(root, "scripts/check-placeholders.mjs")], { cwd: root, encoding: "utf8" });
		assert.equal(placeholderCheck.status, 0, placeholderCheck.stderr);

		const secondRun = invoke(root);
		assert.notEqual(secondRun.status, 0);
	} finally {
		rmSync(root, { recursive: true, force: true });
	}
});
