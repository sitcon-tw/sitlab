import { classNames } from "../lib/classNames";

export interface SpinnerProps {
	label?: string;
	size?: "sm" | "md" | "lg";
	className?: string | undefined;
	/** 0-1. Omit for the indeterminate arc. */
	value?: number;
}

const CIRCUMFERENCE = 2 * Math.PI * 20;

/**
 * MD3 circular progress indicator.
 *
 * Keeps role="status" and the visually hidden label: Button.test.tsx asserts
 * getByRole("status") carries the loading text.
 */
export function Spinner({ label = "Loading", size = "md", className, value }: SpinnerProps) {
	const determinate = typeof value === "number";
	return (
		<span className={classNames("md-spinner", size !== "md" && `md-spinner--${size}`, className)} role="status" data-determinate={determinate || undefined}>
			<svg viewBox="0 0 48 48" width="100%" height="100%" aria-hidden="true">
				<circle
					className="md-spinner__track"
					cx="24"
					cy="24"
					r="20"
					style={determinate ? { strokeDasharray: `${Math.max(0, Math.min(1, value)) * CIRCUMFERENCE} ${CIRCUMFERENCE}` } : undefined}
				/>
			</svg>
			<span className="md-sr-only">{label}</span>
		</span>
	);
}

export function SpinnerLayout({ label = "Loading" }: Pick<SpinnerProps, "label">) {
	return (
		<div className="md-spinner-layout">
			<Spinner size="lg" label={label} />
		</div>
	);
}
