import { describe, expect, it } from "vitest";
import { placeCaretPopover } from "./textareaCaret";

describe("placeCaretPopover", () => {
	it("anchors below the caret when space is available", () => {
		expect(placeCaretPopover({ left: 120, top: 100, bottom: 124 }, 500, { width: 1024, height: 768 })).toEqual({
			left: 120,
			top: 128,
			width: 448,
			maxHeight: 320
		});
	});

	it("clamps the menu to the viewport edge", () => {
		expect(placeCaretPopover({ left: 390, top: 100, bottom: 124 }, 300, { width: 400, height: 600 })).toMatchObject({
			left: 92,
			width: 300
		});
	});

	it("flips above a caret near the bottom of the viewport", () => {
		expect(placeCaretPopover({ left: 120, top: 560, bottom: 584 }, 400, { width: 800, height: 600 })).toEqual({
			left: 120,
			bottom: 44,
			width: 400,
			maxHeight: 320
		});
	});
});
