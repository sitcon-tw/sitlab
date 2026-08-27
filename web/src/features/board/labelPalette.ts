/**
 * GitLab's own suggested label colors.
 *
 * This is the only file in web/ that holds color literals, and it is
 * allowlisted in scripts/check-frontend-style.mjs for that reason: a color
 * picker needs concrete swatches, and these are GitLab's values rather than
 * product design tokens. Everything that renders them passes the value through
 * the --tag-color custom property, so no literal reaches TSX or CSS.
 */
export const labelPalette: readonly string[] = [
	"#009966",
	"#8FBC8F",
	"#3CB371",
	"#0E8A16",
	"#1D76DB",
	"#5843AD",
	"#6699CC",
	"#AD8D43",
	"#D73A4A",
	"#B60205",
	"#FF8C00",
	"#404040"
];

/** The swatch a new label starts on. */
export const defaultLabelColor = labelPalette[4]!;

/** Normalizes #abc and lowercase input to the #RRGGBB the contract requires. */
export function normalizeLabelColor(value: string): string | null {
	const trimmed = value.trim();
	const short = /^#([0-9a-f])([0-9a-f])([0-9a-f])$/i.exec(trimmed);
	if (short) return `#${short[1]}${short[1]}${short[2]}${short[2]}${short[3]}${short[3]}`.toUpperCase();
	return /^#[0-9a-f]{6}$/i.test(trimmed) ? trimmed.toUpperCase() : null;
}
