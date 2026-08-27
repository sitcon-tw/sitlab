import type { HTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";

export interface PanelProps extends HTMLAttributes<HTMLElement> {
	title?: string;
	description?: string;
	actions?: ReactNode;
	padded?: boolean;
}

export function Panel({ title, description, actions, padded = false, className, children, ...props }: PanelProps) {
	const hasHeader = Boolean(title || description || actions);
	return (
		<section className={classNames("md-panel", padded && !hasHeader && "md-panel--padded", className)} {...props}>
			{hasHeader ? (
				<header className="md-panel__header">
					<div>
						{title ? <h2 className="md-panel__title">{title}</h2> : null}
						{description ? <p className="md-panel__description">{description}</p> : null}
					</div>
					{actions}
				</header>
			) : null}
			{hasHeader ? <div className="md-panel__body">{children}</div> : children}
		</section>
	);
}
