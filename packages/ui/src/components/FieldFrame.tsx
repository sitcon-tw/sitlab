import type { ReactNode } from "react";
import { classNames } from "../lib/classNames";

export interface FieldMetaProps {
	label: string;
	description?: string | undefined;
	error?: string | undefined;
	optional?: boolean | undefined;
	/** Compact 44px product control for chrome above dense data. Dialogs stay 56px. */
	dense?: boolean | undefined;
	/** Keeps the label in the upper slot even while an empty field is unfocused. */
	alwaysFloatLabel?: boolean | undefined;
}

/** Shared frame for SITCON product fields. */
export function FieldFrame({
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

export function describedBy(id: string, description?: string, error?: string, explicit?: string) {
	return [explicit, description ? `${id}-description` : null, error ? `${id}-error` : null].filter(Boolean).join(" ") || undefined;
}
