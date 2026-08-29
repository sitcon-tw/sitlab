import { layoutChipRows, wrapEllipsis, type ShareCardData, type ShareCardLabel } from "./shareCard";

/**
 * Draws the share PNG on a Canvas 2D surface.
 *
 * Hand-drawn rather than DOM-rasterized: the layout is a small fixed spec,
 * fillText uses the document's already-loaded Inter + CJK fallback faces, the
 * output is identical across browsers, and each cross-origin avatar can fall
 * back to an initial without ever tainting the canvas.
 *
 * The image follows the active theme: colors come from the `--sb-share-*`
 * roles (design.md, Product roles) read at render time, so no color literal
 * appears in source and the export matches what the sender sees.
 */

const WIDTH = 720;
/** Fixed 2x export, independent of devicePixelRatio, for consistent output. */
const SCALE = 2;
const FRAME_PAD = 24;
const CARD_PAD = 28;
const CARD_RADIUS = 16;
const CONTENT_WIDTH = WIDTH - 2 * FRAME_PAD - 2 * CARD_PAD;
const SECTION_GAP = 18;
const META_SIZE = 13;
const META_HEIGHT = 18;
const META_TITLE_GAP = 14;
const TITLE_SIZE = 24;
const TITLE_LINE_HEIGHT = 34;
const TITLE_MAX_LINES = 3;
const DESC_SIZE = 15;
const DESC_LINE_HEIGHT = 24;
const DESC_MAX_LINES = 3;
const TITLE_DESC_GAP = 10;
const AVATAR_SIZE = 28;
const AVATAR_GAP = 6;
const AVATAR_NAMES_GAP = 12;
const NAME_SIZE = 15;
const DUE_SIZE = 15;
const DUE_HEIGHT = 22;
const CHIP_HEIGHT = 24;
const CHIP_PAD_X = 10;
const CHIP_GAP = 8;
const CHIP_ROW_GAP = 8;
const CHIP_TEXT_SIZE = 12;
const DOT_SIZE = 8;
const DOT_GAP = 6;
const MAX_AVATARS = 5;
const MAX_CHIP_ROWS = 2;

interface ShareTokens {
	frame: string;
	surface: string;
	onSurface: string;
	onSurfaceVariant: string;
	chip: string;
	avatar: string;
	onAvatar: string;
	overdue: string;
	typeface: string;
}

function readShareTokens(): ShareTokens {
	const style = getComputedStyle(document.documentElement);
	const read = (name: string) => {
		const value = style.getPropertyValue(name).trim();
		// A missing token would leave fillStyle at its previous value (canvas
		// ignores invalid assignments) and export a silently black image.
		if (!value) throw new Error(`Share token ${name} is not defined`);
		return value;
	};
	return {
		frame: read("--sb-share-frame"),
		surface: read("--sb-share-surface"),
		onSurface: read("--sb-share-on-surface"),
		onSurfaceVariant: read("--sb-share-on-surface-variant"),
		chip: read("--sb-share-chip"),
		avatar: read("--sb-share-avatar"),
		onAvatar: read("--sb-share-on-avatar"),
		overdue: read("--sb-share-overdue"),
		typeface: read("--md-ref-typeface-plain")
	};
}

/**
 * CORS-fetches one avatar; any failure (no ACAO header — the norm for
 * gitlab.com uploads — timeout, HTTP error) yields null and the renderer draws
 * an initial instead. Only verified bitmaps reach drawImage, so the canvas is
 * never tainted and toBlob cannot throw a security error.
 */
async function loadAvatarBitmap(url: string): Promise<ImageBitmap | null> {
	try {
		const response = await fetch(url, { mode: "cors", signal: AbortSignal.timeout(1200) });
		if (!response.ok) return null;
		return await createImageBitmap(await response.blob());
	} catch {
		return null;
	}
}

export async function renderShareCardImage(data: ShareCardData): Promise<Blob> {
	await document.fonts?.ready;
	const tokens = readShareTokens();
	const shownAssignees = data.assignees.slice(0, MAX_AVATARS);
	const avatars = await Promise.all(shownAssignees.map((assignee) => (assignee.avatarUrl ? loadAvatarBitmap(assignee.avatarUrl) : Promise.resolve(null))));

	const canvas = document.createElement("canvas");
	const ctx = canvas.getContext("2d");
	if (!ctx) throw new Error("Canvas 2D context unavailable");
	const font = (weight: number, size: number) => `${weight} ${size}px ${tokens.typeface}`;
	const measureWith = (fontValue: string) => {
		ctx.font = fontValue;
		return (text: string) => ctx.measureText(text).width;
	};

	// Measure pass: the canvas height depends on how the content wraps.
	const metaText = [data.iidText, data.teamName, data.listName].filter(Boolean).join(" · ");
	const titleLines = wrapEllipsis(measureWith(font(700, TITLE_SIZE)), data.title, CONTENT_WIDTH, TITLE_MAX_LINES);
	const descriptionLines = data.description ? wrapEllipsis(measureWith(font(400, DESC_SIZE)), data.description, CONTENT_WIDTH, DESC_MAX_LINES) : [];
	const measureChip = measureWith(font(500, CHIP_TEXT_SIZE));
	const chipWidth = (label: ShareCardLabel) => 2 * CHIP_PAD_X + (label.color ? DOT_SIZE + DOT_GAP : 0) + measureChip(label.name);
	const overflowChipWidth = (count: number) => 2 * CHIP_PAD_X + measureChip(`+${count}`);
	const chips = layoutChipRows(chipWidth, overflowChipWidth, data.labels, CONTENT_WIDTH, MAX_CHIP_ROWS, CHIP_GAP);
	const chipRowCount = chips.rows.length;

	let cardHeight = CARD_PAD + META_HEIGHT + META_TITLE_GAP + titleLines.length * TITLE_LINE_HEIGHT;
	if (descriptionLines.length) cardHeight += TITLE_DESC_GAP + descriptionLines.length * DESC_LINE_HEIGHT;
	if (data.assignees.length) cardHeight += SECTION_GAP + AVATAR_SIZE;
	if (data.dueDate) cardHeight += SECTION_GAP + DUE_HEIGHT;
	if (chipRowCount) cardHeight += SECTION_GAP + chipRowCount * CHIP_HEIGHT + (chipRowCount - 1) * CHIP_ROW_GAP;
	cardHeight += CARD_PAD;
	const height = cardHeight + 2 * FRAME_PAD;

	// Resizing resets all context state, so every draw below re-sets its own.
	canvas.width = WIDTH * SCALE;
	canvas.height = height * SCALE;
	ctx.scale(SCALE, SCALE);

	ctx.fillStyle = tokens.frame;
	ctx.fillRect(0, 0, WIDTH, height);
	ctx.fillStyle = tokens.surface;
	ctx.beginPath();
	ctx.roundRect(FRAME_PAD, FRAME_PAD, WIDTH - 2 * FRAME_PAD, cardHeight, CARD_RADIUS);
	ctx.fill();

	const left = FRAME_PAD + CARD_PAD;
	let y = FRAME_PAD + CARD_PAD;

	ctx.font = font(600, META_SIZE);
	ctx.fillStyle = tokens.onSurfaceVariant;
	ctx.textAlign = "left";
	ctx.textBaseline = "top";
	ctx.fillText(metaText, left, y + (META_HEIGHT - META_SIZE) / 2);
	y += META_HEIGHT + META_TITLE_GAP;

	ctx.font = font(700, TITLE_SIZE);
	ctx.fillStyle = tokens.onSurface;
	for (const line of titleLines) {
		ctx.fillText(line, left, y + (TITLE_LINE_HEIGHT - TITLE_SIZE) / 2);
		y += TITLE_LINE_HEIGHT;
	}

	if (descriptionLines.length) {
		y += TITLE_DESC_GAP;
		ctx.font = font(400, DESC_SIZE);
		ctx.fillStyle = tokens.onSurfaceVariant;
		for (const line of descriptionLines) {
			ctx.fillText(line, left, y + (DESC_LINE_HEIGHT - DESC_SIZE) / 2);
			y += DESC_LINE_HEIGHT;
		}
	}

	if (data.assignees.length) {
		y += SECTION_GAP;
		let x = left;
		shownAssignees.forEach((assignee, index) => {
			drawAvatar(ctx, tokens, font, x, y, avatars[index] ?? null, assignee.name);
			x += AVATAR_SIZE + AVATAR_GAP;
		});
		if (data.assignees.length > MAX_AVATARS) {
			drawBubble(ctx, tokens, font, x, y, `+${data.assignees.length - MAX_AVATARS}`);
			x += AVATAR_SIZE + AVATAR_GAP;
		}
		x += AVATAR_NAMES_GAP - AVATAR_GAP;
		const names = data.assignees.map((assignee) => assignee.name).join("、");
		ctx.font = font(400, NAME_SIZE);
		ctx.fillStyle = tokens.onSurface;
		ctx.textAlign = "left";
		ctx.textBaseline = "middle";
		const nameLine = wrapEllipsis(measureWith(font(400, NAME_SIZE)), names, left + CONTENT_WIDTH - x, 1)[0];
		if (nameLine) ctx.fillText(nameLine, x, y + AVATAR_SIZE / 2);
		y += AVATAR_SIZE;
	}

	if (data.dueDate) {
		y += SECTION_GAP;
		ctx.font = font(600, DUE_SIZE);
		ctx.fillStyle = data.overdue ? tokens.overdue : tokens.onSurfaceVariant;
		ctx.textAlign = "left";
		ctx.textBaseline = "middle";
		ctx.fillText(`期限 ${data.dueDate}${data.overdue ? "（已逾期）" : ""}`, left, y + DUE_HEIGHT / 2);
		y += DUE_HEIGHT;
	}

	if (chipRowCount) {
		y += SECTION_GAP;
		chips.rows.forEach((row, rowIndex) => {
			let x = left;
			for (const label of row) {
				x += drawChip(ctx, tokens, font, x, y, label) + CHIP_GAP;
			}
			if (chips.overflow > 0 && rowIndex === chipRowCount - 1) {
				drawChip(ctx, tokens, font, x, y, { name: `+${chips.overflow}`, color: null });
			}
			y += CHIP_HEIGHT + (rowIndex < chipRowCount - 1 ? CHIP_ROW_GAP : 0);
		});
	}

	return await new Promise<Blob>((resolve, reject) => {
		canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error("Canvas PNG export failed"))), "image/png");
	});
}

type FontBuilder = (weight: number, size: number) => string;

function drawAvatar(ctx: CanvasRenderingContext2D, tokens: ShareTokens, font: FontBuilder, x: number, y: number, bitmap: ImageBitmap | null, name: string) {
	if (bitmap) {
		ctx.save();
		ctx.beginPath();
		ctx.arc(x + AVATAR_SIZE / 2, y + AVATAR_SIZE / 2, AVATAR_SIZE / 2, 0, Math.PI * 2);
		ctx.clip();
		ctx.drawImage(bitmap, x, y, AVATAR_SIZE, AVATAR_SIZE);
		ctx.restore();
		return;
	}
	// Same fallback glyph rule as shared/Avatar.tsx.
	drawBubble(ctx, tokens, font, x, y, Array.from(name.trim())[0]?.toUpperCase() ?? "?");
}

function drawBubble(ctx: CanvasRenderingContext2D, tokens: ShareTokens, font: FontBuilder, x: number, y: number, text: string) {
	ctx.fillStyle = tokens.avatar;
	ctx.beginPath();
	ctx.arc(x + AVATAR_SIZE / 2, y + AVATAR_SIZE / 2, AVATAR_SIZE / 2, 0, Math.PI * 2);
	ctx.fill();
	ctx.fillStyle = tokens.onAvatar;
	ctx.font = font(600, 12);
	ctx.textAlign = "center";
	ctx.textBaseline = "middle";
	ctx.fillText(text, x + AVATAR_SIZE / 2, y + AVATAR_SIZE / 2 + 1);
	ctx.textAlign = "left";
}

/** Draws one chip at (x, y) and returns its width. */
function drawChip(ctx: CanvasRenderingContext2D, tokens: ShareTokens, font: FontBuilder, x: number, y: number, label: ShareCardLabel): number {
	ctx.font = font(500, CHIP_TEXT_SIZE);
	const textWidth = ctx.measureText(label.name).width;
	const width = 2 * CHIP_PAD_X + (label.color ? DOT_SIZE + DOT_GAP : 0) + textWidth;
	ctx.fillStyle = tokens.chip;
	ctx.beginPath();
	ctx.roundRect(x, y, width, CHIP_HEIGHT, CHIP_HEIGHT / 2);
	ctx.fill();
	let textX = x + CHIP_PAD_X;
	if (label.color) {
		// Label colors are GitLab API data, validated upstream in
		// buildShareCardData — no color literal originates in source.
		ctx.fillStyle = label.color;
		ctx.beginPath();
		ctx.arc(textX + DOT_SIZE / 2, y + CHIP_HEIGHT / 2, DOT_SIZE / 2, 0, Math.PI * 2);
		ctx.fill();
		textX += DOT_SIZE + DOT_GAP;
	}
	ctx.fillStyle = tokens.onSurface;
	ctx.textAlign = "left";
	ctx.textBaseline = "middle";
	ctx.fillText(label.name, textX, y + CHIP_HEIGHT / 2 + 0.5);
	return width;
}
