import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(cleanup);

class TestResizeObserver implements ResizeObserver {
	disconnect() {}

	observe() {}

	unobserve() {}
}

globalThis.ResizeObserver = TestResizeObserver;

Object.defineProperty(HTMLCanvasElement.prototype, "getContext", {
	configurable: true,
	value: () => null
});

Object.defineProperty(window, "matchMedia", {
	configurable: true,
	value: (query: string) => ({
		matches: false,
		media: query,
		addEventListener: () => {},
		removeEventListener: () => {}
	})
});
