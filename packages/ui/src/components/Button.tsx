import { Slot, Slottable } from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, ReactNode, Ref } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";
import { Spinner } from "./Spinner";

export type ButtonVariant = "filled" | "tonal" | "text" | "elevated";
export type ButtonTone = "primary" | "error";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	tone?: ButtonTone;
	size?: ButtonSize;
	loading?: boolean;
	loadingLabel?: string;
	leadingIcon?: ReactNode;
	/** Render the caller's element instead of a <button>, e.g. an <a>. */
	asChild?: boolean;
	ref?: Ref<HTMLButtonElement>;
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
	asChild = false,
	onPointerDown,
	...props
}: ButtonProps) {
	const ripple = useRipple();
	const icon = loading ? <Spinner size="sm" label={loadingLabel} /> : leadingIcon;
	const Root = asChild ? Slot : "button";
	return (
		<Root
			{...(asChild ? {} : { type })}
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
			{/* Slot needs exactly one element child; Slottable marks which one the
			    caller's element is, so the icon and ripple nest inside it. */}
			{asChild ? <Slottable>{children}</Slottable> : children}
			{ripple.rippleNodes}
		</Root>
	);
}
