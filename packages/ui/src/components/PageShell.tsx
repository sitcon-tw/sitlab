import type { HTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";

export interface PageShellProps extends HTMLAttributes<HTMLDivElement> {
	title: string;
	description?: string;
	actions?: ReactNode;
}

export function PageShell({ title, description, actions, className, children, ...props }: PageShellProps) {
	return (
		<div className={classNames("md-page-shell", className)} {...props}>
			<header className="md-page-shell__header">
				<div>
					<h1 className="md-page-shell__title">{title}</h1>
					{description ? <p className="md-page-shell__description">{description}</p> : null}
				</div>
				{actions ? <div className="md-page-shell__actions">{actions}</div> : null}
			</header>
			{children}
		</div>
	);
}
