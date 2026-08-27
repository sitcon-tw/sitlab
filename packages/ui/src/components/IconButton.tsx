import type { ButtonHTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";

export type IconButtonVariant = "standard" | "filled" | "tonal" | "outlined";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "children"> {
	label: string;
	icon: ReactNode;
	variant?: IconButtonVariant;
	size?: "sm" | "md";
	tone?: "neutral" | "error";
	/** Renders the toggle state and sets aria-pressed. */
	selected?: boolean;
}

export function IconButton({
	label,
	icon,
	variant = "standard",
	size = "md",
	tone = "neutral",
	selected,
	className,
	type = "button",
	onPointerDown,
	...props
}: IconButtonProps) {
	const ripple = useRipple();
	return (
		<button
			type={type}
			className={classNames(
				"md-icon-button",
				"md-state-layer",
				`md-icon-button--${variant}`,
				size !== "md" && `md-icon-button--${size}`,
				tone === "error" && "md-icon-button--error",
				className
			)}
			aria-label={label}
			aria-pressed={selected}
			title={label}
			onPointerDown={(event) => {
				ripple.onPointerDown(event);
				onPointerDown?.(event);
			}}
			{...props}
		>
			{icon}
			{ripple.rippleNodes}
		</button>
	);
}
