import type { ReactNode } from "react";

export interface EmptyStateProps {
	title: string;
	description: string;
	icon?: ReactNode;
	action?: ReactNode;
}

export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
	return (
		<div className="md-empty-state">
			{icon ? (
				<div className="md-empty-state__icon" aria-hidden="true">
					{icon}
				</div>
			) : null}
			<h3 className="md-empty-state__title">{title}</h3>
			<p className="md-empty-state__description">{description}</p>
			{action}
		</div>
	);
}
