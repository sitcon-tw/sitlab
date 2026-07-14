import { api, expectData, setCsrfToken } from "@/shared/api/client";
import type { Bootstrap } from "./model";

function readInjectedBootstrap() {
	const element = document.getElementById("__SITCON_BOOTSTRAP__");
	if (!element?.textContent) return null;
	try {
		return JSON.parse(element.textContent) as Bootstrap;
	} catch {
		return null;
	}
}

export async function loadInitialBootstrap() {
	let result = readInjectedBootstrap();
	if (!result && import.meta.env.VITE_SITCON_DEMO === "true") {
		result = (await import("@/test/demoBootstrap")).demoBootstrap;
	}
	if (!result) {
		const response = await api.GET("/bootstrap");
		if (response.response.status === 401) return null;
		result = expectData(response);
	}
	setCsrfToken(result.csrfToken);
	return result;
}
