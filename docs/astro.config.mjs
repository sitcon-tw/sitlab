import mdx from "@astrojs/mdx";
import { defineConfig } from "astro/config";

const site = process.env.PUBLIC_SITE_URL || "http://localhost:4321";
const configuredBase = process.env.PUBLIC_BASE_PATH || "/";
const base = configuredBase === "/" ? "/" : `/${configuredBase.replace(/^\/+|\/+$/g, "")}`;

export default defineConfig({
	site,
	base,
	integrations: [mdx()],
	markdown: {
		shikiConfig: {
			theme: "github-dark-default",
			wrap: true
		}
	},
	output: "static",
	trailingSlash: "always"
});
