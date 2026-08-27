import type { ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";

export interface SegmentedOption<Value extends string> {
	value: Value;
	label: string;
	icon?: ReactNode;
	disabled?: boolean;
}

export interface SegmentedButtonProps<Value extends string> {
	label: string;
	options: Array<SegmentedOption<Value>>;
	value: Value;
	onChange: (value: Value) => void;
	className?: string;
}

/** MD3 single-select segmented button. */
export function SegmentedButton<Value extends string>({ label, options, value, onChange, className }: SegmentedButtonProps<Value>) {
	return (
		<div className={classNames("md-segmented-button", className)} role="group" aria-label={label}>
			{options.map((option) => (
				<Segment key={option.value} option={option} selected={option.value === value} onSelect={() => onChange(option.value)} />
			))}
		</div>
	);
}

function Segment<Value extends string>({ option, selected, onSelect }: { option: SegmentedOption<Value>; selected: boolean; onSelect: () => void }) {
	const ripple = useRipple();
	return (
		<button
			type="button"
			className="md-segmented-button__segment md-state-layer"
			aria-pressed={selected}
			disabled={option.disabled}
			onPointerDown={ripple.onPointerDown}
			onClick={onSelect}
		>
			{option.icon}
			{option.label}
			{ripple.rippleNodes}
		</button>
	);
}
