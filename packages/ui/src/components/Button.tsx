import type { ButtonHTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { Spinner } from "./Spinner";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	size?: ButtonSize;
	loading?: boolean;
	loadingLabel?: string;
	leadingIcon?: ReactNode;
}

export function Button({
	variant = "primary",
	size = "md",
	loading = false,
	loadingLabel = "Working",
	leadingIcon,
	className,
	disabled,
	children,
	type = "button",
	...props
}: ButtonProps) {
	return (
		<button
			type={type}
			className={classNames("pt-button", `pt-button--${variant}`, `pt-button--${size}`, className)}
			disabled={disabled || loading}
			aria-busy={loading || undefined}
			{...props}
		>
			{loading ? <Spinner size="sm" label={loadingLabel} /> : leadingIcon}
			{children}
		</button>
	);
}
