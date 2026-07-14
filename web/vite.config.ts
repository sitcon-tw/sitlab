import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { configDefaults, defineConfig } from "vitest/config";

export default defineConfig({
	plugins: [react()],
	resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
	server: {
		port: 5173,
		proxy: { "/api": { target: process.env.API_PROXY_TARGET ?? "http://localhost:8080", changeOrigin: true } }
	},
	test: {
		environment: "jsdom",
		setupFiles: ["./src/test/setup.ts"],
		exclude: [...configDefaults.exclude, "e2e/**"],
		css: true
	}
});
