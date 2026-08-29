import { StaticChip } from "@project-template/ui";
import styles from "./CardLabels.module.css";
import { isReservedLabel } from "./labels";
import type { Bootstrap, ProjectLabel } from "./model";
import { TagSwatch } from "./TagSwatch";

/**
 * The card's labels as a single non-wrapping row that scrolls on overflow.
 *
 * Team labels are hidden: the card title already carries the team's title
 * prefix, and design.md forbids repeating team and status controls on the card
 * surface. Reserved also covers deprecated and lifecycle names.
 *
 * The chips are presentational. Filtering by label already exists in the filter
 * row, and making each chip a button would add one tab stop per label per card
 * and put an activation target exactly where people scroll and drag. The row
 * itself is the single tab stop so keyboard users can scroll it — Chrome makes
 * scroll containers focusable automatically, Safari and Firefox do not.
 */
export function CardLabels({
	card,
	bootstrap,
	labelMetadata,
	title
}: {
	card: { labels: string[] };
	bootstrap: Bootstrap;
	labelMetadata: Map<string, ProjectLabel>;
	title: string;
}) {
	const visible = card.labels.filter((name) => !isReservedLabel(bootstrap, name));
	if (visible.length === 0) return null;
	return (
		<div className={styles.strip} role="group" aria-label={`${title} 的 Labels`} tabIndex={0}>
			{visible.map((name) => (
				<StaticChip
					key={name}
					className={styles.chip}
					variant="input"
					size="sm"
					label={name}
					title={labelMetadata.get(name)?.description ?? name}
					leading={<TagSwatch label={labelMetadata.get(name)} />}
				/>
			))}
		</div>
	);
}
