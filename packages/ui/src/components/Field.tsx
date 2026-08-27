import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { classNames } from "../lib/classNames";

interface FieldMetaProps {
	label: string;
	description?: string | undefined;
	error?: string | undefined;
	optional?: boolean | undefined;
	/** Material density -2 (48dp) for chrome above dense data. Dialogs stay 56dp. */
	dense?: boolean | undefined;
}

/**
 * Material Design 3 outlined text field.
 *
 * The notch is a real <fieldset>/<legend>: the legend punches a hole in the
 * fieldset border natively, so the label can sit in the outline without
 * measuring text width in JavaScript. The fieldset is aria-hidden because it
 * would otherwise expose an empty group whose legend duplicates the label.
 *
 * Outlined rather than filled: board controls sit on tinted lane and card
 * surfaces where a filled field's container has almost no contrast, and the
 * filled variant's 56dp height would push the board past the viewport at 320px.
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
				<fieldset className="md-field__outline" aria-hidden="true">
					<legend className="md-field__notch">
						<span>{labelContent}</span>
					</legend>
				</fieldset>
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

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "size">, FieldMetaProps {}

export function TextField({
	label,
	description,
	error,
	optional,
	dense,
	id: explicitId,
	className,
	placeholder,
	"aria-describedby": ariaDescribedBy,
	...props
}: TextFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame id={id} label={label} description={description} error={error} optional={optional} dense={dense} className={className}>
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

export function TextAreaField({
	label,
	description,
	error,
	optional,
	dense,
	id: explicitId,
	className,
	placeholder,
	"aria-describedby": ariaDescribedBy,
	...props
}: TextAreaFieldProps) {
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
			className={classNames("md-field--textarea", className)}
		>
			<textarea
				id={id}
				className="md-field__input md-field__input--textarea"
				placeholder={placeholder ?? " "}
				aria-invalid={error ? true : undefined}
				aria-describedby={describedBy(id, description, error, ariaDescribedBy)}
				{...props}
			/>
		</FieldFrame>
	);
}

export interface SelectOption {
	value: string;
	label: string;
	disabled?: boolean;
}

export interface SelectFieldProps extends SelectHTMLAttributes<HTMLSelectElement>, FieldMetaProps {
	options: SelectOption[];
}

/**
 * MD3 outlined select on a native <select>.
 *
 * The element stays native deliberately: replacing it with a Menu breaks the
 * Playwright selectOption calls and the vitest toHaveValue assertions, and
 * loses the platform picker on mobile. MD3's visual spec is satisfiable here.
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
	"aria-describedby": ariaDescribedBy,
	...props
}: SelectFieldProps) {
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
			floating
			className={classNames("md-field--select", className)}
		>
			<select
				id={id}
				className="md-field__input md-field__input--select"
				aria-invalid={error ? true : undefined}
				aria-describedby={describedBy(id, description, error, ariaDescribedBy)}
				{...props}
			>
				{options.map((option) => (
					<option key={option.value} value={option.value} disabled={option.disabled}>
						{option.label}
					</option>
				))}
			</select>
		</FieldFrame>
	);
}
