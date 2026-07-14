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
		<section className={classNames("pt-panel", padded && !hasHeader && "pt-panel--padded", className)} {...props}>
			{hasHeader ? (
				<header className="pt-panel__header">
					<div>
						{title ? <h2 className="pt-panel__title">{title}</h2> : null}
						{description ? <p className="pt-panel__description">{description}</p> : null}
					</div>
					{actions}
				</header>
			) : null}
			{hasHeader ? <div className="pt-panel__body">{children}</div> : children}
		</section>
	);
}
