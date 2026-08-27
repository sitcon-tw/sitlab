import { Check, Loader2, TriangleAlert } from "lucide-react";
import styles from "./SaveIndicator.module.css";
import type { FieldSave } from "./useFieldSaveState";

const labels = { saving: "儲存中", saved: "已儲存", failed: "未同步" } as const;

/**
 * In-place save feedback for one field.
 *
 * Deliberately not a live region and fully aria-hidden: CardDetail owns one
 * visually hidden status region for the whole drawer, because seven concurrent
 * live regions cause announcement storms and several screen readers only track
 * one at a time.
 *
 * Also deliberately not focusable. `failed` is a marker only — the authoritative
 * failure surface stays the card-level alert with its retry button.
 */
export function SaveIndicator({ save, name }: { save: FieldSave | undefined; name: string }) {
	if (!save) return null;
	const text = labels[save.state];
	return (
		<span className={styles.indicator} data-state={save.state} title={`${name}${text}`} aria-hidden="true">
			{save.state === "saving" ? <Loader2 className={styles.spin} size="0.75rem" /> : null}
			{save.state === "saved" ? <Check size="0.75rem" /> : null}
			{save.state === "failed" ? <TriangleAlert size="0.75rem" /> : null}
			<span className={styles.text}>{text}</span>
		</span>
	);
}
