import { spawnSync } from "node:child_process";

export function run(command, args, options = {}) {
	const result = spawnSync(command, args, {
		cwd: options.cwd ?? process.cwd(),
		env: { ...process.env, ...options.env },
		stdio: options.stdio ?? "inherit",
		encoding: "utf8"
	});

	if (result.error) {
		throw result.error;
	}

	if (result.status !== 0) {
		const rendered = [command, ...args].join(" ");
		throw new Error(`${rendered} exited with status ${result.status ?? "unknown"}`);
	}

	return result;
}
