import type { HTMLAttributes, ReactNode } from "react";
import { classNames } from "../lib/classNames";

export interface TopAppBarProps extends HTMLAttributes<HTMLElement> {
	headline: ReactNode;
	leading?: ReactNode;
	trailing?: ReactNode;
	/** Raises the bar onto a container surface once content scrolls beneath it. */
	scrolled?: boolean;
}

/** Material Design 3 small top app bar. */
export function TopAppBar({ headline, leading, trailing, scrolled, className, ...props }: TopAppBarProps) {
	return (
		<header className={classNames("md-top-app-bar", className)} data-scrolled={scrolled || undefined} {...props}>
			{leading ? <div className="md-top-app-bar__leading">{leading}</div> : null}
			<h1 className="md-top-app-bar__headline">{headline}</h1>
			{trailing ? <div className="md-top-app-bar__trailing">{trailing}</div> : null}
		</header>
	);
}
