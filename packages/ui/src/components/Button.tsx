import type { ButtonHTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";
import { Spinner } from "./Spinner";

export type ButtonVariant = "filled" | "tonal" | "outlined" | "text" | "elevated";
export type ButtonTone = "primary" | "error";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	tone?: ButtonTone;
	size?: ButtonSize;
	loading?: boolean;
	loadingLabel?: string;
	leadingIcon?: ReactNode;
}

export function Button({
	variant = "filled",
	tone = "primary",
	size = "md",
	loading = false,
	loadingLabel = "Working",
	leadingIcon,
	className,
	disabled,
	children,
	type = "button",
	onPointerDown,
	...props
}: ButtonProps) {
	const ripple = useRipple();
	const icon = loading ? <Spinner size="sm" label={loadingLabel} /> : leadingIcon;
	return (
		<button
			type={type}
			className={classNames(
				"md-button",
				"md-state-layer",
				`md-button--${variant}`,
				tone === "error" && "md-button--error",
				size !== "md" && `md-button--${size}`,
				className
			)}
			disabled={disabled || loading}
			aria-busy={loading || undefined}
			onPointerDown={(event) => {
				ripple.onPointerDown(event);
				onPointerDown?.(event);
			}}
			{...props}
		>
			{icon ? <span className="md-button__icon">{icon}</span> : null}
			{children}
			{ripple.rippleNodes}
		</button>
	);
}
