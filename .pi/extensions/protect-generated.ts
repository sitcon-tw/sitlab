/**
 * Protect Generated Contract Artifacts
 *
 * Enforces the AGENTS.md rule:
 *   "Generated OpenAPI and TypeScript files are committed review artifacts.
 *    Never hand-edit them. Run `pnpm generate`, then `pnpm generated:check`."
 *
 * Blocks `write` and `edit` tool calls that target generated files produced by
 * `scripts/generate-contract.mjs`. The wire contract lives in `api/**\/*.tsp`;
 * edit that and regenerate instead of touching these outputs by hand.
 *
 * Project-local extension: loads only after the project is trusted.
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

// Tracked outputs of `pnpm generate` (see scripts/generate-contract.mjs),
// plus their committed copies under docs/dist checked by scripts/check-generated.mjs.
const GENERATED_PATHS = [
	"docs/public/openapi.json",
	"docs/dist/openapi.json",
	"server/internal/controller/transport/http/openapi/openapi.json",
	"web/src/shared/api/openapi.d.ts"
];

function isGenerated(rawPath: string): boolean {
	// Normalize to a repo-relative, forward-slash comparison.
	const normalized = rawPath.replace(/\\/g, "/");
	return GENERATED_PATHS.some((p) => normalized === p || normalized.endsWith(`/${p}`));
}

export default function protectGenerated(pi: ExtensionAPI) {
	pi.on("tool_call", async (event, ctx) => {
		if (event.toolName !== "write" && event.toolName !== "edit") {
			return undefined;
		}

		const path = event.input.path as string | undefined;
		if (!path || !isGenerated(path)) {
			return undefined;
		}

		const reason =
			`"${path}" is a generated contract artifact and must not be hand-edited.\n` +
			`Edit the TypeSpec source in api/**/*.tsp, then run:\n` +
			`  pnpm generate\n` +
			`  pnpm generated:check`;

		if (ctx.hasUI) {
			ctx.ui.notify(`Blocked edit to generated file: ${path}`, "warning");
		}

		return { block: true, reason };
	});
}
