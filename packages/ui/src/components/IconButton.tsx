import { Slot, Slottable } from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";

export type IconButtonVariant = "standard" | "filled" | "tonal" | "outlined";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> {
	label: string;
	icon: ReactNode;
	variant?: IconButtonVariant;
	size?: "sm" | "md";
	tone?: "neutral" | "error";
	/** Renders the toggle state and sets aria-pressed. */
	selected?: boolean;
	/** Render the caller's element instead of a <button>, e.g. an <a>. */
	asChild?: boolean;
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
	asChild = false,
	children,
	onPointerDown,
	...props
}: IconButtonProps) {
	const ripple = useRipple();
	const Root = asChild ? Slot : "button";
	return (
		<Root
			{...(asChild ? {} : { type })}
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
			{asChild ? <Slottable>{children}</Slottable> : null}
			{ripple.rippleNodes}
		</Root>
	);
}
