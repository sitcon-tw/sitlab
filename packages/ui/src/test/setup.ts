import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// jsdom has no ResizeObserver; the MD3 tab indicator observes its list.
class TestResizeObserver implements ResizeObserver {
	disconnect() {}

	observe() {}

	unobserve() {}
}

globalThis.ResizeObserver = TestResizeObserver;

function defaultMatchMedia(query: string) {
	return {
		matches: false,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	};
}

const matchMedia = vi.fn(defaultMatchMedia);

Object.defineProperty(window, "matchMedia", {
	configurable: true,
	writable: true,
	value: matchMedia
});

afterEach(() => {
	cleanup();
	matchMedia.mockReset();
	matchMedia.mockImplementation(defaultMatchMedia);
});

Object.defineProperty(HTMLCanvasElement.prototype, "getContext", {
	configurable: true,
	value: () => null
});
