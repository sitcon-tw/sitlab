import type { CSSProperties } from "react";
import { classNames } from "../lib/classNames";

export interface LinearProgressProps {
	label: string;
	className?: string;
	/** 0-1. Omit for the indeterminate sweep. */
	value?: number;
}

/** MD3 linear progress indicator. */
export function LinearProgress({ label, className, value }: LinearProgressProps) {
	const fraction = typeof value === "number" ? Math.max(0, Math.min(1, value)) : null;
	return (
		<span
			className={classNames("md-linear-progress", className)}
			role="progressbar"
			aria-label={label}
			aria-valuemin={fraction === null ? undefined : 0}
			aria-valuemax={fraction === null ? undefined : 100}
			aria-valuenow={fraction === null ? undefined : Math.round(fraction * 100)}
			data-determinate={fraction === null ? undefined : true}
		>
			<span className="md-linear-progress__indicator" style={fraction === null ? undefined : ({ "--md-progress-value": fraction } as CSSProperties)} />
		</span>
	);
}
