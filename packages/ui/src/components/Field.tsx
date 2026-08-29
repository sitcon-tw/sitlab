import { Check } from "lucide-react";
import { forwardRef, useId, useState, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";
import { classNames } from "../lib/classNames";
import { Menu, MenuItem } from "./Menu";

interface FieldMetaProps {
	label: string;
	description?: string | undefined;
	error?: string | undefined;
	optional?: boolean | undefined;
	/** Compact 48px product control for chrome above dense data. Dialogs stay 56px. */
	dense?: boolean | undefined;
	/** Keeps the label in the upper slot even while an empty field is unfocused. */
	alwaysFloatLabel?: boolean | undefined;
}

/**
 * SITCON product field. Its label floats inside the control while a separate,
 * uninterrupted outline preserves the 12px shape on any parent surface.
 */
function FieldFrame({
	id,
	label,
	description,
	error,
	optional,
	dense,
	floating,
	className,
	children
}: FieldMetaProps & { id: string; floating?: boolean | undefined; className?: string | undefined; children: ReactNode }) {
	const labelContent = (
		<>
			{label} {optional ? <span className="md-field__optional">(optional)</span> : null}
		</>
	);
	return (
		<div
			className={classNames("md-field", dense && "md-field--dense", className)}
			data-invalid={error ? true : undefined}
			data-floating={floating || undefined}
		>
			<div className="md-field__box">
				{children}
				<label className="md-field__label" htmlFor={id}>
					{labelContent}
				</label>
				<span className="md-field__outline" aria-hidden="true" />
			</div>
			{description ? (
				<p className="md-field__supporting" id={`${id}-description`}>
					{description}
				</p>
			) : null}
			{error ? (
				<p className="md-field__supporting md-field__supporting--error" id={`${id}-error`} role="alert">
					{error}
				</p>
			) : null}
		</div>
	);
}

function describedBy(id: string, description?: string, error?: string, explicit?: string) {
	return [explicit, description ? `${id}-description` : null, error ? `${id}-error` : null].filter(Boolean).join(" ") || undefined;
}

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "size">, FieldMetaProps {
	/** Content rendered inside the field box before the input, e.g. token chips. */
	leading?: ReactNode | undefined;
}

export function TextField({
	label,
	description,
	error,
	optional,
	dense,
	alwaysFloatLabel,
	leading,
	id: explicitId,
	className,
	placeholder,
	"aria-describedby": ariaDescribedBy,
	...props
}: TextFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame
			id={id}
			label={label}
			description={description}
			error={error}
			optional={optional}
			dense={dense}
			floating={alwaysFloatLabel}
			className={className}
		>
			{leading ? <span className="md-field__leading">{leading}</span> : null}
			<input
				id={id}
				className="md-field__input"
				// A placeholder is always present so :placeholder-shown is a reliable
				// "is empty" signal for the float, with no JS and no controlled-value
				// dependency. CSS reveals a real placeholder only once the label floats.
				placeholder={placeholder ?? " "}
				aria-invalid={error ? true : undefined}
				aria-describedby={describedBy(id, description, error, ariaDescribedBy)}
				{...props}
			/>
		</FieldFrame>
	);
}

export interface TextAreaFieldProps extends TextareaHTMLAttributes<HTMLTextAreaElement>, FieldMetaProps {}

export const TextAreaField = forwardRef<HTMLTextAreaElement, TextAreaFieldProps>(function TextAreaField(
	{ label, description, error, optional, dense, alwaysFloatLabel, id: explicitId, className, placeholder, "aria-describedby": ariaDescribedBy, ...props },
	ref
) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame
			id={id}
			label={label}
			description={description}
			error={error}
			optional={optional}
			dense={dense}
			floating={alwaysFloatLabel}
			className={classNames("md-field--textarea", className)}
		>
			<textarea
				ref={ref}
				id={id}
				className="md-field__input md-field__input--textarea"
				placeholder={placeholder ?? " "}
				aria-invalid={error ? true : undefined}
				aria-describedby={describedBy(id, description, error, ariaDescribedBy)}
				{...props}
			/>
		</FieldFrame>
	);
});

export interface SelectOption {
	value: string;
	label: string;
	disabled?: boolean;
}

export interface SelectFieldProps extends FieldMetaProps {
	options: SelectOption[];
	id?: string | undefined;
	className?: string | undefined;
	value?: string | undefined;
	defaultValue?: string | undefined;
	onValueChange?: ((value: string) => void) | undefined;
	disabled?: boolean | undefined;
	required?: boolean | undefined;
	name?: string | undefined;
	autoFocus?: boolean | undefined;
	"aria-describedby"?: string | undefined;
}

/**
 * Product-styled single-choice field. It shares the same keyboard-accessible,
 * shadow-free menu surface as the board's sorting and filtering controls.
 * The label is always floated because a select always has a value.
 */
export function SelectField({
	label,
	description,
	error,
	optional,
	dense,
	options,
	id: explicitId,
	className,
	value,
	defaultValue,
	onValueChange,
	disabled,
	required,
	name,
	autoFocus,
	"aria-describedby": ariaDescribedBy
}: SelectFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	const firstEnabledValue = options.find((option) => !option.disabled)?.value ?? "";
	const [uncontrolledValue, setUncontrolledValue] = useState(() => defaultValue ?? firstEnabledValue);
	const selectedValue = value ?? uncontrolledValue;
	const selectedOption = options.find((option) => option.value === selectedValue) ?? options.find((option) => !option.disabled);
	const selectOption = (nextValue: string) => {
		if (value === undefined) setUncontrolledValue(nextValue);
		onValueChange?.(nextValue);
	};
	return (
		<FieldFrame
			id={id}
			label={label}
			description={description}
			error={error}
			optional={optional}
			dense={dense}
			floating
			className={classNames("md-field--select", className)}
		>
			<Menu
				label={`${label}選項`}
				className="md-select-menu"
				trigger={
					<button
						id={id}
						type="button"
						className="md-field__input md-field__input--select"
						disabled={disabled}
						autoFocus={autoFocus}
						aria-required={required || undefined}
						aria-invalid={error ? true : undefined}
						aria-describedby={describedBy(id, description, error, ariaDescribedBy)}
					>
						{selectedOption?.label ?? ""}
					</button>
				}
			>
				{options.map((option) => (
					<MenuItem
						key={option.value}
						disabled={option.disabled}
						selected={selectedValue === option.value}
						leading={selectedValue === option.value ? <Check size="1.125rem" aria-hidden="true" /> : <span aria-hidden="true" />}
						onSelect={() => selectOption(option.value)}
					>
						{option.label}
					</MenuItem>
				))}
			</Menu>
			{name ? <input type="hidden" name={name} value={selectedValue} disabled={disabled} /> : null}
		</FieldFrame>
	);
}
