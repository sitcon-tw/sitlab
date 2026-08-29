import { isReservedLabel } from "./labels";
import { memberById, taipeiDateAfter, type BoardCard, type Bootstrap, type ProjectLabel } from "./model";

/**
 * Resolved, presentational content for the exported share image. Everything is
 * looked up eagerly — team prefix, list name, member names, label colors — so
 * the renderer never needs bootstrap and the builder stays unit-testable.
 */
export interface ShareCardData {
	iidText: string;
	title: string;
	/** Raw description text flattened to one whitespace-normalized run; the
	 * renderer clamps it. Mirrors the card tile, which shows the source text
	 * rather than rendered Markdown. */
	description: string;
	teamName: string | null;
	listName: string | null;
	assignees: ShareCardAssignee[];
	dueDate: string | null;
	overdue: boolean;
	labels: ShareCardLabel[];
}

export interface ShareCardAssignee {
	name: string;
	avatarUrl: string | null;
}

export interface ShareCardLabel {
	name: string;
	/** Validated `#rrggbb`, or null when GitLab supplied anything else. */
	color: string | null;
}

export function buildShareCardData(bootstrap: Bootstrap, card: BoardCard, labelMetadata: Map<string, ProjectLabel>): ShareCardData {
	const team = bootstrap.teams.find((item) => item.key === card.teamKey);
	const title = team && !card.title.startsWith(team.titlePrefix) ? `${team.titlePrefix} ${card.title}` : card.title;
	const list = bootstrap.board.lists.find((item) => item.key === card.listKey);
	const overdue = Boolean(card.dueDate && card.dueDate < taipeiDateAfter(0) && !list?.closed);
	const assignees = card.assigneeGitLabUserIds.flatMap((gitLabUserId) => {
		const member = memberById(bootstrap, gitLabUserId);
		return member ? [{ name: member.displayName, avatarUrl: member.avatarUrl }] : [];
	});
	const labels = card.labels
		.filter((name) => !isReservedLabel(bootstrap, name))
		.map((name) => {
			const color = labelMetadata.get(name)?.color;
			return { name, color: color && /^#[0-9a-f]{6}$/i.test(color) ? color : null };
		});
	return {
		iidText: `#${card.issueIid > 0 ? card.issueIid : "new"}`,
		title,
		description: card.description.replace(/\s+/g, " ").trim(),
		teamName: team?.name ?? null,
		listName: list?.name ?? null,
		assignees,
		dueDate: card.dueDate,
		overdue,
		labels
	};
}

export function shareCardFilename(issueIid: number) {
	return `sitcon-board-${issueIid > 0 ? issueIid : "new"}.png`;
}

/** Text width in the target font, injected so layout stays canvas-free. */
export type MeasureText = (text: string) => number;

const ellipsis = "…";

/**
 * Greedy per-character wrapping: titles are mostly CJK, which has no word
 * boundaries, so breaking anywhere is correct there and merely inelegant for
 * the occasional Latin run. Overflow past the last line ends in an ellipsis.
 */
export function wrapEllipsis(measure: MeasureText, text: string, maxWidth: number, maxLines: number): string[] {
	if (maxLines < 1) return [];
	const lines: string[] = [];
	let line = "";
	for (const character of Array.from(text.trim())) {
		if (!line || measure(line + character) <= maxWidth) {
			line += character;
			continue;
		}
		if (lines.length === maxLines - 1) return [...lines, ellipsize(measure, line, maxWidth)];
		lines.push(line);
		line = character;
	}
	if (line) lines.push(line);
	return lines;
}

function ellipsize(measure: MeasureText, line: string, maxWidth: number): string {
	let kept = Array.from(line);
	while (kept.length > 1 && measure(kept.join("") + ellipsis) > maxWidth) kept = kept.slice(0, -1);
	return kept.join("") + ellipsis;
}

export interface ChipRowsLayout {
	rows: ShareCardLabel[][];
	/** Labels that did not fit; when > 0 a `+N` chip closes the last row. */
	overflow: number;
}

/**
 * Packs label chips into at most maxRows rows. When labels remain after the
 * last row fills up, trailing chips are evicted until the `+N` overflow chip
 * fits alongside the survivors.
 */
export function layoutChipRows(
	chipWidth: (label: ShareCardLabel) => number,
	overflowChipWidth: (count: number) => number,
	labels: ShareCardLabel[],
	maxWidth: number,
	maxRows: number,
	gap: number
): ChipRowsLayout {
	if (maxRows < 1) return { rows: [], overflow: labels.length };
	const rowWidth = (row: ShareCardLabel[]) => row.reduce((total, label, index) => total + (index ? gap : 0) + chipWidth(label), 0);
	const rows: ShareCardLabel[][] = [];
	let row: ShareCardLabel[] = [];
	for (let index = 0; index < labels.length; index += 1) {
		const label = labels[index]!;
		if (!row.length || rowWidth([...row, label]) <= maxWidth) {
			row.push(label);
			continue;
		}
		if (rows.length === maxRows - 1) {
			let overflow = labels.length - index;
			while (row.length && rowWidth(row) + gap + overflowChipWidth(overflow) > maxWidth) {
				row.pop();
				overflow += 1;
			}
			rows.push(row);
			return { rows, overflow };
		}
		rows.push(row);
		row = [label];
	}
	if (row.length) rows.push(row);
	return { rows, overflow: 0 };
}
