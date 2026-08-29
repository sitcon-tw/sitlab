import { TriangleAlert } from "lucide-react";
import { classNames } from "../lib/classNames";
import { Button } from "./Button";

export interface UnsavedBarProps {
	/** Warning line, localized by the caller, e.g. 「小心，你還沒儲存」. */
	message: string;
	/** Localized by the caller, like ConfirmDialog's cancelLabel; the primitive ships no copy. */
	revertLabel: string;
	saveLabel: string;
	savingLabel?: string | undefined;
	onRevert: () => void;
	/** id of the form the save button submits when the bar sits outside that form. */
	form?: string | undefined;
	saving?: boolean;
	saveDisabled?: boolean;
	className?: string | undefined;
}

/**
 * Warning bar for forms that hold unsaved edits, shown only while the fields
 * differ from the saved values. Deviation 11 in design.md.
 *
 * Deliberately not a live region: the surface that mounts it already owns a
 * single status region for save announcements, and a region that mounts with
 * its content already present would not announce anyway.
 *
 * Positioning belongs to the consumer (sticky above a drawer's bottom edge,
 * static inside an inline row form) via `className` on the wrapper.
 */
export function UnsavedBar({ message, revertLabel, saveLabel, savingLabel, onRevert, form, saving = false, saveDisabled = false, className }: UnsavedBarProps) {
	return (
		<div className={classNames("md-unsaved-bar", className)}>
			<TriangleAlert size="1rem" aria-hidden="true" />
			<p className="md-unsaved-bar__message">{message}</p>
			<div className="md-unsaved-bar__actions">
				<Button type="button" variant="text" disabled={saving} onClick={onRevert}>
					{revertLabel}
				</Button>
				<Button type="submit" {...(form ? { form } : {})} variant="filled" loading={saving} loadingLabel={savingLabel ?? saveLabel} disabled={saveDisabled}>
					{saveLabel}
				</Button>
			</div>
		</div>
	);
}
