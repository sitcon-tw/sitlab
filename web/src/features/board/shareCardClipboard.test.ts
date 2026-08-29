import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { canCopyImageToClipboard, deliverSharePng } from "./shareCardClipboard";

const constructed: Array<Record<string, Blob | Promise<Blob>>> = [];

class FakeClipboardItem {
	static supports: ((type: string) => boolean) | undefined = undefined;

	constructor(readonly items: Record<string, Blob | Promise<Blob>>) {
		constructed.push(items);
	}
}

function stubClipboard(write: (items: unknown[]) => Promise<void>) {
	vi.stubGlobal("ClipboardItem", FakeClipboardItem);
	Object.defineProperty(navigator, "clipboard", { configurable: true, value: { write } });
}

const blob = new Blob(["png"], { type: "image/png" });
let clickedAnchors: HTMLAnchorElement[] = [];

beforeEach(() => {
	constructed.length = 0;
	clickedAnchors = [];
	FakeClipboardItem.supports = undefined;
	// jsdom has neither object URLs nor navigable downloads.
	URL.createObjectURL = vi.fn(() => "blob:fake");
	URL.revokeObjectURL = vi.fn();
	vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
		clickedAnchors.push(this);
	});
});

afterEach(() => {
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
	Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
});

describe("canCopyImageToClipboard", () => {
	it("requires ClipboardItem and clipboard.write", () => {
		expect(canCopyImageToClipboard()).toBe(false);
		stubClipboard(async () => {});
		expect(canCopyImageToClipboard()).toBe(true);
	});

	it("consults ClipboardItem.supports when present", () => {
		stubClipboard(async () => {});
		FakeClipboardItem.supports = () => false;
		expect(canCopyImageToClipboard()).toBe(false);
		FakeClipboardItem.supports = () => true;
		expect(canCopyImageToClipboard()).toBe(true);
	});
});

describe("deliverSharePng", () => {
	it("copies through a ClipboardItem constructed before the blob settles", async () => {
		const write = vi.fn(async () => {});
		stubClipboard(write);
		let resolveBlob!: (value: Blob) => void;
		const pending = deliverSharePng(() => new Promise<Blob>((resolve) => (resolveBlob = resolve)), "sitcon-board-127.png");
		// Safari requirement: the item wraps the still-pending promise.
		expect(constructed).toHaveLength(1);
		expect(constructed[0]!["image/png"]).toBeInstanceOf(Promise);
		resolveBlob(blob);
		await expect(pending).resolves.toBe("copied");
		expect(write).toHaveBeenCalledOnce();
		expect(clickedAnchors).toHaveLength(0);
	});

	it("falls back to a download when the clipboard write is refused", async () => {
		stubClipboard(async () => {
			throw new DOMException("denied", "NotAllowedError");
		});
		await expect(deliverSharePng(() => Promise.resolve(blob), "sitcon-board-127.png")).resolves.toBe("downloaded");
		expect(clickedAnchors).toHaveLength(1);
		expect(clickedAnchors[0]!.download).toBe("sitcon-board-127.png");
	});

	it("downloads directly when the browser has no image clipboard", async () => {
		await expect(deliverSharePng(() => Promise.resolve(blob), "sitcon-board-127.png")).resolves.toBe("downloaded");
		expect(clickedAnchors).toHaveLength(1);
	});

	it("rejects a failed render instead of downloading an empty file", async () => {
		stubClipboard(async (items) => {
			await (items[0] as FakeClipboardItem).items["image/png"];
		});
		await expect(deliverSharePng(() => Promise.reject(new Error("render failed")), "sitcon-board-127.png")).rejects.toThrow("render failed");
		expect(clickedAnchors).toHaveLength(0);
	});

	it("rejects a failed render on the download path too", async () => {
		await expect(deliverSharePng(() => Promise.reject(new Error("render failed")), "sitcon-board-127.png")).rejects.toThrow("render failed");
		expect(clickedAnchors).toHaveLength(0);
	});
});
