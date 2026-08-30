export interface CaretViewportRect {
	left: number;
	top: number;
	bottom: number;
}

export interface CaretPopoverPosition {
	left: number;
	top?: number;
	bottom?: number;
	width: number;
	maxHeight: number;
}

interface ViewportSize {
	width: number;
	height: number;
}

const viewportPadding = 8;
const caretGap = 4;
const preferredWidth = 448;
const preferredHeight = 320;
const minimumUsefulHeight = 96;

const mirroredProperties = [
	"borderBottomWidth",
	"borderLeftWidth",
	"borderRightWidth",
	"borderTopWidth",
	"boxSizing",
	"direction",
	"fontFamily",
	"fontSize",
	"fontStyle",
	"fontVariant",
	"fontWeight",
	"letterSpacing",
	"lineHeight",
	"paddingBottom",
	"paddingLeft",
	"paddingRight",
	"paddingTop",
	"tabSize",
	"textAlign",
	"textIndent",
	"textTransform",
	"wordBreak",
	"wordSpacing"
] as const;

/** Measures the textarea caret in viewport coordinates without changing selection or scroll. */
export function measureTextareaCaret(textarea: HTMLTextAreaElement, position = textarea.selectionStart): CaretViewportRect {
	const textareaRect = textarea.getBoundingClientRect();
	const styles = window.getComputedStyle(textarea);
	const mirror = document.createElement("div");
	const marker = document.createElement("span");

	Object.assign(mirror.style, {
		position: "absolute",
		top: "0",
		left: "-9999px",
		visibility: "hidden",
		overflow: "hidden",
		whiteSpace: "pre-wrap",
		overflowWrap: "break-word",
		width: `${textareaRect.width}px`
	});
	for (const property of mirroredProperties) mirror.style[property] = styles[property];

	mirror.textContent = textarea.value.slice(0, Math.max(0, Math.min(position, textarea.value.length)));
	marker.textContent = "\u200b";
	mirror.append(marker);
	document.body.append(mirror);

	try {
		const mirrorRect = mirror.getBoundingClientRect();
		const markerRect = marker.getBoundingClientRect();
		const lineHeight = Number.parseFloat(styles.lineHeight) || markerRect.height || Number.parseFloat(styles.fontSize) * 1.2;
		const left = markerRect.left - mirrorRect.left - textarea.scrollLeft;
		const top = markerRect.top - mirrorRect.top - textarea.scrollTop;
		return {
			left: textareaRect.left + left,
			top: textareaRect.top + top,
			bottom: textareaRect.top + top + lineHeight
		};
	} finally {
		mirror.remove();
	}
}

/** Places a caret popover within the viewport and flips it above the caret when needed. */
export function placeCaretPopover(caret: CaretViewportRect, fieldWidth: number, viewport: ViewportSize): CaretPopoverPosition {
	const usableWidth = Math.max(0, viewport.width - viewportPadding * 2);
	const width = Math.min(preferredWidth, fieldWidth, usableWidth);
	const left = clamp(caret.left, viewportPadding, Math.max(viewportPadding, viewport.width - width - viewportPadding));
	const spaceBelow = viewport.height - caret.bottom - caretGap - viewportPadding;
	const spaceAbove = caret.top - caretGap - viewportPadding;
	const placeAbove = spaceBelow < minimumUsefulHeight && spaceAbove > spaceBelow;
	const availableHeight = Math.max(0, placeAbove ? spaceAbove : spaceBelow);
	const common = { left, width, maxHeight: Math.min(preferredHeight, availableHeight) };

	return placeAbove ? { ...common, bottom: viewport.height - caret.top + caretGap } : { ...common, top: caret.bottom + caretGap };
}

function clamp(value: number, minimum: number, maximum: number) {
	return Math.min(Math.max(value, minimum), maximum);
}
