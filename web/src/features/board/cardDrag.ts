/* Pointer-activation policy for whole-card dragging. The entire card surface
 * may begin a drag, with two carve-outs: interactive controls (the due date
 * field, assignee picker, links, retry) keep their own gestures, and the
 * title button opts back in via data-card-drag-surface because a press that
 * never travels the activation distance still lands as a click that opens the
 * card. The dedicated handle stays the keyboard activator. */
export function allowsCardDragActivation(target: EventTarget | null, source: { element?: Element | undefined; handle?: Element | undefined }): boolean {
	if (!(target instanceof Element)) return false;
	if (target === source.element || source.handle?.contains(target)) return true;
	const interactive = target.closest("input, select, textarea, a, button");
	if (!interactive || interactive === source.element) return true;
	return interactive.hasAttribute("data-card-drag-surface");
}
