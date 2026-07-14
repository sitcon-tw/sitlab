import type { ButtonHTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "children"> {
	label: string;
	icon: ReactNode;
	size?: "sm" | "md";
	tone?: "neutral" | "danger";
}

export function IconButton({ label, icon, size = "md", tone = "neutral", className, type = "button", ...props }: IconButtonProps) {
	return (
		<button
			type={type}
			className={classNames("pt-icon-button", `pt-icon-button--${size}`, tone === "danger" && "pt-icon-button--danger", className)}
			aria-label={label}
			title={label}
			{...props}
		>
			{icon}
		</button>
	);
}
