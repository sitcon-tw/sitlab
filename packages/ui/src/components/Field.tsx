import { useId, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { classNames } from "../lib/classNames";

interface FieldMetaProps {
	label: string;
	description?: string | undefined;
	error?: string | undefined;
	optional?: boolean | undefined;
}

function FieldFrame({ id, label, description, error, optional, children }: FieldMetaProps & { id: string; children: React.ReactNode }) {
	const descriptionId = description ? `${id}-description` : undefined;
	const errorId = error ? `${id}-error` : undefined;
	return (
		<div className="pt-field">
			<label className="pt-field__label" htmlFor={id}>
				{label} {optional ? <span className="pt-field__optional">(optional)</span> : null}
			</label>
			{children}
			{description ? (
				<p className="pt-field__description" id={descriptionId}>
					{description}
				</p>
			) : null}
			{error ? (
				<p className="pt-field__error" id={errorId}>
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

export function TextField({ label, description, error, optional, id: explicitId, className, "aria-describedby": ariaDescribedBy, ...props }: TextFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame id={id} label={label} description={description} error={error} optional={optional}>
			<input
				id={id}
				className={classNames("pt-input", className)}
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
	id: explicitId,
	className,
	"aria-describedby": ariaDescribedBy,
	...props
}: TextAreaFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame id={id} label={label} description={description} error={error} optional={optional}>
			<textarea
				id={id}
				className={classNames("pt-textarea", className)}
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

export function SelectField({
	label,
	description,
	error,
	optional,
	options,
	id: explicitId,
	className,
	"aria-describedby": ariaDescribedBy,
	...props
}: SelectFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	return (
		<FieldFrame id={id} label={label} description={description} error={error} optional={optional}>
			<select
				id={id}
				className={classNames("pt-select", className)}
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
