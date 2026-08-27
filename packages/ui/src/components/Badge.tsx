import type { HTMLAttributes } from "react";
import { classNames } from "../lib/classNames";

export type BadgeTone = "error" | "neutral" | "primary";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
	tone?: BadgeTone;
	/** Renders the 6dp dot form, which carries no label. */
	dot?: boolean;
}

/**
 * MD3 badge: a small dot or count marker. The tone-carrying status pill this
 * used to be is now a Chip.
 */
export function Badge({ tone = "error", dot = false, className, children, ...props }: BadgeProps) {
	return (
		<span className={classNames("md-badge", tone !== "error" && `md-badge--${tone}`, dot && "md-badge--dot", className)} {...props}>
			{dot ? null : children}
		</span>
	);
}
