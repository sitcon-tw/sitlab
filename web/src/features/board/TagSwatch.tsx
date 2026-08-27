import type { CSSProperties } from "react";
import type { ProjectLabel } from "./model";
import styles from "./TagSwatch.module.css";

/**
 * The label's GitLab color, passed through a custom property.
 *
 * The value is runtime API data, so no color literal appears in source and the
 * frontend style check never sees one.
 */
export function TagSwatch({ label }: { label: ProjectLabel | undefined }) {
	const color = label && /^#[0-9a-f]{6}$/i.test(label.color) ? label.color : undefined;
	return <span className={styles.swatch} style={color ? ({ "--tag-color": color } as CSSProperties) : undefined} aria-hidden="true" />;
}
