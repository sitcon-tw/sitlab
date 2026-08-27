import type { ReactNode } from "react";
import { classNames } from "../lib/classNames";

export type ToastTone = "info" | "success" | "danger";

export interface ToastMessage {
	id: string;
	title: string;
	description?: string;
	tone?: ToastTone;
	action?: ReactNode;
}

export interface ToastRegionProps {
	messages: ToastMessage[];
}

/**
 * Material Design 3 snackbar region.
 *
 * Documented deviation: MD3 shows a single snackbar at a time. The board needs
 * to surface several failures at once, so this stacks. See design.md.
 *
 * `info` and `success` share the inverse-surface container; MD3 has no tonal
 * snackbar variants, and only failure earns a distinct one.
 */
export function ToastRegion({ messages }: ToastRegionProps) {
	return (
		<div className="md-toast-region" aria-label="Notifications">
			{messages.map((message) => (
				<div
					className={classNames("md-toast", message.tone === "danger" && "md-toast--danger")}
					key={message.id}
					role={message.tone === "danger" ? "alert" : "status"}
				>
					<div className="md-toast__text">
						<h2 className="md-toast__title">{message.title}</h2>
						{message.description ? <p className="md-toast__description">{message.description}</p> : null}
					</div>
					{message.action ? <div className="md-toast__action">{message.action}</div> : null}
				</div>
			))}
		</div>
	);
}
