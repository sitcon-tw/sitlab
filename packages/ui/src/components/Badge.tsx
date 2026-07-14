import type { HTMLAttributes } from "react";
import { classNames } from "../lib/classNames";

export type BadgeTone = "neutral" | "success" | "warning" | "danger" | "info";
export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
	tone?: BadgeTone;
}

export function Badge({ tone = "neutral", className, ...props }: BadgeProps) {
	return <span className={classNames("pt-badge", `pt-badge--${tone}`, className)} {...props} />;
}
