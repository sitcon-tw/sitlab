import { classNames } from "../lib/classNames";

export interface SpinnerProps {
	label?: string;
	size?: "sm" | "md" | "lg";
	className?: string;
}

export function Spinner({ label = "Loading", size = "md", className }: SpinnerProps) {
	return (
		<span className={classNames("pt-spinner", `pt-spinner--${size}`, className)} role="status">
			<span className="pt-sr-only">{label}</span>
		</span>
	);
}

export function SpinnerLayout({ label = "Loading" }: Pick<SpinnerProps, "label">) {
	return (
		<div className="pt-spinner-layout">
			<Spinner size="lg" label={label} />
		</div>
	);
}
