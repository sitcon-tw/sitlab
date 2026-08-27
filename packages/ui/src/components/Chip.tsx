import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";

export type ChipVariant = "assist" | "filter" | "input" | "suggestion";

interface ChipContent {
	variant?: ChipVariant | undefined;
	elevated?: boolean | undefined;
	selected?: boolean | undefined;
	leading?: ReactNode;
	trailing?: ReactNode;
}

export interface ChipProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children">, ChipContent {
	label: string;
	/** Renders a trailing remove button. Requires removeLabel. */
	onRemove?: () => void;
	removeLabel?: string;
	removeIcon?: ReactNode;
	removeDisabled?: boolean;
}

export interface StaticChipProps extends Omit<HTMLAttributes<HTMLSpanElement>, "children">, ChipContent {
	label: string;
}

function chipClass({ variant = "assist", elevated }: ChipContent, className?: string, interactive = true) {
	return classNames("md-chip", `md-chip--${variant}`, elevated && "md-chip--elevated", interactive && "md-state-layer", className);
}

/** Interactive MD3 chip. Use StaticChip for presentation-only chips. */
export function Chip({
	label,
	variant = "assist",
	elevated,
	selected,
	leading,
	trailing,
	onRemove,
	removeLabel,
	removeIcon,
	removeDisabled,
	className,
	type = "button",
	onPointerDown,
	...props
}: ChipProps) {
	const ripple = useRipple();
	const pressable = variant === "filter";
	return (
		<span className={chipClass({ variant, elevated }, className)} data-selected={selected || undefined}>
			<button
				type={type}
				className="md-chip__button"
				aria-pressed={pressable ? Boolean(selected) : undefined}
				onPointerDown={(event) => {
					ripple.onPointerDown(event);
					onPointerDown?.(event);
				}}
				{...props}
			>
				{leading ? <span className="md-chip__leading">{leading}</span> : null}
				<span className="md-chip__label">{label}</span>
			</button>
			{trailing ? <span className="md-chip__trailing">{trailing}</span> : null}
			{onRemove && removeLabel ? (
				<button type="button" className="md-chip__remove" aria-label={removeLabel} title={removeLabel} disabled={removeDisabled} onClick={onRemove}>
					{removeIcon}
				</button>
			) : null}
			{ripple.rippleNodes}
		</span>
	);
}

/** Presentational chip: no tab stop, no state layer. */
export function StaticChip({ label, variant = "assist", elevated, selected, leading, trailing, className, ...props }: StaticChipProps) {
	return (
		<span className={chipClass({ variant, elevated, selected }, className, false)} data-selected={selected || undefined} {...props}>
			{leading ? <span className="md-chip__leading">{leading}</span> : null}
			<span className="md-chip__label">{label}</span>
			{trailing ? <span className="md-chip__trailing">{trailing}</span> : null}
		</span>
	);
}
